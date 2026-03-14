# TASK04 — C Composition Orchestrators (3 patterns)

## Objective

Add C as a composition orchestrator language, implementing all three backend-to-backend patterns with Go workers.

## Strategic Importance

C is the universal FFI layer — virtually every language can call C.
Proving C composition works has outsized impact on the ecosystem:

1. **Unlocks C-library holons** — FFmpeg (Megg), whisper.cpp (Wisupaa), SQLite, OpenSSL and any holon wrapping a C library can participate in composition pipelines.
2. **Validates the lowest common denominator** — if `connect(slug)` and gRPC work from C, they work from anywhere.
3. **Enables embedded/constrained orchestration** — IoT gateways and microcontrollers can orchestrate holon services without a managed runtime.

## Deliverables

1. **Direct call:** `recipes/composition/direct-call/charon-direct-c-go/`
   - `orchestrator → charon-worker-compute`
   - Expected: `Compute(42)` → `result = 1764`

2. **Pipeline:** `recipes/composition/pipeline/charon-pipeline-c-go/`
   - `orchestrator → charon-worker-compute → charon-worker-transform`
   - Expected: `Compute(5)` → `25`, then `Transform("25")` → `"52"`

3. **Fan-out:** `recipes/composition/fan-out/charon-fanout-c-go/`
   - `orchestrator → {compute, transform}` in parallel
   - Expected: both results + aggregation

## Implementation Notes

- gRPC client via `grpc-c` or raw Connect-protocol HTTP/2
- `connect(slug)` via manual resolution (no C SDK yet)
- Build system: CMake or Makefile

## Acceptance Criteria

- [ ] `op build` exits 0 for all 3 compositions
- [ ] `op run` exits 0 within 30s
- [ ] Workers discovered via `connect(slug)`
- [ ] Correct RPC results printed
- [ ] No orphan worker processes after orchestrator exits
