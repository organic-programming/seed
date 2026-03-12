# TASK03 — Java HostUI × 8 Daemons

## Objective

Create the Java HostUI holon and 8 assembly manifests pairing it with each daemon language.

## Deliverables

1. **HostUI holon:** `recipes/hostui/gudule-greeting-hostui-java/`
   - UI toolkit: Swing or JavaFX
   - Build system: Gradle
   - gRPC client via `grpc-java`
   - `connect(slug)` via the Java gRPC stub
   - Transport: `stdio`

2. **8 assembly manifests:**
   - `gudule-greeting-java-go`
   - `gudule-greeting-java-rust`
   - `gudule-greeting-java-swift`
   - `gudule-greeting-java-kotlin`
   - `gudule-greeting-java-dart`
   - `gudule-greeting-java-python`
   - `gudule-greeting-java-csharp`
   - `gudule-greeting-java-node`

## Acceptance Criteria

- [ ] `op build` exits 0 for all 8 assemblies
- [ ] `op run` starts daemon + Java UI
- [ ] `ListLanguages` and `SayHello` RPCs work end-to-end
- [ ] No orphan processes after UI exit
