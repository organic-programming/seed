# TASK02 — C Daemon × 6 HostUIs

## Objective

Create the C daemon holon and 6 assembly manifests pairing it with each HostUI.

## Deliverables

1. **Daemon holon:** `recipes/daemons/gudule-greeting-daemon-c/`
   - Build system: CMake or Makefile
   - SDK: gRPC server via `grpc-c` or bare Connect-protocol over HTTP
   - Implements `ListLanguages` and `SayHello` RPCs
   - Serves the `connect(slug)` primitive.

2. **6 assembly manifests:**
   - `gudule-greeting-flutter-c`
   - `gudule-greeting-swiftui-c`
   - `gudule-greeting-kotlinui-c`
   - `gudule-greeting-c-web` (Note the reversed naming for web)
   - `gudule-greeting-dotnet-c`
   - `gudule-greeting-qt-c`

## Acceptance Criteria

- [ ] `op build` exits 0 for all 6 assemblies
- [ ] `op run` accurately provisions the daemon and UI components.
- [ ] The C daemon handles incoming remote procedure calls effectively from higher level UI frameworks securely.
