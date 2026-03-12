# TASK01 — Proto: UpdateToolchain and ToolchainInfo RPCs

## Context

v0.2 introduces two new RPCs to `go.v1.GoService`: one to
replace the active Go version, one to query it.

## Objective

Add proto definitions and regenerate Go stubs.

## Changes

### `protos/go/v1/go.proto`

Add messages and RPC methods:

```protobuf
message UpdateToolchainRequest {
  string target_version = 1;  // semver (e.g. "1.25.0") or "latest"
}

message UpdateToolchainResponse {
  string previous_version = 1;
  string current_version = 2;
  string changelog_url = 3;
  bool   was_noop = 4;
}

message ToolchainInfoRequest {}

message ToolchainInfoResponse {
  string version = 1;
  string goroot = 2;
  string goos = 3;
  string goarch = 4;
  bool   cgo_enabled = 5;
}

service GoService {
  // ...existing RPCs...
  rpc UpdateToolchain(UpdateToolchainRequest) returns (UpdateToolchainResponse);
  rpc ToolchainInfo(ToolchainInfoRequest) returns (ToolchainInfoResponse);
}
```

### `gen/go/go/v1/` (regenerated)

Run `op generate` or `protoc` to regenerate Go stubs.

## Acceptance Criteria

- [ ] Proto compiles without errors
- [ ] Generated Go stubs include both new RPCs
- [ ] Existing RPCs unchanged

## Dependencies

None.
