# TASK02 — C HostUI × 8 Daemons

## Objective

Create the C HostUI holon and 8 assembly manifests pairing it with each daemon language.

## Deliverables

1. **HostUI holon:** `recipes/hostui/gudule-greeting-hostui-c/`
   - UI toolkit: GTK 4 or SDL2
   - Build system: CMake or Makefile
   - gRPC client via `grpc-c` or raw Connect-protocol HTTP
   - `connect(slug)` via manual resolution (no C SDK yet)
   - Transport: `stdio`

2. **8 assembly manifests:**
   - `gudule-greeting-c-go`
   - `gudule-greeting-c-rust`
   - `gudule-greeting-c-swift`
   - `gudule-greeting-c-kotlin`
   - `gudule-greeting-c-dart`
   - `gudule-greeting-c-python`
   - `gudule-greeting-c-csharp`
   - `gudule-greeting-c-node`

## Acceptance Criteria

- [ ] `op build` exits 0 for all 8 assemblies
- [ ] `op run` starts daemon + C UI
- [ ] `ListLanguages` and `SayHello` RPCs work end-to-end
- [ ] No orphan processes after UI exit
