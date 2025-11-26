package engine

import (
	"fmt"
	"sort"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/metrics"
	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

// CmdType defines the kind of command sent to a shard.
type CmdType int

const (
	CmdSubmit CmdType = iota
	CmdCancel
	CmdGetOrder
	CmdGetBook
)

// Cmd is a command routed to a shard.
type Cmd struct {
	Typ     CmdType
	Order   *model.Order // for submit
	OrderID string       // for cancel/get
	Symbol  string       // routing key (for submit/getbook)
	Depth   int          // for orderbook snapshot
	Reply   chan interface{}
}

// SubmitResult is returned by a submit command.
type SubmitResult struct {
	Order      *model.Order  // order after processing (filled/remaining updated)
	Trades     []model.Order // trades executed (trade objects simplified)
	StatusCode int           // HTTP-like status (201/200/202 semantics)
	Err        string        // non-empty on error
}

// CancelResult for cancel command
type CancelResult struct {
	OK  bool
	Err string
}

// GetResult for GET order
type GetResult struct {
	Order *model.Order
	Err   string
}

// BookSnapshot is returned by GetOrderBook
type BookSnapshot struct {
	Symbol string
	Bids   []map[string]interface{}
	Asks   []map[string]interface{}
}

// shard is the actor owning a subset of symbols.
type shard struct {
	in      chan *Cmd
	books   map[string]*OrderBook   // symbol -> orderbook (owned)
	orders  map[string]*model.Order // orderID -> order (owned)
	bufSize int
	quit    chan struct{}
}

// newShard creates and starts a shard loop.
func newShard(bufSize int) *shard {
	s := &shard{
		in:      make(chan *Cmd, bufSize),
		books:   make(map[string]*OrderBook),
		orders:  make(map[string]*model.Order),
		bufSize: bufSize,
		quit:    make(chan struct{}),
	}
	go s.loop()
	return s
}

func (s *shard) loop() {
	for {
		select {
		case cmd := <-s.in:
			switch cmd.Typ {
			case CmdSubmit:
				s.handleSubmit(cmd)
			case CmdCancel:
				s.handleCancel(cmd)
			case CmdGetOrder:
				s.handleGet(cmd)
			case CmdGetBook:
				s.handleGetBook(cmd)
			}
		case <-s.quit:
			return
		}
	}
}

func (s *shard) stop() {
	close(s.quit)
}

func (s *shard) getOrCreateBook(symbol string) *OrderBook {
	ob, ok := s.books[symbol]
	if !ok {
		ob = NewOrderBook(symbol)
		s.books[symbol] = ob
	}
	return ob
}

func (s *shard) handleSubmit(cmd *Cmd) {
	o := cmd.Order
	ob := s.getOrCreateBook(o.Symbol)

	// Process order inside the shard (serial)
	trades, err := ob.ProcessOrder(o)
	if err != nil {
		// market rejection etc.
		res := SubmitResult{Err: err.Error()}
		cmd.Reply <- res
		return
	}

	// If limit order with remaining quantity, store it in shard state
	if o.Type == model.LIMIT && o.Quantity > 0 {
		s.orders[o.ID] = o
	}

	// Build status semantics used by API: 201, 200, 202
	status := 201
	if o.Filled > 0 && o.Quantity == 0 {
		status = 200
	} else if o.Filled > 0 && o.Quantity > 0 {
		status = 202
	}

	res := SubmitResult{
		Order:      o,
		Trades:     trades,
		StatusCode: status,
	}

	// instrumentation: count this submit (regardless of trade/remaining)
	metrics.AddOrdersProcessed(1)

	cmd.Reply <- res
}

func (s *shard) handleCancel(cmd *Cmd) {
	id := cmd.OrderID
	o, ok := s.orders[id]
	if !ok {
		// either not found or already filled/removed
		cmd.Reply <- CancelResult{OK: false, Err: "order not found"}
		return
	}

	// If fully filled
	if o.Filled >= o.Quantity {
		cmd.Reply <- CancelResult{OK: false, Err: "cannot cancel a fully filled order"}
		return
	}

	// Remove from shard orders map
	delete(s.orders, id)

	// Remove from orderbook price level
	ob := s.getOrCreateBook(o.Symbol)
	var sideMap map[int64]*PriceLevel
	if o.Side == model.BUY {
		sideMap = ob.Bids
	} else {
		sideMap = ob.Asks
	}

	if level, ok := sideMap[o.Price]; ok {
		// rebuild slice without the cancelled order
		newOrders := level.Orders[:0]
		for _, ord := range level.Orders {
			if ord.ID == o.ID {
				continue
			}
			newOrders = append(newOrders, ord)
		}
		level.Orders = newOrders
		if len(level.Orders) == 0 {
			if o.Side == model.BUY {
				delete(ob.Bids, o.Price)
			} else {
				delete(ob.Asks, o.Price)
			}
		}
	}

	cmd.Reply <- CancelResult{OK: true}
}

func (s *shard) handleGet(cmd *Cmd) {
	id := cmd.OrderID
	o, ok := s.orders[id]
	if !ok {
		cmd.Reply <- GetResult{Err: "order not found"}
		return
	}
	cmd.Reply <- GetResult{Order: o}
}

func (s *shard) handleGetBook(cmd *Cmd) {
	ob := s.getOrCreateBook(cmd.Symbol)
	depth := cmd.Depth
	if depth <= 0 {
		depth = 10
	}
	snap := BookSnapshot{
		Symbol: cmd.Symbol,
		Bids:   aggregate(ob.Bids, depth, false),
		Asks:   aggregate(ob.Asks, depth, true),
	}
	cmd.Reply <- snap
}

// small helper for debugging
func (s *shard) String() string {
	return fmt.Sprintf("shard{books=%d,orders=%d}", len(s.books), len(s.orders))
}

// aggregate builds a price-level summary:
// For bids -> highest first (asc=false)
// For asks -> lowest first (asc=true)
func aggregate(side map[int64]*PriceLevel, depth int, asc bool) []map[string]interface{} {
	if depth <= 0 {
		return []map[string]interface{}{}
	}

	// collect prices
	prices := make([]int64, 0, len(side))
	for p := range side {
		prices = append(prices, p)
	}

	// sort prices
	if asc {
		sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	} else {
		sort.Slice(prices, func(i, j int) bool { return prices[i] > prices[j] })
	}

	// aggregate quantities
	out := make([]map[string]interface{}, 0, depth)
	for _, p := range prices {
		if len(out) >= depth {
			break
		}
		level := side[p]
		total := int64(0)
		for _, o := range level.Orders {
			total += (o.Quantity - o.Filled)
		}
		out = append(out, map[string]interface{}{
			"price":    p,
			"quantity": total,
		})
	}
	return out
}
