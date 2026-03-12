# TASK03 — Built-in Middleware: Logger and Metrics

## Context

The first two middleware provide immediate value for debugging
and profiling holon interactions.

## Objective

Implement the `logger` and `metrics` middleware.

## Changes

### `internal/middleware/logger.go` [NEW]

Logs every RPC with method, duration, status, and payload sizes.

```go
func Logger(w io.Writer) Interceptor
```

Output format (one line per call):
```
2026-03-12T08:00:00Z  /go.v1.GoService/Build  OK  42ms  req=128B  resp=256B
```

### `internal/middleware/metrics.go` [NEW]

Tracks per-method counters and latency histograms in memory.

```go
// Metrics returns an interceptor that collects RPC statistics.
func Metrics() (Interceptor, *Stats)

// Stats holds per-method counters.
type Stats struct {
    mu      sync.RWMutex
    Methods map[string]*MethodStats
}

type MethodStats struct {
    Count    int64
    Errors   int64
    TotalMs  float64
    MinMs    float64
    MaxMs    float64
}
```

### Tests

- `TestLoggerOutput` — verify log line format
- `TestMetricsCount` — verify call counting
- `TestMetricsLatency` — verify min/max tracking

## Acceptance Criteria

- [ ] Logger writes one line per RPC to the configured writer
- [ ] Metrics tracks count, errors, and latency per method
- [ ] Both middleware are composable in a chain

## Dependencies

TASK02 (middleware interface).
