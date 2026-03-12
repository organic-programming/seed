# TASK01 — C++ HostUI × 8 Daemons

## Objective

Create the C++ HostUI holon and 8 assembly manifests pairing it with each daemon language.

## Deliverables

1. **HostUI holon:** `recipes/hostui/gudule-greeting-hostui-cpp/`
   - UI toolkit: Qt 6 or Dear ImGui (choose based on portability)
   - Build system: CMake
   - gRPC client via `grpc++`
   - `connect(slug)` via the C++ SDK (or raw resolution if no SDK yet)
   - Transport: `stdio`

2. **8 assembly manifests:**
   - `gudule-greeting-cpp-go`
   - `gudule-greeting-cpp-rust`
   - `gudule-greeting-cpp-swift`
   - `gudule-greeting-cpp-kotlin`
   - `gudule-greeting-cpp-dart`
   - `gudule-greeting-cpp-python`
   - `gudule-greeting-cpp-csharp`
   - `gudule-greeting-cpp-node`

## Acceptance Criteria

- [ ] `op build` exits 0 for all 8 assemblies
- [ ] `op run` starts daemon + C++ UI
- [ ] `ListLanguages` and `SayHello` RPCs work end-to-end
- [ ] No orphan processes after UI exit
