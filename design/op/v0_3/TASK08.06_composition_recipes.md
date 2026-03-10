# TASK08.06 — Composition Recipes

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

Two simple worker holons in `recipes/composition/workers/`:
- `compute-worker` — accepts number, returns square
- `transform-worker` — accepts string, returns uppercase

Workers are Go-only (gRPC is language-agnostic — callers don't
know the worker's implementation language).

## Directory Structure

```
recipes/composition/
├── workers/
│   ├── compute-worker/
│   └── transform-worker/
├── direct-call/
│   ├── go/
│   ├── rust/
│   ├── swift/
│   ├── ...
│   └── cpp/
├── pipeline/
│   └── (same 11 languages)
└── fan-out/
    └── (same 11 languages)
```

## Acceptance Criteria

- [ ] 2 worker holons build and run
- [ ] Direct-call implemented in all 11 languages
- [ ] Pipeline implemented in all 11 languages
- [ ] Fan-out implemented in all 11 languages
- [ ] Each composition builds with `op build` and runs with `op run`

## Dependencies

None (workers are standalone; orchestrators use SDK `connect(slug)`).
