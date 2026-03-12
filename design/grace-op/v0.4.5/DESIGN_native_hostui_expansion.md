# DESIGN — Native Expansion (v0.4.5)

## Goal

Complete the assembly matrix with three native HostUI frameworks — **C++**, **C**, and **Java** — each paired with the 8 existing daemons (Go, Rust, Swift, Kotlin, Dart, Python, C#, Node.js). Also add **C** as a composition orchestrator language.

This brings the total from 48 to **72 assemblies** and from 33 to **36 composition recipes**.

## Motivation

The current matrix covers managed/framework UIs (Flutter, SwiftUI, Compose, Web, Dotnet, Qt) but lacks low-level native options and the JVM desktop ecosystem. Adding C++, C, and Java HostUIs:

1. **Proves SDK generality** — validates that `connect(slug)` works from languages without first-class gRPC support.
2. **Covers the remaining major desktop ecosystems** — GTK/SDL (C), imgui/Qt (C++), Swing/JavaFX (Java).
3. **Unlocks embedded/constrained use-cases** — C and C++ HostUIs are essential for IoT and kiosk deployments.

## New HostUI Matrix

| HostUI | Language | UI Toolkit | Transport | Assemblies |
|--------|----------|------------|-----------|------------|
| C++ | C++ | Qt or Dear ImGui | stdio | 8 |
| C | C | GTK 4 or SDL2 | stdio | 8 |
| Java | Java | Swing or JavaFX | stdio | 8 |

All desktop HostUIs use `stdio` transport (consistent with Flutter, SwiftUI, Compose, Dotnet, Qt).

## Assembly Naming

Follows the existing convention: `gudule-greeting-{hostui}-{daemon}`.

```
# C++ HostUI
gudule-greeting-cpp-go
gudule-greeting-cpp-rust
gudule-greeting-cpp-swift
gudule-greeting-cpp-kotlin
gudule-greeting-cpp-dart
gudule-greeting-cpp-python
gudule-greeting-cpp-csharp
gudule-greeting-cpp-node

# C HostUI
gudule-greeting-c-go
gudule-greeting-c-rust
gudule-greeting-c-swift
gudule-greeting-c-kotlin
gudule-greeting-c-dart
gudule-greeting-c-python
gudule-greeting-c-csharp
gudule-greeting-c-node

# Java HostUI
gudule-greeting-java-go
gudule-greeting-java-rust
gudule-greeting-java-swift
gudule-greeting-java-kotlin
gudule-greeting-java-dart
gudule-greeting-java-python
gudule-greeting-java-csharp
gudule-greeting-java-node
```

## SDK Requirements

Each HostUI must use `connect(slug)` for daemon discovery — no hardcoded addresses.

| Language | gRPC Library | Build System |
|----------|-------------|--------------|
| C++ | grpc++ (via vcpkg or system) | CMake |
| C | grpc-c or raw HTTP/2 | CMake / Makefile |
| Java | grpc-java | Gradle |

## Verification

Each assembly must pass the standard checklist:

- `op build` exits 0
- `op run` starts (daemon + UI processes)
- `connect(slug)` resolves
- `ListLanguages` RPC returns language list
- `SayHello` RPC returns greeting
- No orphan daemon process after UI exits

Update `VERIFICATION_composition.md` with sections A.7 (C++), A.8 (C), A.9 (Java) once implemented.

## C Composition Orchestrators

C is strategically critical: it is the universal FFI layer, so a proven C orchestrator unlocks composition for every holon wrapping a C library (FFmpeg/Megg, whisper.cpp/Wisupaa, SQLite, OpenSSL, etc.).

Adds C to the existing 11 orchestrator languages (Go, Rust, Swift, Kotlin, Dart, C#, Node.js, Python, Ruby, Java, C++) for all three backend-to-backend patterns:

| Pattern | Recipe | Expected Output |
|---------|--------|-----------------|
| Direct call | `charon-direct-c-go` | `Compute(42)` → `1764` |
| Pipeline | `charon-pipeline-c-go` | `25` then `"52"` |
| Fan-out | `charon-fanout-c-go` | Both results + aggregation |

gRPC client via `grpc-c` or raw Connect-protocol HTTP/2. Shares build tooling with the C HostUI (TASK02).
