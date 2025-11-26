package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/metrics"
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
		"orders_processed": metrics.GetOrdersProcessed(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// MetricsHandler returns simple metrics (JSON)
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"orders_processed": metrics.GetOrdersProcessed(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
