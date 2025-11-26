Performance Summary — Order Matching Engine

Test environment:
- Host: Windows 10 (local machine)
- Server: go run ./cmd/server (API :8080, pprof :6060)
- Load tool: cmd/load with -stats mode

1) Native Host Run — Baseline

Command:
go run ./cmd/load -c 100 -n 50000 -sym LOAD -sleep 1 -stats

Summary:
- total_requests: 50000
- concurrency: 100
- duration_sec: 6.4499
- req_per_sec: 7752.10
- mean_ms: 11.3895
- max_ms: 39.111
- p50_ms: 10.8292
- p90_ms: 15.0385
- p99_ms: 19.067

2) Low Concurrency Run

Command:
go run ./cmd/load -c 50 -n 20000 -sym LOAD -sleep 1 -stats

Summary:
- total_requests: 20000
- concurrency: 50
- duration_sec: 2.7949
- req_per_sec: 7155.83
- mean_ms: 5.2138
- max_ms: 20.6186
- p50_ms: 5.1899
- p90_ms: 7.0502
- p99_ms: 9.8207

3) Dockerized Server Benchmarks

Run X — Host → Docker

Command:
go run ./cmd/load -url http://127.0.0.1:8080/api/v1/orders -c 100 -n 20000 -sym LOAD -sleep 1 -stats

Summary:
- total_requests: 20000
- concurrency: 100
- duration_sec: 14.40
- req_per_sec: 1388.54
- mean_ms: 69.562
- p50_ms: 69.016
- p90_ms: 120.678
- p99_ms: 171.510

Run Y — Docker → Docker

Command:
docker compose run --rm load -url http://server:8080/api/v1/orders -c 50 -n 20000 -sym LOAD -sleep 1 -stats

Summary:
- total_requests: 20000
- concurrency: 50
- duration_sec: 7.0499
- req_per_sec: 2836.88
- mean_ms: 13.734
- p50_ms: 8.175
- p90_ms: 34.181
- p99_ms: 61.788

Final Recommendation:
Use host-native results for primary benchmarking. Docker results included for reproducibility.
