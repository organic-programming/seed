# TASK006 — Implement `connect` in `cpp-holons`

## Context

The Organic Programming SDK fleet requires a `connect` module in every SDK.
`cpp-holons` uses a header-based architecture in `include/holons/`.

The **reference implementation** is `go-holons/pkg/connect/connect.go` — study
it before starting.

## Workspace

- SDK root: `sdk/cpp-holons/`
- Existing files: `include/holons/holons.hpp` (main header with discover, transport, etc.)
- Reference: `sdk/go-holons/pkg/connect/connect.go`
- Spec: `sdk/TODO_CONNECT.md` § `cpp-holons`

## What to implement

Create `include/holons/connect.hpp` (or add to `holons.hpp` if the existing
pattern keeps everything in one file — check first).

### Public API

```cpp
namespace holons {
  std::shared_ptr<grpc::Channel> connect(const std::string& target);
  std::shared_ptr<grpc::Channel> connect(const std::string& target, const ConnectOptions& opts);
  void disconnect(std::shared_ptr<grpc::Channel> channel);

  struct ConnectOptions {
    int timeout_ms = 5000;
    std::string transport = "stdio"; // "tcp" for explicit override
    bool start = true;
    std::string port_file;
  };
}
```

### Resolution logic

Same 3-step algorithm:
1. `target` contains `:` → direct dial via `grpc::CreateChannel`.
2. Else → slug → discover → port file → start → dial.

### Process management

- Use `popen()` or `fork()`/`exec()` for process launch.
- Track started processes in a `static std::map`.
- `disconnect()`: close channel, if ephemeral → SIGTERM → 2s wait → SIGKILL.

### Port file convention

Path: `$CWD/.op/run/<slug>.port`
Content: `tcp://127.0.0.1:<port>\n`

## Testing

Add tests in `test/` following existing patterns.

## Rules

- Follow existing code style in `holons.hpp`.
- Use C++17 standard library (`std::filesystem`, `std::optional`).
- Use `grpc::CreateChannel` and `grpc::InsecureChannelCredentials`.
- Build with existing `CMakeLists.txt` — adjust if needed.
- All existing tests must still pass.
