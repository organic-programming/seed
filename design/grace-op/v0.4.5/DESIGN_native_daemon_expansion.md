# DESIGN — Native Daemon Expansion (v0.4.5)

## Goal

Extend the canonical native daemon baseline with three foundational languages — **C++**, **C**, and **Java** — paring them with the 6 existing HostUIs (Flutter, SwiftUI, Kotlinui, Web, Dotnet, Qt). Also add **C** as a primary composition orchestrator.

This brings the total assemblies from 48 to **66** (18 new assemblies) and the composition recipes from 33 to **36** (3 new recipes).

## Motivation

The current matrix extensively covers modern memory-safe languages but misses the lowest common denominators (C/C++) and the massive enterprise JVM ecosystem (Java). Adding these as background Daemons enables them to serve robust backend workloads:

1. **Strategic C integration** — C is the universal FFI language. Adding a C daemon proves the SDK primitive `connect(slug)` works safely from memory-managed and low-level environments alike.
2. **High-performance computing & legacy wrapping** — C++ daemons allow seamless integration of advanced engine pipelines and graphics libraries into the Organic Programming architecture.
3. **Enterprise logic** — Java remains a dominant backend language; integrating it proves JVM parity and enterprise scale readyness.

## New Assembly Matrix

| Daemon | Language | Transport Default | Assemblies |
|--------|----------|-------------------|------------|
| C++ | C++ | TCP | 6 |
| C | C | TCP | 6 |
| Java | Java | TCP | 6 |

For Web assemblies, the Connect protocol is served over HTTP. Desktop/Mobile UIs will connect to the daemons via `stdio` or Connect over `tcp` as dictated by their specific environment policies.

## Assembly Naming

Follows standard conventions: `gudule-greeting-{hostui}-{daemon}`, except Web, which is reversed `gudule-greeting-{daemon}-web`.

```
# C++ Daemon
gudule-greeting-flutter-cpp
gudule-greeting-swiftui-cpp
gudule-greeting-kotlinui-cpp
gudule-greeting-cpp-web
gudule-greeting-dotnet-cpp
gudule-greeting-qt-cpp

# C Daemon
gudule-greeting-flutter-c
gudule-greeting-swiftui-c
gudule-greeting-kotlinui-c
gudule-greeting-c-web
gudule-greeting-dotnet-c
gudule-greeting-qt-c

# Java Daemon
gudule-greeting-flutter-java
gudule-greeting-swiftui-java
gudule-greeting-kotlinui-java
gudule-greeting-java-web
gudule-greeting-dotnet-java
gudule-greeting-qt-java
```

## SDK Requirements

Each daemon must implement the backend service side of the `connect(slug)` primitive, serving the `ListLanguages` and `SayHello` gRPC methods.

| Language | gRPC Library | Build System |
|----------|-------------|--------------|
| C++ | grpc++ (via vcpkg or system) | CMake |
| C | grpc-c or raw HTTP/2 | CMake / Makefile |
| Java | grpc-java | Gradle |

## Verification

Each assembly must pass the standard functional lifecycle checklist:

- `op build` exits 0
- `op run` starts (daemon process + UI process)
- `connect(slug)` succeeds
- `ListLanguages` RPC works
- `SayHello` RPC works

Update `VERIFICATION_composition.md` by appending the new assemblies into the existing UI sections (A.1 through A.6).

## C Composition Orchestrators

C is strategically critical. By making C an orchestrator, we validate that any low-level system or library can act as a fully competent initiator in the Organic Programming mesh.

Adds C to the composition orchestrators (Go, Rust, Swift, Kotlin, Dart, C#, Node.js, Python, Ruby, Java, C++) for backend-to-backend patterns:

| Pattern | Recipe | Expected Output |
|---------|--------|-----------------|
| Direct call | `charon-direct-c-go` | `Compute(42)` → `1764` |
| Pipeline | `charon-pipeline-c-go` | `25` then `"52"` |
| Fan-out | `charon-fanout-c-go` | Both results + aggregation |
