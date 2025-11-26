package metrics

import "sync/atomic"

// Simple process-level metrics used by the engine & api.
// Very small and intentionally minimal.

var ordersProcessed int64

// IncOrdersProcessed increments the counter by 1.
func IncOrdersProcessed() {
	atomic.AddInt64(&ordersProcessed, 1)
}

// AddOrdersProcessed increments the counter by n.
func AddOrdersProcessed(n int64) {
	atomic.AddInt64(&ordersProcessed, n)
}

// GetOrdersProcessed returns the current value.
func GetOrdersProcessed() int64 {
	return atomic.LoadInt64(&ordersProcessed)
}
