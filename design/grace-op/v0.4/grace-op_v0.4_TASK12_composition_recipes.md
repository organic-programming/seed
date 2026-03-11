# TASK12 — Composition Recipes

## Summary

Create backend-to-backend recipes demonstrating holon composition.
3 patterns × 11 orchestrator languages. Uses shared worker holons.

## Patterns (v1)

| Pattern | Topology | Teaches |
|---|---|---|
| **Direct Call** | A → B | `connect(slug)`, basic RPC |
| **Pipeline** | A → B → C | Multi-service chain, error propagation |
| **Fan-Out** | A → {B, C} parallel | Async dispatch, partial success |

## Orchestrator Languages (11)

Go, Rust, Swift, Kotlin, C#, Dart, Node.js, Python, Ruby, Java, C++.

All have `connect(slug)` implementations in their SDKs.

## Shared Workers

Two simple worker holons:
- `charon-worker-compute` — accepts number, returns square
- `charon-worker-transform` — accepts string, returns reversed

Workers are Go-only (gRPC is language-agnostic — callers don't
know the worker's implementation language).

## Directory Structure

```
recipes/composition/
├── workers/
│   ├── charon-worker-compute/
│   └── charon-worker-transform/
├── direct-call/
│   ├── charon-direct-go-go/
│   ├── charon-direct-rust-go/
│   ├── charon-direct-swift-go/
│   ├── charon-direct-kotlin-go/
│   ├── charon-direct-dart-go/
│   ├── charon-direct-csharp-go/
│   ├── charon-direct-node-go/
│   ├── charon-direct-python-go/
│   ├── charon-direct-ruby-go/
│   ├── charon-direct-java-go/
│   └── charon-direct-cpp-go/
├── pipeline/
│   └── (same 11: charon-pipeline-<lang>-go)
└── fan-out/
    └── (same 11: charon-fanout-<lang>-go)
```

## Acceptance Criteria

- [ ] 2 worker holons build and run
- [ ] Direct-call implemented in all 11 languages (canonical names per [DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md))
- [ ] Pipeline implemented in all 11 languages
- [ ] Fan-out implemented in all 11 languages
- [ ] Each composition builds with `op build` and runs with `op run`

## Dependencies

TASK11.

## Reference

Proto contracts, code snippets, and full name list:
[DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md)
