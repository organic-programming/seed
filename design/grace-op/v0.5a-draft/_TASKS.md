# Grace-OP Draft Roadmap — v0.5 Transport Completion + v0.5a `mem://` Validation

> Draft only.
> This folder does not modify, replace, or invalidate the existing
> `v0.5/` or `v0.5.1/` documents. It records a new roadmap proposal
> beside them so the current material remains reusable as-is.

## Milestones

### v0.5 — Definitive Transport Completion

In this draft roadmap, **v0.5** is the milestone that definitively
implements every transport that is expected from each supported SDK.

That includes the necessary `serve`, `transport`, and `connect`
behavior for the SDK to reach its intended transport surface.

This folder does **not** restate the detailed v0.5 work items. It
only fixes the milestone intent so the follow-up validation work in
v0.5a has a clear dependency.

### v0.5a — Validate `mem://` in `/examples`

After v0.5 transport completion, **v0.5a** validates that **`op` can
launch a mem-to-mem holon flow in any supported language**.

The example lives under one shared root:

- `examples/mem-ping-pong/`

That shared example contains one language-specific implementation per
supported native SDK:

- `examples/mem-ping-pong/c/`
- `examples/mem-ping-pong/cpp/`
- `examples/mem-ping-pong/csharp/`
- `examples/mem-ping-pong/dart/`
- `examples/mem-ping-pong/go/`
- `examples/mem-ping-pong/java/`
- `examples/mem-ping-pong/js/`
- `examples/mem-ping-pong/kotlin/`
- `examples/mem-ping-pong/python/`
- `examples/mem-ping-pong/ruby/`
- `examples/mem-ping-pong/rust/`
- `examples/mem-ping-pong/swift/`

Each example validates that:

- Two logical holons of the **same language** can compose inside one
  OS process over `mem://`.
- `op` can launch the language implementation successfully.
- The caller reaches its peer through the language SDK's official
  `connect(slug)` path, not a raw gRPC dial.
- The ping-pong party performs exactly **1000 turns**.
- The initial value is `0`.
- The final value is therefore **`1000`**.
- The example emits elapsed time in a structured, machine-readable
  form.

## Execution Strategy

- Task numbering is inventory only, not execution priority.
- v0.5a starts only after v0.5 transport completion.
- **TASK05 (Go)** runs first and serves as the v0.5a reference
  implementation.
- After TASK05 is validated, the remaining language tasks may proceed
  in parallel using the Go example as the reference for structure,
  SDK usage, RPC semantics, output shape, and test expectations.
- `op` is part of the validation target, not just a convenience tool.
- Every example MUST import and use its matching SDK directly.
- If an example exposes a real SDK gap, the SDK may be modified to
  fix it. Example-specific transport bypasses or raw gRPC workarounds
  do not satisfy this milestone.
- All tasks follow the shared design note:
  [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 05 | [TASK05](./grace-op_v0.5a-draft_TASK05_go_mem_ping_pong.md) | Go implementation in `examples/mem-ping-pong/go/` (**reference implementation**) | v0.5 |
| 01 | [TASK01](./grace-op_v0.5a-draft_TASK01_c_mem_ping_pong.md) | C implementation in `examples/mem-ping-pong/c/` | v0.5 |
| 02 | [TASK02](./grace-op_v0.5a-draft_TASK02_cpp_mem_ping_pong.md) | C++ implementation in `examples/mem-ping-pong/cpp/` | v0.5 |
| 03 | [TASK03](./grace-op_v0.5a-draft_TASK03_csharp_mem_ping_pong.md) | C# implementation in `examples/mem-ping-pong/csharp/` | v0.5 |
| 04 | [TASK04](./grace-op_v0.5a-draft_TASK04_dart_mem_ping_pong.md) | Dart implementation in `examples/mem-ping-pong/dart/` | v0.5 |
| 06 | [TASK06](./grace-op_v0.5a-draft_TASK06_java_mem_ping_pong.md) | Java implementation in `examples/mem-ping-pong/java/` | v0.5 |
| 07 | [TASK07](./grace-op_v0.5a-draft_TASK07_js_mem_ping_pong.md) | JavaScript implementation in `examples/mem-ping-pong/js/` | v0.5 |
| 08 | [TASK08](./grace-op_v0.5a-draft_TASK08_kotlin_mem_ping_pong.md) | Kotlin implementation in `examples/mem-ping-pong/kotlin/` | v0.5 |
| 09 | [TASK09](./grace-op_v0.5a-draft_TASK09_python_mem_ping_pong.md) | Python implementation in `examples/mem-ping-pong/python/` | v0.5 |
| 10 | [TASK10](./grace-op_v0.5a-draft_TASK10_ruby_mem_ping_pong.md) | Ruby implementation in `examples/mem-ping-pong/ruby/` | v0.5 |
| 11 | [TASK11](./grace-op_v0.5a-draft_TASK11_rust_mem_ping_pong.md) | Rust implementation in `examples/mem-ping-pong/rust/` | v0.5 |
| 12 | [TASK12](./grace-op_v0.5a-draft_TASK12_swift_mem_ping_pong.md) | Swift implementation in `examples/mem-ping-pong/swift/` | v0.5 |

## Dependency Graph

```text
v0.5 transport completion
        |
        +--> TASK05 Go (reference)
                |
                +--> TASK01 C
                +--> TASK02 C++
                +--> TASK03 C#
                +--> TASK04 Dart
                +--> TASK06 Java
                +--> TASK07 JavaScript
                +--> TASK08 Kotlin
                +--> TASK09 Python
                +--> TASK10 Ruby
                +--> TASK11 Rust
                +--> TASK12 Swift
```
