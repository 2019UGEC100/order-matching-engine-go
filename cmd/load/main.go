package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
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

type StatsSummary struct {
	TotalRequests int     `json:"total_requests"`
	Concurrency   int     `json:"concurrency"`
	DurationSec   float64 `json:"duration_sec"`
	ReqPerSec     float64 `json:"req_per_sec"`
	MeanMs        float64 `json:"mean_ms"`
	MaxMs         float64 `json:"max_ms"`
	P50Ms         float64 `json:"p50_ms"`
	P90Ms         float64 `json:"p90_ms"`
	P99Ms         float64 `json:"p99_ms"`
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	var (
		urlFlag   = flag.String("url", "http://localhost:8080/api/v1/orders", "orders endpoint")
		conns     = flag.Int("c", 50, "concurrency (goroutines)")
		total     = flag.Int("n", 1000, "total requests")
		symbol    = flag.String("sym", "LOAD", "symbol")
		sleepMs   = flag.Int("sleep", 0, "ms sleep between requests per goroutine")
		statsMode = flag.Bool("stats", false, "record per-request latency and print p50/p90/p99")
	)
	flag.Parse()

	// Prefer IPv4 loopback so Windows doesn't pick ::1 unexpectedly
	if *urlFlag == "http://localhost:8080/api/v1/orders" {
		*urlFlag = "http://127.0.0.1:8080/api/v1/orders"
	}

	// Seed RNG for jitter
	rand.Seed(time.Now().UnixNano())

	// Tuned transport for heavy load and connection reuse
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:        0, // unlimited global idle conns
		MaxIdleConnsPerHost: maxInt(*conns, 1000),
		MaxConnsPerHost:     maxInt(*conns, 1000),
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		ForceAttemptHTTP2:   false,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Prepare workload distribution
	reqsPerWorker := (*total + *conns - 1) / *conns
	var wg sync.WaitGroup

	// Stats collection
	var mu sync.Mutex
	durations := make([]float64, 0, *total) // ms

	start := time.Now()

	worker := func(id int) {
		defer wg.Done()
		for j := 0; j < reqsPerWorker; j++ {
			// stop early if we've already recorded enough requests
			mu.Lock()
			sent := len(durations)
			mu.Unlock()
			if sent >= *total {
				return
			}

			r := Req{
				Symbol:   *symbol,
				Side:     "BUY",
				Type:     "LIMIT",
				Price:    100 + int64(id%10),
				Quantity: 1,
			}

			// marshal once per logical request
			b, _ := json.Marshal(r)

			// record start if in stats mode
			var t0 time.Time
			if *statsMode {
				t0 = time.Now()
			}

			// Retry loop with fresh http.Request per attempt
			var resp *http.Response
			var err error
			maxRetries := 5
			baseDelay := 50 * time.Millisecond

			for attempt := 0; attempt <= maxRetries; attempt++ {
				// create a fresh request for this attempt (new Body reader)
				req, _ := http.NewRequest("POST", *urlFlag, bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")

				resp, err = client.Do(req)
				if err == nil {
					break
				}
				// backoff with jitter
				backoff := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
				jitter := time.Duration(rand.Int63n(int64(backoff/2) + 1))
				if rand.Intn(2) == 0 {
					backoff -= jitter
				} else {
					backoff += jitter
				}
				time.Sleep(backoff)
			}

			// close response body if present
			if err == nil && resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}

			// record timing if statsMode
			if *statsMode {
				elapsed := time.Since(t0).Seconds() * 1000.0 // ms
				mu.Lock()
				durations = append(durations, elapsed)
				mu.Unlock()
			}

			// log error if it persisted after retries
			if err != nil {
				fmt.Fprintf(os.Stderr, "request error: %v\n", err)
			}

			if *sleepMs > 0 {
				time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
			}
			// --- REPLACEMENT END ---

		}
	}

	// Launch workers
	for i := 0; i < *conns; i++ {
		wg.Add(1)
		go worker(i)
	}

	wg.Wait()
	elapsedTotal := time.Since(start).Seconds()
	sent := len(durations)

	// If statsMode=false, print a simple summary and exit
	if !*statsMode {
		fmt.Printf("done: total=%d concurrency=%d duration=%v req/s=%.2f\n",
			*total, *conns, time.Duration(elapsedTotal*1e9), float64(*total)/elapsedTotal)
		return
	}

	// Trim durations if extra entries due to concurrent append
	if sent > *total {
		durations = durations[:*total]
		sent = *total
	}

	// Compute stats
	sort.Float64s(durations)
	var sum float64
	var max float64
	for _, v := range durations {
		sum += v
		if v > max {
			max = v
		}
	}
	mean := 0.0
	if sent > 0 {
		mean = sum / float64(sent)
	}

	p := func(q float64) float64 {
		if sent == 0 {
			return 0
		}
		// nearest-rank style
		idx := int(math.Floor(q*float64(sent-1) + 0.5))
		if idx < 0 {
			idx = 0
		}
		if idx >= sent {
			idx = sent - 1
		}
		return durations[idx]
	}

	summary := StatsSummary{
		TotalRequests: sent,
		Concurrency:   *conns,
		DurationSec:   elapsedTotal,
		ReqPerSec:     float64(sent) / elapsedTotal,
		MeanMs:        mean,
		MaxMs:         max,
		P50Ms:         p(0.50),
		P90Ms:         p(0.90),
		P99Ms:         p(0.99),
	}

	// Print plain
	fmt.Printf("SUMMARY: total=%d concurrency=%d duration=%.2fs req/s=%.2f\n",
		summary.TotalRequests, summary.Concurrency, summary.DurationSec, summary.ReqPerSec)
	fmt.Printf("LATENCY(ms): mean=%.3f max=%.3f p50=%.3f p90=%.3f p99=%.3f\n",
		summary.MeanMs, summary.MaxMs, summary.P50Ms, summary.P90Ms, summary.P99Ms)

	// Print JSON
	js, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Printf("\nJSON:\n%s\n", string(js))
}
