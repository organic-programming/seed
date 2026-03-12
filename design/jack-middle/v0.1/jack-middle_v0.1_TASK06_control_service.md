# TASK06 — Control Proto and MiddleService

## Context

Jack exposes a small control service so operators can query
proxy status, list recorded calls, and reconfigure middleware
at runtime.

## Objective

Define the `middle.v1.MiddleService` proto and implement its
server handlers.

## Changes

### `protos/middle/v1/middle.proto` [NEW]

```protobuf
syntax = "proto3";
package middle.v1;

service MiddleService {
  rpc Status(StatusRequest) returns (StatusResponse);
  rpc ListRecords(ListRecordsRequest) returns (ListRecordsResponse);
  rpc SetMiddleware(SetMiddlewareRequest) returns (SetMiddlewareResponse);
}

message StatusRequest {}

message StatusResponse {
  string target = 1;
  string listen_address = 2;
  repeated string middleware = 3;
  int64 total_calls = 4;
  int64 total_errors = 5;
  map<string, MethodStat> methods = 6;
}

message MethodStat {
  int64 count = 1;
  int64 errors = 2;
  double avg_ms = 3;
}

message ListRecordsRequest {
  int32 limit = 1;
  string method_filter = 2;
}

message ListRecordsResponse {
  repeated RecordEntry records = 1;
}

message RecordEntry {
  string method = 1;
  string timestamp = 2;
  double duration_ms = 3;
  string status = 4;
  string file_path = 5;
}

message SetMiddlewareRequest {
  repeated string middleware = 1;
}

message SetMiddlewareResponse {
  repeated string active_middleware = 1;
}
```

### `internal/service/service.go` [NEW]

Implement `MiddleService` handlers backed by the `Stats`
and `Recorder` state from the middleware.

### `cmd/jack-middle/main.go`

Register `MiddleService` alongside the `UnknownServiceHandler`.

## Acceptance Criteria

- [ ] `Status` returns target, address, middleware list, and per-method stats
- [ ] `ListRecords` lists recorded files with optional method filter
- [ ] `SetMiddleware` hot-reloads the middleware chain
- [ ] `MiddleService` RPCs are not forwarded to the backend

## Dependencies

TASK03 (metrics for stats), TASK04 (recorder for records), TASK05 (CLI wires it).
