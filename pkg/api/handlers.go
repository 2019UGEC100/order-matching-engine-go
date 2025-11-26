package api

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

var (
	startTime       = time.Now()
	ordersProcessed int64 // placeholder counter, increment later
)

// HealthHandler returns basic health information
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"status":           "ok",
		"uptime_sec":       int64(time.Since(startTime).Seconds()),
		"orders_processed": atomic.LoadInt64(&ordersProcessed),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// MetricsHandler returns simple metrics (JSON)
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"orders_processed": atomic.LoadInt64(&ordersProcessed),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
