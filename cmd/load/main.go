package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Req struct {
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`
	Type     string `json:"type"`
	Price    int64  `json:"price"`
	Quantity int64  `json:"quantity"`
}

func main() {
	var (
		url     = flag.String("url", "http://localhost:8080/api/v1/orders", "orders endpoint")
		conns   = flag.Int("c", 50, "concurrency")
		total   = flag.Int("n", 1000, "total requests")
		symbol  = flag.String("sym", "LOAD", "symbol")
		sleepMs = flag.Int("sleep", 0, "ms sleep between requests per goroutine")
	)
	flag.Parse()

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	reqsPerWorker := (*total + *conns - 1) / *conns

	start := time.Now()

	for i := 0; i < *conns; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < reqsPerWorker; j++ {
				r := Req{
					Symbol:   *symbol,
					Side:     "BUY",
					Type:     "LIMIT",
					Price:    100 + int64(id%10),
					Quantity: 1,
				}

				b, _ := json.Marshal(r)

				req, _ := http.NewRequest("POST", *url, bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")

				_, err := client.Do(req)
				if err != nil {
					fmt.Printf("err: %v\n", err)
				}

				if *sleepMs > 0 {
					time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()

	d := time.Since(start)
	fmt.Printf("done: total=%d concurrency=%d duration=%v req/s=%.2f\n", *total, *conns, d, float64(*total)/d.Seconds())
}
