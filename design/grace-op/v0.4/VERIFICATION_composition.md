# Manual Verification Guide — All v0.4 Recipes

Exhaustive command-by-command guide to verify every assembly (48)
and every composition recipe (33). Each entry has exact `op build`
and `op run` commands.

All paths are relative to the seed root (`organic-programming/`).

---

## Prerequisites

```bash
# Install the op CLI
op install grace-op

# Verify op is available
op version
```

Required toolchains (install as needed):

| Language | Toolchain |
|----------|-----------|
| Go | `go` (1.22+) |
| Rust | `rustup` + `cargo` |
| Swift | Xcode CLI tools (macOS/Linux) |
| Kotlin | JDK 17+ + Gradle |
| Dart | `dart` SDK |
| Python | `python3` + `grpcio` |
| C# | `dotnet` SDK 8+ |
| Node.js | `node` 20+ + `npm` |
| Ruby | `ruby` 3+ + `grpc` gem |
| Java | JDK 17+ + Gradle |
| C++ | CMake + gRPC (via vcpkg or brew) |
| Flutter | `flutter` SDK |
| Qt | Qt 6 + CMake |

---

## Part A — Assemblies (48)

Each assembly is a thin `holon.yaml` pairing a daemon with a HostUI.
Verification: build succeeds and the app launches (daemon starts, UI connects via `connect(slug)`).

> [!TIP]
> **Transport rules:**
> - Desktop HostUIs (Flutter, SwiftUI, Compose, Dotnet, Qt): `stdio`
> - Web assemblies: `tcp` (Connect protocol over HTTP)

---

❌ failure
⚠️ quality issue
✅ success

**@bpds 12/02/2026 on mac os 15.7.2 (24G325) Apple M4**
> @bpds: i do run each case, then in case of failure give a screenshot to codex, please guide it to fix the issue. When an issue is fixed i do verify if there are no regression. 

### A.1 Flutter × 8 Daemons

```bash
# A.1.1 — Flutter + Go
op build recipes/assemblies/gudule-greeting-flutter-go ✅ ✅ ✅ ✅ ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-go ✅ ✅ ✅ ✅ ✅ ✅ **@bpds 12/02/2026**

# A.1.2 — Flutter + Rust
op build recipes/assemblies/gudule-greeting-flutter-rust ✅ ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-rust ❌ ✅ ✅ **@bpds 12/02/2026**

# A.1.3 — Flutter + Swift
op build recipes/assemblies/gudule-greeting-flutter-swift ✅ ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-swift ❌ ❌ ✅ **@bpds 12/02/2026**

# A.1.4 — Flutter + Kotlin
op build recipes/assemblies/gudule-greeting-flutter-kotlin ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-kotlin ✅ ✅ **@bpds 12/02/2026**

# A.1.5 — Flutter + Dart
op build recipes/assemblies/gudule-greeting-flutter-dart ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-dart ❌ ✅ **@bpds 12/02/2026**

# A.1.6 — Flutter + Python
op build recipes/assemblies/gudule-greeting-flutter-python ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-python ❌ ✅ **@bpds 12/02/2026**

# A.1.7 — Flutter + C#
op build recipes/assemblies/gudule-greeting-flutter-csharp ✅ ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-csharp ✅ ✅ ✅ **@bpds 12/02/2026**

# A.1.8 — Flutter + Node.js
op build recipes/assemblies/gudule-greeting-flutter-node ✅ ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-flutter-node ❌ ❌ ✅ **@bpds 12/02/2026**
```

**Verify:** Flutter window opens → ListLanguages populates dropdown → SayHello returns greeting.

---

### A.2 SwiftUI × 8 Daemons (macOS only)

```bash
# A.2.1 — SwiftUI + Go
op build recipes/assemblies/gudule-greeting-swiftui-go ✅ ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-go ❌ ✅ ✅ **@bpds 12/02/2026** 

# A.2.2 — SwiftUI + Rust
op build recipes/assemblies/gudule-greeting-swiftui-rust ✅ ✅ ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-rust ❌ ✅ ✅ **@bpds 12/02/2026** 

# A.2.3 — SwiftUI + Swift
op build recipes/assemblies/gudule-greeting-swiftui-swift ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-swift ✅ **@bpds 12/02/2026**

# A.2.4 — SwiftUI + Kotlin
op build recipes/assemblies/gudule-greeting-swiftui-kotlin ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-kotlin ✅ **@bpds 12/02/2026**

# A.2.5 — SwiftUI + Dart
op build recipes/assemblies/gudule-greeting-swiftui-dart ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-dart ✅ **@bpds 12/02/2026**

# A.2.6 — SwiftUI + Python
op build recipes/assemblies/gudule-greeting-swiftui-python  ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-python  ✅ **@bpds 12/02/2026**

# A.2.7 — SwiftUI + C#
op build recipes/assemblies/gudule-greeting-swiftui-csharp ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-csharp ❌ ✅ **@bpds 12/02/2026** 

# A.2.8 — SwiftUI + Node.js
op build recipes/assemblies/gudule-greeting-swiftui-node ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-swiftui-node ✅ **@bpds 12/02/2026**
```

**Verify:** `.app` bundle launches → `_CodeSignature/CodeResources` present → ListLanguages → SayHello works.

---

### A.3 Kotlinui × 8 Daemons

```bash
# A.3.1 — Kotlinui + Go
op build recipes/assemblies/gudule-greeting-kotlinui-go ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-go ✅ **@bpds 12/02/2026**

# A.3.2 — Kotlinui + Rust
op build recipes/assemblies/gudule-greeting-kotlinui-rust ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-rust ✅ **@bpds 12/02/2026**

# A.3.3 — Kotlinui + Swift
op build recipes/assemblies/gudule-greeting-kotlinui-swift ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-swift ✅ **@bpds 12/02/2026**

# A.3.4 — Kotlinui + Kotlin
op build recipes/assemblies/gudule-greeting-kotlinui-kotlin ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-kotlin ✅ **@bpds 12/02/2026**

# A.3.5 — Kotlinui + Dart
op build recipes/assemblies/gudule-greeting-kotlinui-dart ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-dart ✅ **@bpds 12/02/2026**

# A.3.6 — Kotlinui + Python
op build recipes/assemblies/gudule-greeting-kotlinui-python ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-python ✅ **@bpds 12/02/2026**

# A.3.7 — Kotlinui + C#
op build recipes/assemblies/gudule-greeting-kotlinui-csharp ✅ ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-csharp ✅ ✅ **@bpds 12/02/2026**

# A.3.8 — Kotlinui + Node.js
op build recipes/assemblies/gudule-greeting-kotlinui-node ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-kotlinui-node ✅ **@bpds 12/02/2026**
```

**Verify:** Compose desktop window → ListLanguages → SayHello.

---

### A.4 Web × 8 Daemons (reversed naming: daemon-web)

Web assemblies use Connect protocol over HTTP. The daemon embeds and
serves the web client.

```bash
# A.4.1 — Go + Web
op build recipes/assemblies/gudule-greeting-go-web 
op run   recipes/assemblies/gudule-greeting-go-web
# → open http://localhost:<port> in browser

# A.4.2 — Rust + Web
op build recipes/assemblies/gudule-greeting-rust-web
op run   recipes/assemblies/gudule-greeting-rust-web

# A.4.3 — Swift + Web
op build recipes/assemblies/gudule-greeting-swift-web
op run   recipes/assemblies/gudule-greeting-swift-web

# A.4.4 — Kotlin + Web
op build recipes/assemblies/gudule-greeting-kotlin-web
op run   recipes/assemblies/gudule-greeting-kotlin-web

# A.4.5 — Dart + Web
op build recipes/assemblies/gudule-greeting-dart-web
op run   recipes/assemblies/gudule-greeting-dart-web

# A.4.6 — Python + Web
op build recipes/assemblies/gudule-greeting-python-web
op run   recipes/assemblies/gudule-greeting-python-web

# A.4.7 — C# + Web
op build recipes/assemblies/gudule-greeting-csharp-web
op run   recipes/assemblies/gudule-greeting-csharp-web

# A.4.8 — Node.js + Web
op build recipes/assemblies/gudule-greeting-node-web
op run   recipes/assemblies/gudule-greeting-node-web
```

**Verify:** Browser page loads at `localhost:<port>` → ListLanguages → SayHello via Connect RPC.

---

### A.5 Dotnet × 8 Daemons

```bash
# A.5.1 — Dotnet + Go
op build recipes/assemblies/gudule-greeting-dotnet-go ❌ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-dotnet-go

# A.5.2 — Dotnet + Rust
op build recipes/assemblies/gudule-greeting-dotnet-rust
op run   recipes/assemblies/gudule-greeting-dotnet-rust

# A.5.3 — Dotnet + Swift
op build recipes/assemblies/gudule-greeting-dotnet-swift
op run   recipes/assemblies/gudule-greeting-dotnet-swift

# A.5.4 — Dotnet + Kotlin
op build recipes/assemblies/gudule-greeting-dotnet-kotlin
op run   recipes/assemblies/gudule-greeting-dotnet-kotlin

# A.5.5 — Dotnet + Dart
op build recipes/assemblies/gudule-greeting-dotnet-dart
op run   recipes/assemblies/gudule-greeting-dotnet-dart

# A.5.6 — Dotnet + Python
op build recipes/assemblies/gudule-greeting-dotnet-python
op run   recipes/assemblies/gudule-greeting-dotnet-python

# A.5.7 — Dotnet + C#
op build recipes/assemblies/gudule-greeting-dotnet-csharp
op run   recipes/assemblies/gudule-greeting-dotnet-csharp

# A.5.8 — Dotnet + Node.js
op build recipes/assemblies/gudule-greeting-dotnet-node
op run   recipes/assemblies/gudule-greeting-dotnet-node
```

**Verify:** .NET MAUI window → ListLanguages → SayHello.

---

### A.6 Qt × 8 Daemons

```bash
# A.6.1 — Qt + Go
op build recipes/assemblies/gudule-greeting-qt-go ✅ **@bpds 12/02/2026**
op run   recipes/assemblies/gudule-greeting-qt-go ❌ **@bpds 12/02/2026**

# A.6.2 — Qt + Rust
op build recipes/assemblies/gudule-greeting-qt-rust
op run   recipes/assemblies/gudule-greeting-qt-rust

# A.6.3 — Qt + Swift
op build recipes/assemblies/gudule-greeting-qt-swift
op run   recipes/assemblies/gudule-greeting-qt-swift

# A.6.4 — Qt + Kotlin
op build recipes/assemblies/gudule-greeting-qt-kotlin
op run   recipes/assemblies/gudule-greeting-qt-kotlin

# A.6.5 — Qt + Dart
op build recipes/assemblies/gudule-greeting-qt-dart
op run   recipes/assemblies/gudule-greeting-qt-dart

# A.6.6 — Qt + Python
op build recipes/assemblies/gudule-greeting-qt-python
op run   recipes/assemblies/gudule-greeting-qt-python

# A.6.7 — Qt + C#
op build recipes/assemblies/gudule-greeting-qt-csharp
op run   recipes/assemblies/gudule-greeting-qt-csharp

# A.6.8 — Qt + Node.js
op build recipes/assemblies/gudule-greeting-qt-node
op run   recipes/assemblies/gudule-greeting-qt-node
```

**Verify:** Qt window → ListLanguages → SayHello.

---

### Assembly Verification Checklist

For each assembly (A.1.1 through A.6.8):

- [ ] `op build` exits 0
- [ ] `op run` starts (daemon process + UI process)
- [ ] `connect(slug)` resolves (no hardcoded address)
- [ ] `ListLanguages` RPC returns language list
- [ ] `SayHello` RPC returns greeting
- [ ] No orphan daemon process after UI exits
- [ ] SwiftUI: `_CodeSignature/CodeResources` present (v0.4.4 auto-sign)
- [ ] Web: browser page loads, Connect RPC works

---

## Part B — Composition Recipes (33)

Backend-to-backend patterns. No UI — orchestrator calls Go workers
via `connect(slug)`.

### B.0 Build the Shared Workers (once)

```bash
op build recipes/composition/workers/charon-worker-compute  ✅ **@bpds 12/02/2026**
op build recipes/composition/workers/charon-worker-transform  ✅ **@bpds 12/02/2026**
```

Verify:
```bash
ls recipes/composition/workers/charon-worker-compute/build/ ❌ **@bpds 12/02/2026**
ls recipes/composition/workers/charon-worker-transform/build/ ❌ **@bpds 12/02/2026**
```

---

### B.1 Direct Call (11 orchestrators)

Topology: `orchestrator → charon-worker-compute`
Expected: sends `Compute(42)`, prints `result = 1764`.

```bash
# B.1.1 — Go → Go
op build recipes/composition/direct-call/charon-direct-go-go ✅ **@bpds 12/02/2026**
op run   recipes/composition/direct-call/charon-direct-go-go ✅ **@bpds 12/02/2026**

# B.1.2 — Rust → Go
op build recipes/composition/direct-call/charon-direct-rust-go
op run   recipes/composition/direct-call/charon-direct-rust-go

# B.1.3 — Swift → Go
op build recipes/composition/direct-call/charon-direct-swift-go
op run   recipes/composition/direct-call/charon-direct-swift-go

# B.1.4 — Kotlin → Go
op build recipes/composition/direct-call/charon-direct-kotlin-go
op run   recipes/composition/direct-call/charon-direct-kotlin-go

# B.1.5 — Dart → Go
op build recipes/composition/direct-call/charon-direct-dart-go
op run   recipes/composition/direct-call/charon-direct-dart-go

# B.1.6 — C# → Go
op build recipes/composition/direct-call/charon-direct-csharp-go
op run   recipes/composition/direct-call/charon-direct-csharp-go

# B.1.7 — Node.js → Go
op build recipes/composition/direct-call/charon-direct-node-go
op run   recipes/composition/direct-call/charon-direct-node-go

# B.1.8 — Python → Go
op build recipes/composition/direct-call/charon-direct-python-go
op run   recipes/composition/direct-call/charon-direct-python-go

# B.1.9 — Ruby → Go
op build recipes/composition/direct-call/charon-direct-ruby-go
op run   recipes/composition/direct-call/charon-direct-ruby-go

# B.1.10 — Java → Go
op build recipes/composition/direct-call/charon-direct-java-go
op run   recipes/composition/direct-call/charon-direct-java-go

# B.1.11 — C++ → Go
op build recipes/composition/direct-call/charon-direct-cpp-go
op run   recipes/composition/direct-call/charon-direct-cpp-go
```

---

### B.2 Pipeline (11 orchestrators)

Topology: `orchestrator → charon-worker-compute → charon-worker-transform`
Expected: `Compute(5)` → `25`, then `Transform("25")` → `"52"`. Both results printed.

```bash
# B.2.1 — Go → Go
op build recipes/composition/pipeline/charon-pipeline-go-go ❌ **@bpds 12/02/2026** fails binding issue, architecture must be controlled.
op run   recipes/composition/pipeline/charon-pipeline-go-go

# B.2.2 — Rust → Go
op build recipes/composition/pipeline/charon-pipeline-rust-go
op run   recipes/composition/pipeline/charon-pipeline-rust-go

# B.2.3 — Swift → Go
op build recipes/composition/pipeline/charon-pipeline-swift-go
op run   recipes/composition/pipeline/charon-pipeline-swift-go

# B.2.4 — Kotlin → Go
op build recipes/composition/pipeline/charon-pipeline-kotlin-go
op run   recipes/composition/pipeline/charon-pipeline-kotlin-go

# B.2.5 — Dart → Go
op build recipes/composition/pipeline/charon-pipeline-dart-go
op run   recipes/composition/pipeline/charon-pipeline-dart-go

# B.2.6 — C# → Go
op build recipes/composition/pipeline/charon-pipeline-csharp-go
op run   recipes/composition/pipeline/charon-pipeline-csharp-go

# B.2.7 — Node.js → Go
op build recipes/composition/pipeline/charon-pipeline-node-go
op run   recipes/composition/pipeline/charon-pipeline-node-go

# B.2.8 — Python → Go
op build recipes/composition/pipeline/charon-pipeline-python-go
op run   recipes/composition/pipeline/charon-pipeline-python-go

# B.2.9 — Ruby → Go
op build recipes/composition/pipeline/charon-pipeline-ruby-go
op run   recipes/composition/pipeline/charon-pipeline-ruby-go

# B.2.10 — Java → Go
op build recipes/composition/pipeline/charon-pipeline-java-go
op run   recipes/composition/pipeline/charon-pipeline-java-go

# B.2.11 — C++ → Go
op build recipes/composition/pipeline/charon-pipeline-cpp-go
op run   recipes/composition/pipeline/charon-pipeline-cpp-go
```

---

### B.3 Fan-Out (11 orchestrators)

Topology: `orchestrator → {charon-worker-compute, charon-worker-transform}` in parallel
Expected: both Compute and Transform execute concurrently. Aggregated results printed.

```bash
# B.3.1 — Go → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-go-go ✅ **@bpds 12/02/2026**
op run   recipes/composition/fan-out/charon-fanout-go-go ✅ **@bpds 12/02/2026**

# B.3.2 — Rust → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-rust-go
op run   recipes/composition/fan-out/charon-fanout-rust-go

# B.3.3 — Swift → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-swift-go
op run   recipes/composition/fan-out/charon-fanout-swift-go

# B.3.4 — Kotlin → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-kotlin-go
op run   recipes/composition/fan-out/charon-fanout-kotlin-go

# B.3.5 — Dart → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-dart-go
op run   recipes/composition/fan-out/charon-fanout-dart-go

# B.3.6 — C# → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-csharp-go
op run   recipes/composition/fan-out/charon-fanout-csharp-go

# B.3.7 — Node.js → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-node-go
op run   recipes/composition/fan-out/charon-fanout-node-go

# B.3.8 — Python → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-python-go
op run   recipes/composition/fan-out/charon-fanout-python-go

# B.3.9 — Ruby → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-ruby-go
op run   recipes/composition/fan-out/charon-fanout-ruby-go

# B.3.10 — Java → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-java-go
op run   recipes/composition/fan-out/charon-fanout-java-go

# B.3.11 — C++ → Go (parallel)
op build recipes/composition/fan-out/charon-fanout-cpp-go
op run   recipes/composition/fan-out/charon-fanout-cpp-go
```

---

### Composition Verification Checklist

For each composition (B.1.1 through B.3.11):

- [ ] `op build` exits 0
- [ ] `op run` exits 0 within 30s
- [ ] Workers discovered via `connect(slug)` (no hardcoded addresses)
- [ ] Correct RPC result printed:
  - Direct: `1764`
  - Pipeline: `25` then `"52"`
  - Fan-out: both results + aggregation
- [ ] No orphan worker processes after orchestrator exits

---

## Part C — Milestone Checkpoints

### C.1 TASK06 — 3×3 Cross-Language Validation (v0.4.2)

The critical early-validation matrix. These 9 assemblies must all
pass **before** generating the remaining 39.

```bash
# Row 1: Go daemon × 3 HostUIs
op build recipes/assemblies/gudule-greeting-flutter-go  ✅ **@bpds 12/02/2026** # Already validated (v0.4.1/TASK04)
op run   recipes/assemblies/gudule-greeting-flutter-go  ✅ **@bpds 12/02/2026**

op build recipes/assemblies/gudule-greeting-go-web
op run   recipes/assemblies/gudule-greeting-go-web

op build recipes/assemblies/gudule-greeting-qt-go
op run   recipes/assemblies/gudule-greeting-qt-go

# Row 2: Rust daemon × 3 HostUIs
op build recipes/assemblies/gudule-greeting-flutter-rust
op run   recipes/assemblies/gudule-greeting-flutter-rust

op build recipes/assemblies/gudule-greeting-rust-web
op run   recipes/assemblies/gudule-greeting-rust-web

op build recipes/assemblies/gudule-greeting-qt-rust
op run   recipes/assemblies/gudule-greeting-qt-rust

# Row 3: Swift daemon × 3 HostUIs
op build recipes/assemblies/gudule-greeting-flutter-swift
op run   recipes/assemblies/gudule-greeting-flutter-swift

op build recipes/assemblies/gudule-greeting-swift-web
op run   recipes/assemblies/gudule-greeting-swift-web

op build recipes/assemblies/gudule-greeting-qt-swift
op run   recipes/assemblies/gudule-greeting-qt-swift
```

### C.2 Automated Test Matrix

```bash
# Full matrix (all 48 assemblies + 33 compositions)
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go

# Assemblies only
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "gudule-*"

# Compositions only
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "charon-*"

# By pattern
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "charon-direct-*"
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "charon-pipeline-*"
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "charon-fanout-*"

# By daemon language (across all HostUIs)
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "*-go"
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "*-rust*"
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "*-swift*"

# By HostUI framework
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "*-flutter-*"
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "*-swiftui-*"
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --filter "*-web"

# JSON output (CI)
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --format json

# Dry run
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --dry-run

# Custom timeout
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go --timeout 60s
```

---

## Part D — Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `connect(slug): not found` | Worker/daemon not built or not in `$OPPATH` | Rebuild the target holon |
| `transport: dial tcp: connection refused` | Target process crashed or timed out | Check logs, restart |
| `missing toolchain` (⏭️ in matrix) | Language SDK not installed | Install SDK |
| Build succeeds but run hangs | Waiting for readiness probe | Increase `--timeout`, check health |
| `exec format error` | Binary built for wrong architecture | `op build --clean <path>`, rebuild |
| SwiftUI: unsigned bundle error | v0.4.4 auto-sign not applied | Run `op build` (auto ad-hoc signs) |
| Web: page won't load | Wrong port or daemon not serving | Check stdout for port, verify `http://localhost:<port>` |
| `_CodeSignature` missing (SwiftUI) | Build ran with `--no-sign` | Remove flag, or run `codesign --force --deep --sign - <app>` manually |
