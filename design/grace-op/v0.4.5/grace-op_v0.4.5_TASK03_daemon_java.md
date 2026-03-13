# TASK03 — Java Daemon × 6 HostUIs

## Objective

Create the Java daemon holon and 6 assembly manifests pairing it with each HostUI.

## Deliverables

1. **Daemon holon:** `recipes/daemons/gudule-greeting-daemon-java/`
   - Build system: Gradle
   - SDK: gRPC server via `grpc-java`
   - Implements `ListLanguages` and `SayHello` RPCs
   - Serves the `connect(slug)` primitive.

2. **6 assembly manifests:**
   - `gudule-greeting-flutter-java`
   - `gudule-greeting-swiftui-java`
   - `gudule-greeting-kotlinui-java`
   - `gudule-greeting-java-web` (Note the reversed naming for web)
   - `gudule-greeting-dotnet-java`
   - `gudule-greeting-qt-java`

## Acceptance Criteria

- [ ] `op build` exits 0 for all 6 assemblies
- [ ] `op run` accurately provisions the daemon and UI components.
- [ ] The Java daemon fulfills its gRPC contractual obligations towards the 6 UI consumers.
