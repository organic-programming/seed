# TASK01 — Swift: Complete Transport Coverage

## Summary

`sdk/swift-holons/Sources/Holons/Serve.swift` currently has an explicit guard
that rejects every transport except `tcp://`:

```swift
guard parsed.scheme == "tcp" else {
    throw TransportError.runtimeUnsupported(
        uri: listenURI,
        reason: "Serve.run(...) currently supports tcp:// only"
    )
}
```

Remove this guard and implement `unix://` and `stdio://` using SwiftNIO
primitives. Also add a ws dial path so Swift HostUIs can reach ws-served daemons.

## Target

| Transport | Before | After |
|-----------|:------:|:-----:|
| `tcp://` | ✅ | ✅ |
| `unix://` | ❌ | ✅ |
| `stdio://` | ❌ | ✅ |
| ws server | ❌ | 🚫 library |
| ws client (dial) | ❌ | ✅ |

## Implementation

### `Serve.startWithOptions`

Replace the tcp-only guard with a dispatch on `parsed.scheme`:

- **`unix://`** — `ServerBootstrap` on a `UnixDomainSocketAddress`. Clean stale socket on start and shutdown.
- **`stdio://`** — wrap `FileHandle.standardInput` / standard output as a NIO `Channel` using `NIOPipeBootstrap`. Single-connection semantics: accept once, then stop the listener loop.

### `Transport.swift`

`Transport.parse` already handles all schemes. No change needed.

### `connect()` ws dial

In `sdk/swift-holons/Sources/Holons/Connect.swift`, when the resolved URI scheme is `ws` or `wss`, open a WebSocket channel using `NIOHTTP1` + `WebSocketUpgradeClientHandler` (available in `swift-nio-extras`). Wrap the resulting channel as a `GRPCChannel` using `ClientConnection.usingTLSBackedByNIOSSL` / plain variant.

## Acceptance Criteria

- [ ] `swift test` passes, including:
  - `ServeTests.testUnixRoundTrip` — bind on `unix:///tmp/holons-test.sock`, connect, call Describe
  - `ServeTests.testStdioAcceptsOne` — verify serve loop terminates after one connection closes
- [ ] `op build recipes/daemons/gudule-daemon-greeting-swift` (from v0.4.2) still passes
- [ ] `--listen stdio://` reaches `SayHello` when piped from a Go test

## Dependencies

`sdk/swift-holons` only. No recipe changes.
