# TASK002 — Implement `connect` in `swift-holons`

## Context

The Organic Programming SDK fleet requires a `connect` module in every SDK.
`connect` composes discover + start + dial into a single name-based resolution
primitive. See `AGENT.md` Article 11 "Connect — Name-Based Resolution".

The **reference implementation** is `go-holons/pkg/connect/connect.go` — study
it before starting.

## Workspace

- SDK root: `sdk/swift-holons/`
- Existing modules: `Sources/Holons/Discover.swift`, `Sources/Holons/Identity.swift`,
  `Sources/Holons/Serve.swift`, `Sources/Holons/Transport.swift`
- Reference: `sdk/go-holons/pkg/connect/connect.go`
- Spec: `sdk/TODO_CONNECT.md` § `swift-holons`

## What to implement

Create `Sources/Holons/Connect.swift`.

### Public API

```swift
public func connect(_ target: String) throws -> GRPCChannel
public func connect(_ target: String, options: ConnectOptions) throws -> GRPCChannel
public func disconnect(_ channel: GRPCChannel) throws
```

### Resolution logic

Same as reference (see TASK001 for the 3-step algorithm):
target contains `:` → direct dial; else → discover → port file check → start → dial.

### Process management

- Use `Foundation.Process` to launch the binary.
- Track started processes for cleanup on disconnect.
- Ephemeral mode: disconnect stops the process (SIGTERM → 2s → SIGKILL).

### Port file convention

Path: `$CWD/.op/run/<slug>.port`
Content: `tcp://127.0.0.1:<port>\n`

## Testing

1. Direct dial test
2. Slug resolution test (with temp holon tree)
3. Port file reuse test
4. Stale port file cleanup test

Follow `swift test` patterns. All existing tests must pass.

## Rules

- Follow existing code style in `Discover.swift` and `Transport.swift`.
- Do not modify existing files except to add `connect` export if needed.
- Use `grpc-swift` `ClientConnection` for the channel.
