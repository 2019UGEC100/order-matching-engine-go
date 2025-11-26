package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/api"
	"github.com/2019UGEC100/order-matching-engine-go/pkg/engine"
)

func main() {
	// use all available CPUs
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Create router with N shards and buffer size 1024
	router := engine.NewRouter(runtime.NumCPU(), 1024)
	// Ensure graceful stop on exit
	defer router.Stop()

	// Initialize API package with router
	api.Init(router)

	// Start pprof server on :6060
	go func() {
		log.Println("pprof server on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Println("pprof listen error:", err)
		}
	}()

	mux := http.NewServeMux()
	// health & metrics handlers
	mux.HandleFunc("/health", api.HealthHandler)
	mux.HandleFunc("/metrics", api.MetricsHandler)

	// Orders API
	mux.HandleFunc("/api/v1/orders", api.CreateOrderHandler) // POST
	mux.HandleFunc("/api/v1/orders/", api.OrderByIDHandler)  // GET/DELETE by id
	mux.HandleFunc("/api/v1/orderbook/", api.GetOrderBookHandler)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on %s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v\n", err)
		}
	}()

	// Graceful shutdown on Ctrl+C
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	log.Println("shutting down server...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server Shutdown: %v", err)
	}
	log.Println("server stopped.")
}
