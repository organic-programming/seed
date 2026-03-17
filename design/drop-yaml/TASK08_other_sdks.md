# TASK08 — Remaining SDKs → proto-only

## Scope

Apply the same pattern as TASK06/07 to each of:

| SDK | Key files | YAML dependency |
|-----|-----------|-----------------|
| `c-holons` | `holons.h` / `identity.c` | custom parser |
| `cpp-holons` | `holons.hpp`, `serve.hpp` | likely yaml-cpp |
| `rust-holons` | `describe.rs`, `serve.rs`, `identity.rs`, `discover.rs` | `serde_yaml` |
| `java-holons` | `Identity.java`, `Describe.java`, `Serve.java`, `Discover.java` | `snakeyaml` |
| `kotlin-holons` | `Identity.kt`, `Describe.kt`, `Serve.kt`, `Discover.kt` | `snakeyaml` |
| `ruby-holons` | `identity.rb`, `describe.rb`, `serve.rb`, `discover.rb` | stdlib `yaml` |
| `python-holons` | identity, describe, serve, discover modules | `pyyaml` |
| `dart-holons` | identity, describe, serve, discover | `yaml` package |
| `js-holons` | identity, describe, serve, discover | `js-yaml` or similar |
| `js-web-holons` | discover | manifest fetch |
| `csharp-holons` | identity, describe, serve, discover | `YamlDotNet` |

## Per-SDK checklist

For each SDK:

1. Remove `holon.yaml` parsing from identity module.
2. Remove `holonYAMLPath` parameter from describe/serve.
3. Remove YAML filename from discover scan.
4. Remove YAML library dependency if unused elsewhere.
5. Update tests to use `holon.proto`.
6. Update `README.md`.

## Verification

Each SDK's native build + test must pass.
