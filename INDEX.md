# Index

## Status Legend
| Symbol | Meaning |
|:---:|---|
| **✅** | Validated / Completed |
| **?** | Not yet verified / Draft |
| **-** | Not implemented / Not applicable |
| **⚠️** | Serious Issues known |
| **❌** | to be ignored |

## Specs (Root)

| Spec | Impl | Doc | Topic |
|:---:|:---:|---|---|
| ? | ? | [COAX.md](COAX.md) | Coaccessibility — every holon must be reachable |
| ✅ | ? | [COMMUNICATION.md](COMMUNICATION.md) | Holon communication protocol |
| ⚠️ | ? | [CONSTITUTION.md](CONSTITUTION.md) | Organic Programming paradigm — axiomatic foundation |
| ⚠️ | ? | [CONVENTIONS.md](CONVENTIONS.md) | Per-language holon directory structure |
| ✅ | - | [DISCOVERY.md](DISCOVERY.md) | Cross-SDK discovery algorithm and API |
| ? | - | [INSTANCES.md](INSTANCES.md) | Instance lifecycle and registry |
| ? | ? | [MCP.md](MCP.md) | MCP exposure via `op mcp` |
| ✅ | ? | [PACKAGE.md](PACKAGE.md) | `.holon` package format |
| ? | ? | [PROTO.md](PROTO.md) | `holon.proto` — the holon descriptor format |
| ? | ? | [README.md](README.md) | Root repository overview |
| ? | - | [SESSIONS.md](SESSIONS.md) | Network connection tracing and metrics |
| ? | ? | [TDD.md](TDD.md) | TDD promotion pipeline — unit → integration → regression |
| ⚠️ | ? | [WHY.md](WHY.md) | Rationale and guiding motivations |

## op CLI (holons/grace-op)

| Spec | Impl | Doc | Topic |
|:---:|:---:|---|---|
| ? | ? | [holons/grace-op/INSTALL.md](holons/grace-op/INSTALL.md) | `op` installation guide |
| ? | ? | [holons/grace-op/OP.md](holons/grace-op/OP.md) | `op` command reference |
| ? | ? | [holons/grace-op/OP_BUILD.md](holons/grace-op/OP_BUILD.md) | `op build` |
| ✅ | - | [holons/grace-op/OP_DISCOVERY.md](holons/grace-op/OP_DISCOVERY.md) | `op`-specific discovery expression routing |
| ? | ? | [holons/grace-op/OP_PROXY.md](holons/grace-op/OP_PROXY.md) | `op proxy` |
| ? | ? | [holons/grace-op/OP_RUN.md](holons/grace-op/OP_RUN.md) | `op run` |
| ? | ? | [holons/grace-op/README.md](holons/grace-op/README.md) | `op` overview |


## SDK Interfaces (sdk)

| Spec | Impl | Doc | Topic |
|:---:|:---:|---|---|
| ✅ | - | [sdk/README.md](sdk/README.md) | SDK contract — what every SDK must implement |
| ? | ? | [c-holons](sdk/c-holons/README.md) | auto-indexed |
| ? | ? | [cpp-holons](sdk/cpp-holons/README.md) | auto-indexed |
| ? | ? | [csharp-holons](sdk/csharp-holons/README.md) | auto-indexed |
| ? | ? | [dart-holons](sdk/dart-holons/README.md) | auto-indexed |
| ? | ? | [go-holons](sdk/go-holons/README.md) | auto-indexed |
| ? | ? | [java-holons](sdk/java-holons/README.md) | auto-indexed |
| ? | ? | [js-holons](sdk/js-holons/README.md) | auto-indexed |
| ? | ? | [js-web-holons](sdk/js-web-holons/README.md) | auto-indexed |
| ? | ? | [kotlin-holons](sdk/kotlin-holons/README.md) | auto-indexed |
| ? | ? | [python-holons](sdk/python-holons/README.md) | auto-indexed |
| ? | ? | [ruby-holons](sdk/ruby-holons/README.md) | auto-indexed |
| ? | ? | [rust-holons](sdk/rust-holons/README.md) | auto-indexed |

## Examples Hello-World

| ? | ? | [gabriel-greeting-app-swiftui](examples/hello-world/gabriel-greeting-app-swiftui/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-c](examples/hello-world/gabriel-greeting-c/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-cpp](examples/hello-world/gabriel-greeting-cpp/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-csharp](examples/hello-world/gabriel-greeting-csharp/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-dart](examples/hello-world/gabriel-greeting-dart/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-go](examples/hello-world/gabriel-greeting-go/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-java](examples/hello-world/gabriel-greeting-java/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-kotlin](examples/hello-world/gabriel-greeting-kotlin/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-node](examples/hello-world/gabriel-greeting-node/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-python](examples/hello-world/gabriel-greeting-python/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-ruby](examples/hello-world/gabriel-greeting-ruby/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-rust](examples/hello-world/gabriel-greeting-rust/README.md) | auto-indexed |
| ? | ? | [gabriel-greeting-swift](examples/hello-world/gabriel-greeting-swift/README.md) | auto-indexed |

## Kits & Packaging

| Spec | Impl | Doc | Topic |
|:---:|:---:|---|---|

| ? | ? | [organism_kits/README.md](organism_kits/README.md) | Organism kits — siblings resolution, app bundles |

## Implementation Details

| Spec | Impl | Doc | Topic |
|:---:|:---:|---|---|
| - | ? | [COAX_IP_interesting.md](_implementation/COAX_IP_interesting.md) | COAX — interesting IP patterns |
| - | ? | [OP_GET.md](_implementation/OP_GET.md) | `op get` implementation notes |
| - | ? | [ORIGIN_FLAG.md](_implementation/ORIGIN_FLAG.md) | `--origin` flag implementation |
| - | - | [_TODO.md](_implementation/_TODO.md) | Active task list |

## Archived Implementation Notes

| Spec | Impl | Doc | Topic |
|:---:|:---:|---|---|
| ❌| ? | [DESIGN_auto_build_on_connect.md](_implementation/done/DESIGN_auto_build_on_connect.md) | Archive: Auto-build on connect |
| ❌ | ? | [SDK_CLEANUP.md](_implementation/done/SDK_CLEANUP.md) | Archive: SDK Cleanup |
| ❌ | ? | [SDK_CLEANUP_plan.md](_implementation/done/SDK_CLEANUP_plan.md) | Archive: SDK Cleanup Plan |
| ❌ | ? | [codex_remove_reflection.md](_implementation/done/codex_remove_reflection.md) | Archive: Codex Reflection Removal |
| ❌ | ? | [op_coax_mcp_support.md](_implementation/done/op_coax_mcp_support.md) | Archive: MCP Support |
| ❌ | ? | [op_migrate_mcp_to_describe.md](_implementation/done/op_migrate_mcp_to_describe.md) | Archive: MCP to Describe Migration |