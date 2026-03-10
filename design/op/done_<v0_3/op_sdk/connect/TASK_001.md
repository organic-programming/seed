# TASK_001 — Fix Go Reference `connect` (stdio default + tests)

## Context

`go-holons` is the reference implementation. It must be correct first
because all other SDKs are ports of it.

**Repository**: `organic-programming/sdk/go-holons`
**File**: `pkg/connect/connect.go`

## What is wrong

1. `Connect()` defaults to `transport: "tcp"` (line 54). The spec
   says `"stdio"`.
2. The test suite (`connect_test.go`) only tests TCP paths. There is
   **no test** for the default stdio path.
3. The test server `runServeProcess()` only handles TCP — it does not
   support `serve --listen stdio://`.

## What to do

### 1. Fix the default transport

```go
// BEFORE (line 54)
Transport: "tcp",

// AFTER
Transport: "stdio",
```

### 2. Add stdio test

Add `TestConnectStartsSlugViaStdioByDefault` that:
- Creates a holon fixture.
- Calls `Connect(slug)` with no options.
- Asserts the child was started with `serve --listen stdio://`.
- Asserts gRPC round-trip works over the stdio pipe.
- Calls `Disconnect()`, asserts process is stopped.
- Asserts no port file was written.

### 3. Update `runServeProcess` to support stdio

The test binary doubles as a fake holon. When started with
`--listen stdio://` it must serve gRPC over stdin/stdout.

### 4. Run ALL existing tests

After changes, every existing test must still pass:
- `TestConnectDirectTCPRoundTrip`
- `TestConnectStartsSlugEphemerally` (now uses stdio by default)
- `TestConnectWithOptsWritesPortFileAndLeavesProcessRunning`
- `TestConnectReusesExistingPortFile`
- `TestConnectRemovesStalePortFileAndStartsFresh`

## Verification

```bash
cd sdk/go-holons && go test -v -count=1 ./pkg/connect/...
```

All 6 tests (5 existing + 1 new stdio test) must pass.

## Rules

- Do not change the `ConnectWithOpts` API.
- Do not break the TCP path — it still works with explicit
  `ConnectOptions{Transport: "tcp"}`.
- Commit only when all tests pass.
