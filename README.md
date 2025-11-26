# Low-Latency Order Matching Engine (Go)

Minimal: sharded, in-memory order matching engine (limit + market), REST API, pprof, simple metrics.

## Prerequisites
- Go >= 1.25 installed and on PATH.
- Git.
- (Optional) Graphviz for pprof flamegraphs.

## Quick setup & build
git clone https://github.com/2019UGEC100/order-matching-engine-go.git
cd order-matching-engine-go
go mod tidy
go test ./...
go build ./cmd/server
go run ./cmd/server
Server: API on :8080, pprof on :6060.

## API Reference
POST /api/v1/orders
GET /api/v1/orders/{order_id}
DELETE /api/v1/orders/{order_id}
GET /api/v1/orderbook/{symbol}?depth=N
GET /health
GET /metrics

## Running & Testing
(go commands omitted for brevity)

## Load testing & p99 latency
Use cmd/load tool:
go run ./cmd/load -c 80 -n 2000 -sym LOAD

## Profiling
CPU: go tool pprof http://localhost:6060/debug/pprof/profile?seconds=20
Flamegraph: go tool pprof -http=:8081 ...

## Project layout
cmd/server
cmd/load
pkg/api
pkg/engine
pkg/model
pkg/metrics

## Submission checklist
go test ./...
go build ./cmd/server
Include all code + README
