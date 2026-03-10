# TASK05 — SDK Auto-Detect Execution Mode and Transport Chain

## Objective

Each SDK detects its own execution mode at startup (binary,
framework, or WASM) and configures the connect transport chain
accordingly. No manual configuration needed.

## Repositories

All SDK repos:
- `go-holons`: `github.com/organic-programming/go-holons`
- `rust-holons`: `github.com/organic-programming/rust-holons`
- `cpp-holons`: `github.com/organic-programming/cpp-holons`
- `swift-holons`: `github.com/organic-programming/swift-holons`
- `kotlin-holons`: `github.com/organic-programming/kotlin-holons`
- `dart-holons`: `github.com/organic-programming/dart-holons`
- `dotnet-holons`: `github.com/organic-programming/dotnet-holons`
- `node-holons`: `github.com/organic-programming/node-holons`
- `python-holons`: `github.com/organic-programming/python-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Transport Selection

## Scope

### Transport chain per mode

| Mode | Connect chain |
|---|---|
| binary | `mem → stdio → unix → tcp → rest+sse` |
| framework | `mem → tcp → rest+sse` |
| wasm | `mem → rest+sse` |

### Detection logic

- **binary**: default when running as a standalone process
- **framework**: detected when loaded as a shared library
  (e.g., no `main()`, called via FFI)
- **wasm**: detected via build tags or runtime environment
  (`GOOS=js`, `wasm_bindgen`, Pyodide, etc.)
- **interpreted** (Node, Python): always binary mode unless
  running inside a WASM runtime

### SDK API

The connect chain auto-selects transports. Holons built as
frameworks or WASM don't try `stdio://` or `unix://` (unavailable
in those modes). No code change needed in holon source — the SDK
handles it.

## Acceptance Criteria

- [ ] Go SDK: binary mode → full chain, WASM → `mem → rest+sse`
- [ ] Rust SDK: same behavior
- [ ] C++ SDK: framework mode → `mem → tcp → rest+sse`
- [ ] Swift SDK: framework mode detection
- [ ] Kotlin SDK: framework mode detection
- [ ] Dart SDK: framework + WASM modes
- [ ] C# SDK: framework + WASM modes
- [ ] Node SDK: WASM mode (when running in WASI)
- [ ] Python SDK: WASM mode (when running in Pyodide)
- [ ] No config required — detection is automatic
- [ ] Existing binary holons unaffected

## Dependencies

TASK03 (Go cross-compilation must work first to test framework/WASM modes).
v0.6 (REST+SSE transport must exist).
