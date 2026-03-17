# Drop `holon.yaml` — Task Sequence

> **Goal:** Remove all `holon.yaml` support from the codebase.
> `holon.proto` is the only manifest format going forward.
> No retro-compatibility, no fallback.

| #  | File | Title | Scope | Status |
|----|------|-------|-------|--------|
| 01 | [TASK01](./design/drop-yaml/TASK01_grace_op_identity.md) | grace-op: identity model → proto-only | `holons/grace-op/internal/identity/` | — |
| 02 | [TASK02](./design/drop-yaml/TASK02_grace_op_manifest.md) | grace-op: manifest loading → proto-only | `holons/grace-op/internal/holons/manifest.go` | — |
| 03 | [TASK03](./design/drop-yaml/TASK03_grace_op_discovery.md) | grace-op: discovery + lifecycle → proto-only | `holons/grace-op/internal/holons/discovery.go`, `lifecycle.go` | — |
| 04 | [TASK04](./design/drop-yaml/TASK04_grace_op_who_scaffold_mod.md) | grace-op: who, scaffold, mod → proto-only | `internal/who/`, `internal/scaffold/`, `internal/mod/` | — |
| 05 | [TASK05](./design/drop-yaml/TASK05_grace_op_cli_tests.md) | grace-op: update all CLI + server tests | `internal/cli/*_test.go`, `internal/server/`, `internal/suggest/` | — |
| 06 | [TASK06](./design/drop-yaml/TASK06_go_sdk.md) | go-holons SDK → proto-only | `sdk/go-holons/` | — |
| 07 | [TASK07](./design/drop-yaml/TASK07_swift_sdk.md) | swift-holons SDK → proto-only | `sdk/swift-holons/` | — |
| 08 | [TASK08](./design/drop-yaml/TASK08_other_sdks.md) | Remaining SDKs → proto-only | `sdk/{c,cpp,rust,java,kotlin,ruby,python,dart,js,js-web,csharp}-holons/` | — |
| 09 | [TASK09](./design/drop-yaml/TASK09_examples.md) | Active examples → proto-only | `examples/hello-world/` | — |
| 10 | [TASK10](./design/drop-yaml/TASK10_docs_protos.md) | Top-level docs + proto comments | `*.md`, `_protos/`, generated stubs | — |

## Ordering Rules

- **01–05** are grace-op internals — do them first, in order (each depends on the previous).
- **06–08** are SDK tasks — independent of each other, can run in parallel after 01–05.
- **09** depends on 07 (Swift SDK changes affect SwiftUI example).
- **10** is final — update docs after code is settled.

## Out of Scope (ported separately)

- `holons/jess-npm/`
- `holons/rob-go/`
- `holons/wisupaa-whisper/`
- `examples/legacy/`
