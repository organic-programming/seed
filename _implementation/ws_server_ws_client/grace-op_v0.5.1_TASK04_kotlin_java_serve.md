# TASK04 — Kotlin + Java: Wire stdio/unix Dispatch in `Serve`

## Summary

Both `kotlin-holons/src/main/kotlin/org/organicprogramming/holons/Serve.kt` and
`java-holons/src/main/java/org/organicprogramming/holons/Serve.java` have an
accurate `Transport.listen()` that returns the correct `Listener` variant for
all schemes. The gap is that `Serve` never dispatches `unix` or `stdio`
— it either ignores them or falls through to an error.

## Target

| Transport | Before | After |
|-----------|:------:|:-----:|
| `tcp://` | ✅ | ✅ |
| `unix://` | ✅ (Transport) / ❌ (Serve) | ✅ |
| `stdio://` | ✅ (Transport) / ❌ (Serve) | ✅ |
| ws server | ⚠️ parse | 🚫 library (documented) |
| ws client | ❌ | ❌ |

## Implementation — Kotlin

In `Serve.kt`, `runWithOptions` currently does:

```kotlin
val listener = Transport.listen(listenUri)
when (listener) {
    is Transport.Listener.Tcp -> startTcpServer(listener.socket, ...)
    else -> throw UnsupportedOperationException("unsupported: $listenUri")
}
```

Replace with full dispatch:

```kotlin
is Transport.Listener.Tcp   -> startTcpServer(listener.socket, builder)
is Transport.Listener.Unix  -> startUnixServer(listener.channel, builder)
is Transport.Listener.Stdio -> startStdioServer(builder)   // stdin/stdout pipe
is Transport.Listener.WS    -> throw UnsupportedOperationException(
    "ws:// server is not supported by the JVM gRPC library")
```

- **`unix`** — `NettyServerBuilder.forAddress(DomainSocketAddress(path))` (Netty transport with epoll/kqueue).
- **`stdio`** — pipe `System.in` / `System.out` into a `ServerBuilder` via a `PipedInputStream` bridge, same approach as Python's `_StdioServeBridge`; or use a temp Unix socket pair (simpler on JVM).

## Implementation — Java

Same dispatch in `Serve.java`, mirroring the Kotlin implementation.

## Acceptance Criteria

### Kotlin
- [ ] `./gradlew test` passes with:
  - `ServeTest.tcpRoundTrip` (existing)
  - `ServeTest.unixRoundTrip`
  - `ServeTest.stdioRoundTrip`
- [ ] `ws://` throws a clear `UnsupportedOperationException` with a message explaining the library constraint

### Java
- [ ] `./gradlew test` passes with equivalent tests in `ServeTest.java`

## Dependencies

`sdk/kotlin-holons`, `sdk/java-holons` only.  
Requires `grpc-netty` (or `grpc-netty-shaded`) for unix domain socket support on JVM.
