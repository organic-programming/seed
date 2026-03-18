# TASK04 — Built-in Middleware: Recorder and Fault Injection

## Context

Two advanced middleware for testing and replay scenarios.

## Objective

Implement the `recorder` and `fault` middleware.

## Changes

### `internal/middleware/recorder.go` [NEW]

Records full request/response payloads to disk for replay.

```go
// Recorder returns an interceptor that writes each call to
// a JSON file in the specified directory.
func Recorder(dir string) Interceptor
```

Record format (one JSON file per call):
```json
{
  "method": "/go.v1.GoService/Build",
  "timestamp": "2026-03-12T08:00:00Z",
  "duration_ms": 42,
  "request_b64": "...",
  "response_b64": "...",
  "status": "OK"
}
```

### `internal/middleware/fault.go` [NEW]

Injects synthetic errors at a configurable rate.

```go
// Fault returns an interceptor that fails a fraction of RPCs
// with the given gRPC status code.
func Fault(rate float64, code codes.Code) Interceptor
```

### Tests

- `TestRecorderWritesFile` — verify JSON file created
- `TestRecorderPayloadCapture` — verify request/response captured
- `TestFaultInjection` — verify error returned at expected rate
- `TestFaultZeroRate` — verify no errors when rate is 0.0

## Acceptance Criteria

- [ ] Recorder writes one JSON file per call
- [ ] Payloads are base64-encoded raw protobuf
- [ ] Fault middleware injects errors at the configured rate
- [ ] Fault middleware passes through when rate is 0.0

## Dependencies

TASK02 (middleware interface).
