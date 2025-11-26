package engine

import (
	"errors"
	"sort"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

// PriceLevel holds FIFO queue of orders at one price.
type PriceLevel struct {
	Price  int64
	Orders []*model.Order
}

// OrderBook holds buy & sell levels for a single symbol.
type OrderBook struct {
	Symbol string

	Bids map[int64]*PriceLevel // price -> level (BUY side)
	Asks map[int64]*PriceLevel // price -> level (SELL side)
}

// NewOrderBook creates a fresh book for a symbol.
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		Bids:   make(map[int64]*PriceLevel),
		Asks:   make(map[int64]*PriceLevel),
	}
}

// addToBook inserts leftover limit order into the appropriate side.
func (ob *OrderBook) addToBook(o *model.Order) {
	sideMap := ob.Bids
	if o.Side == model.SELL {
		sideMap = ob.Asks
	}

	level, ok := sideMap[o.Price]
	if !ok {
		level = &PriceLevel{Price: o.Price}
		sideMap[o.Price] = level
	}

	level.Orders = append(level.Orders, o) // FIFO append
}

// matchLimit tries to match a limit order.
func (ob *OrderBook) matchLimit(o *model.Order) (trades []model.Order) {
	if o.Side == model.BUY {
		return ob.matchBuyLimit(o)
	}
	return ob.matchSellLimit(o)
}

// matchBuyLimit matches BUY with lowest ASK prices.
func (ob *OrderBook) matchBuyLimit(o *model.Order) (trades []model.Order) {
	askPrices := ob.sortedPrices(ob.Asks, true) // ascending
	for _, p := range askPrices {
		if o.Quantity == 0 {
			break
		}
		if p > o.Price {
			break // cannot cross further
		}
		level := ob.Asks[p]
		i := 0
		for i < len(level.Orders) && o.Quantity > 0 {
			sell := level.Orders[i]

			// trade at resting order's price
			tradeQty := min(o.Quantity, sell.Quantity-sell.Filled)
			if tradeQty <= 0 {
				i++
				continue
			}

			sell.Filled += tradeQty
			o.Filled += tradeQty
			o.Quantity -= tradeQty

			trades = append(trades, model.Order{
				ID:       "", // Trade object simplified
				Symbol:   ob.Symbol,
				Side:     model.BUY,
				Price:    sell.Price,
				Quantity: tradeQty,
			})

			if sell.Filled == sell.Quantity {
				// remove sell order from level
				level.Orders = append(level.Orders[:i], level.Orders[i+1:]...)
				continue
			}
			i++
		}

		if len(level.Orders) == 0 {
			delete(ob.Asks, p)
		}
	}
	return trades
}

// matchSellLimit matches SELL with highest BID prices.
func (ob *OrderBook) matchSellLimit(o *model.Order) (trades []model.Order) {
	bidPrices := ob.sortedPrices(ob.Bids, false) // descending
	for _, p := range bidPrices {
		if o.Quantity == 0 {
			break
		}
		if p < o.Price {
			break
		}
		level := ob.Bids[p]
		i := 0
		for i < len(level.Orders) && o.Quantity > 0 {
			buy := level.Orders[i]

			tradeQty := min(o.Quantity, buy.Quantity-buy.Filled)
			if tradeQty <= 0 {
				i++
				continue
			}

			buy.Filled += tradeQty
			o.Filled += tradeQty
			o.Quantity -= tradeQty

			trades = append(trades, model.Order{
				ID:       "",
				Symbol:   ob.Symbol,
				Side:     model.SELL,
				Price:    buy.Price,
				Quantity: tradeQty,
			})

			if buy.Filled == buy.Quantity {
				level.Orders = append(level.Orders[:i], level.Orders[i+1:]...)
				continue
			}
			i++
		}
		if len(level.Orders) == 0 {
			delete(ob.Bids, p)
		}
	}
	return trades
}

// sortedPrices returns sorted keys of side map.
// asc=true → ascending, asc=false → descending.
func (ob *OrderBook) sortedPrices(m map[int64]*PriceLevel, asc bool) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	if asc {
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		return keys
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] > keys[j] })
	return keys
}

// ProcessOrder handles LIMIT and MARKET orders for a single symbol.
// MARKET must fully execute or be rejected.
func (ob *OrderBook) ProcessOrder(o *model.Order) ([]model.Order, error) {
	if o.Type == model.MARKET {
		return ob.processMarket(o)
	}
	return ob.processLimit(o), nil
}

func (ob *OrderBook) processLimit(o *model.Order) []model.Order {
	trades := ob.matchLimit(o)
	if o.Quantity > 0 && o.Type == model.LIMIT {
		ob.addToBook(o)
	}
	return trades
}

func (ob *OrderBook) processMarket(o *model.Order) ([]model.Order, error) {
	// compute available liquidity
	available := int64(0)
	if o.Side == model.BUY {
		for _, lvl := range ob.Asks {
			for _, s := range lvl.Orders {
				available += s.Quantity - s.Filled
			}
		}
	} else {
		for _, lvl := range ob.Bids {
			for _, b := range lvl.Orders {
				available += b.Quantity - b.Filled
			}
		}
	}
	if available < o.Quantity {
		return nil, errors.New("insufficient liquidity for market order")
	}
	return ob.matchLimit(o), nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
