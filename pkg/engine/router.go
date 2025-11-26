package engine

import (
	"hash/fnv"
	"runtime"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

// Router routes commands to N shards.
type Router struct {
	shards []*shard
	n      int
	buf    int
}

// NewRouter creates a router with numShards worker shards and channel buffer size buf.
func NewRouter(numShards int, buf int) *Router {
	if numShards <= 0 {
		numShards = runtime.NumCPU()
	}
	r := &Router{
		shards: make([]*shard, numShards),
		n:      numShards,
		buf:    buf,
	}
	for i := 0; i < numShards; i++ {
		r.shards[i] = newShard(buf)
	}
	return r
}

func (r *Router) Stop() {
	for _, s := range r.shards {
		s.stop()
	}
}

// routeIdx returns the shard index for a symbol.
func (r *Router) routeIdx(symbol string) int {
	h := fnv.New32a()
	h.Write([]byte(symbol))
	return int(h.Sum32()) % r.n
}

// SubmitOrder routes an order to the owning shard and waits for a SubmitResult.
func (r *Router) SubmitOrder(o *model.Order) SubmitResult {
	idx := r.routeIdx(o.Symbol)
	reply := make(chan interface{})
	cmd := &Cmd{
		Typ:    CmdSubmit,
		Order:  o,
		Symbol: o.Symbol,
		Reply:  reply,
	}
	r.shards[idx].in <- cmd
	ri := <-reply
	return ri.(SubmitResult)
}

// CancelOrder routes a cancel request to the shard that should own the order.
// Note: caller must know which symbol the order belonged to. For this router we
// accept cancel with orderID and symbol to ensure correct routing. If orderID->symbol mapping is external,
// caller must supply symbol; for now we'll assume caller provides symbol (the API will route by symbol).
func (r *Router) CancelOrder(symbol, orderID string) CancelResult {
	idx := r.routeIdx(symbol)
	reply := make(chan interface{})
	cmd := &Cmd{Typ: CmdCancel, OrderID: orderID, Symbol: symbol, Reply: reply}
	r.shards[idx].in <- cmd
	ri := <-reply
	return ri.(CancelResult)
}

// GetOrder retrieves an order by id from the shard owning the symbol.
// As above, caller must pass the symbol for routing.
func (r *Router) GetOrder(symbol, orderID string) GetResult {
	idx := r.routeIdx(symbol)
	reply := make(chan interface{})
	cmd := &Cmd{Typ: CmdGetOrder, OrderID: orderID, Symbol: symbol, Reply: reply}
	r.shards[idx].in <- cmd
	ri := <-reply
	return ri.(GetResult)
}

// GetOrderBook returns aggregated snapshot for a symbol.
func (r *Router) GetOrderBook(symbol string, depth int) BookSnapshot {
	idx := r.routeIdx(symbol)
	reply := make(chan interface{})
	cmd := &Cmd{Typ: CmdGetBook, Symbol: symbol, Depth: depth, Reply: reply}
	r.shards[idx].in <- cmd
	ri := <-reply
	return ri.(BookSnapshot)
}
