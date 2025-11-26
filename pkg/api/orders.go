package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/engine"
	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

// router is set by Init
var (
	router     *engine.Router
	idToSymbol = struct {
		mu sync.RWMutex
		m  map[string]string
	}{m: make(map[string]string)}
)

// Init wires the API package to the engine Router.
// Call this once at server startup.
func Init(r *engine.Router) {
	router = r
}

// -------------------------------
// POST /api/v1/orders
// -------------------------------
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

	// Submit to router (which routes to correct shard)
	if router == nil {
		writeError(w, http.StatusInternalServerError, "router not initialized")
		return
	}

	res := router.SubmitOrder(&req)
	if res.Err != "" {
		writeError(w, http.StatusBadRequest, res.Err)
		return
	}

	// If order is now resting in book (limit with remaining), record id->symbol mapping
	if req.Type == model.LIMIT && req.Quantity > 0 {
		idToSymbol.mu.Lock()
		idToSymbol.m[req.ID] = req.Symbol
		idToSymbol.mu.Unlock()
	}

	// ensure trades_executed is [] not null
	var tradesResp []model.Order
	if res.Trades == nil {
		tradesResp = []model.Order{}
	} else {
		tradesResp = res.Trades
	}

	resp := map[string]interface{}{
		"order_id":        res.Order.ID,
		"symbol":          res.Order.Symbol,
		"side":            res.Order.Side,
		"type":            res.Order.Type,
		"price":           res.Order.Price,
		"quantity":        res.Order.Quantity + res.Order.Filled,
		"filled_quantity": res.Order.Filled,
		"remaining":       res.Order.Quantity,
		"trades_executed": tradesResp,
	}

	writeJSON(w, res.StatusCode, resp)
}

// -------------------------------
// GET/DELETE dispatcher
// -------------------------------
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

// -------------------------------
// GET /api/v1/orders/{id}
// -------------------------------
func GetOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path)

	// lookup symbol
	idToSymbol.mu.RLock()
	sym, ok := idToSymbol.m[id]
	idToSymbol.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	res := router.GetOrder(sym, id)
	if res.Err != "" || res.Order == nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	o := res.Order
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

// -------------------------------
// DELETE /api/v1/orders/{id}
// -------------------------------
func CancelOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r.URL.Path)

	// lookup symbol
	idToSymbol.mu.RLock()
	sym, ok := idToSymbol.m[id]
	idToSymbol.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	res := router.CancelOrder(sym, id)
	if !res.OK {
		writeError(w, http.StatusBadRequest, res.Err)
		return
	}

	// remove mapping
	idToSymbol.mu.Lock()
	delete(idToSymbol.m, id)
	idToSymbol.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// -------------------------------
// GET /api/v1/orderbook/{symbol}?depth=N
// -------------------------------
func GetOrderBookHandler(w http.ResponseWriter, r *http.Request) {
	if router == nil {
		writeError(w, http.StatusInternalServerError, "router not initialized")
		return
	}

	symbol := pathParam(r.URL.Path)
	depth := 10
	if ds := r.URL.Query().Get("depth"); ds != "" {
		if d, err := strconv.Atoi(ds); err == nil && d > 0 {
			depth = d
		}
	}

	snap := router.GetOrderBook(symbol, depth)

	resp := map[string]interface{}{
		"symbol": symbol,
		"bids":   snap.Bids,
		"asks":   snap.Asks,
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
