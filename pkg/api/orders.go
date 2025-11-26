package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/engine"
	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
	"github.com/2019UGEC100/order-matching-engine-go/pkg/store"
)

var (
	// Temporary globals for Step 6 (will be sharded in Step 7)
	orderStore = store.NewStore()
	orderBooks = make(map[string]*engine.OrderBook)
)

// getOrCreateBook returns a single-symbol orderbook (not concurrency-safe here — shard in Step 7)
func getOrCreateBook(symbol string) *engine.OrderBook {
	ob, ok := orderBooks[symbol]
	if !ok {
		ob = engine.NewOrderBook(symbol)
		orderBooks[symbol] = ob
	}
	return ob
}

// CreateOrderHandler handles POST /api/v1/orders
// Expects JSON body matching model.Order (symbol, side, type, price (cents) for LIMIT, quantity)
func CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	var req model.Order
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Assign ID and timestamp
	req.ID = uuid.NewString()
	req.Timestamp = time.Now().UnixMilli()

	// Process against the single-symbol orderbook
	ob := getOrCreateBook(req.Symbol)

	trades, err := ob.ProcessOrder(&req)
	if err != nil {
		// e.g., market rejection
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// If it is a resting limit order (has remaining qty), add to store for lookup/cancel
	if req.Type == model.LIMIT && req.Quantity > 0 {
		orderStore.Add(&req)
	}

	// Build response
	// ensure trades_executed is [] instead of null
	var tradesResp []model.Order
	if trades == nil {
		tradesResp = []model.Order{}
	} else {
		tradesResp = trades
	}

	resp := map[string]interface{}{
		"order_id":        req.ID,
		"symbol":          req.Symbol,
		"side":            req.Side,
		"type":            req.Type,
		"price":           req.Price,
		"quantity":        req.Quantity + req.Filled, // original quantity
		"filled_quantity": req.Filled,
		"remaining":       req.Quantity,
		"trades_executed": tradesResp,
	}

	// Determine status code:
	// - 201 CREATED if resting limit order (no fills and order is resting)
	// - 200 OK if fully filled immediately
	// - 202 ACCEPTED if partially filled and remaining rests
	status := http.StatusCreated
	if req.Filled > 0 && req.Quantity == 0 {
		status = http.StatusOK
	} else if req.Filled > 0 && req.Quantity > 0 {
		status = http.StatusAccepted
	}

	writeJSON(w, status, resp)
}

// OrderByIDHandler dispatches GET/DELETE for /api/v1/orders/{id}
func OrderByIDHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		GetOrderHandler(w, r)
	case http.MethodDelete:
		CancelOrderHandler(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// GetOrderHandler handles GET /api/v1/orders/{id}
func GetOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path)
	o, err := orderStore.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	resp := map[string]interface{}{
		"order_id":        o.ID,
		"symbol":          o.Symbol,
		"side":            o.Side,
		"type":            o.Type,
		"price":           o.Price,
		"quantity":        o.Quantity + o.Filled,
		"filled_quantity": o.Filled,
		"remaining":       o.Quantity - o.Filled,
	}

	writeJSON(w, http.StatusOK, resp)
}

// CancelOrderHandler handles DELETE /api/v1/orders/{id}
func CancelOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path)

	o, err := orderStore.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	// If filled completely already
	if o.Filled >= o.Quantity {
		writeError(w, http.StatusBadRequest, "cannot cancel a fully filled order")
		return
	}

	// Remove from in-memory order store
	if err := orderStore.Remove(id); err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	// ALSO remove from the orderbook (price level) so book view is consistent.
	// This is safe in Step 6 (single-threaded). In Step 7, shard-owner will do this.
	ob := getOrCreateBook(o.Symbol)
	var sideMap map[int64]*engine.PriceLevel
	if o.Side == model.BUY {
		sideMap = ob.Bids
	} else {
		sideMap = ob.Asks
	}

	level, ok := sideMap[o.Price]
	if ok {
		// find and remove the order from the level.Orders slice
		newOrders := level.Orders[:0]
		for _, ord := range level.Orders {
			if ord.ID == o.ID {
				// skip — effectively removing this order
				continue
			}
			newOrders = append(newOrders, ord)
		}
		level.Orders = newOrders

		// if level empty, delete the price level map entry
		if len(level.Orders) == 0 {
			if o.Side == model.BUY {
				delete(ob.Bids, o.Price)
			} else {
				delete(ob.Asks, o.Price)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GetOrderBookHandler handles GET /api/v1/orderbook/{symbol}?depth=N
func GetOrderBookHandler(w http.ResponseWriter, r *http.Request) {
	symbol := pathParam(r.URL.Path)
	ob := getOrCreateBook(symbol)

	depth := 10
	if ds := r.URL.Query().Get("depth"); ds != "" {
		if d, err := strconv.Atoi(ds); err == nil && d > 0 {
			depth = d
		}
	}

	resp := map[string]interface{}{
		"symbol": symbol,
		"bids":   aggregate(ob.Bids, depth, false), // highest first
		"asks":   aggregate(ob.Asks, depth, true),  // lowest first
	}

	writeJSON(w, http.StatusOK, resp)
}

// ----------------- helpers -----------------

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func pathParam(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return parts[len(parts)-1]
}

func aggregate(side map[int64]*engine.PriceLevel, depth int, asc bool) []map[string]interface{} {
	if depth <= 0 {
		return []map[string]interface{}{}
	}

	prices := make([]int64, 0, len(side))
	for p := range side {
		prices = append(prices, p)
	}

	if asc {
		sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	} else {
		sort.Slice(prices, func(i, j int) bool { return prices[i] > prices[j] })
	}

	out := make([]map[string]interface{}, 0, depth)
	for _, p := range prices {
		if len(out) >= depth {
			break
		}
		level := side[p]
		total := int64(0)
		for _, o := range level.Orders {
			// remaining quantity per order
			total += (o.Quantity - o.Filled)
		}
		out = append(out, map[string]interface{}{
			"price":    p,
			"quantity": total,
		})
	}
	return out
}
