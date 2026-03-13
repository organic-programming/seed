# TASK01 — C++ Daemon × 6 HostUIs

## Objective

Create the C++ daemon holon and 6 assembly manifests pairing it with each HostUI.

## Deliverables

1. **Daemon holon:** `recipes/daemons/gudule-greeting-daemon-cpp/`
   - Build system: CMake
   - SDK: gRPC server via `grpc++`
   - Implements `ListLanguages` and `SayHello` RPCs
   - Serves the `connect(slug)` primitive.

2. **6 assembly manifests:**
   - `gudule-greeting-flutter-cpp`
   - `gudule-greeting-swiftui-cpp`
   - `gudule-greeting-kotlinui-cpp`
   - `gudule-greeting-cpp-web` (Note the reversed naming for web)
   - `gudule-greeting-dotnet-cpp`
   - `gudule-greeting-qt-cpp`

## Acceptance Criteria

- [ ] `op build` exits 0 for all 6 assemblies
- [ ] `op run` accurately provisions the daemon and UI components.
- [ ] The C++ daemon responds correctly to dynamic composition RPCs (`ListLanguages` & `SayHello`) from all 6 different UI stacks.
