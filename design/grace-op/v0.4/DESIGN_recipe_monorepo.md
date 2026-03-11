# Recipe Monorepo Architecture

Canonical naming reference, proto contracts, composition code patterns,
and the full assembly matrix.

> [!CAUTION]
> **Every name below is canonical.** Codex must use these exact names
> for directories, `family_name` in `holon.yaml`, and binary names.
> Do not invent names — this document is the canonical source of truth for **naming** (directory names, `family_name` values, proto contracts). For machine-readable inventory (source paths, SDKs, runners, task IDs) see [recipes.yaml](./recipes.yaml).

Two personas:
- **`given_name: gudule`** — Greeting scenario (daemons, HostUIs, assemblies, testmatrix)
- **`given_name: charon`** — Composition scenario (workers, orchestrators)

---

## 1. Shared Proto

```
recipes/protos/greeting/v1/
└── greeting.proto              ← canonical GreetingService definition
```

Each daemon and HostUI has a local `.proto` wrapper that re-exports
via `import public` and adds holon-specific meta-annotations
(see [PROTO.md](../../PROTO.md)).

---

## 2. DRY Daemons (8)

| `family_name` | binary | directory |
|---|---|---|
| `Greeting-Daemon-Go` | `gudule-daemon-greeting-go` | `recipes/daemons/gudule-daemon-greeting-go/` |
| `Greeting-Daemon-Rust` | `gudule-daemon-greeting-rust` | `recipes/daemons/gudule-daemon-greeting-rust/` |
| `Greeting-Daemon-Swift` | `gudule-daemon-greeting-swift` | `recipes/daemons/gudule-daemon-greeting-swift/` |
| `Greeting-Daemon-Kotlin` | `gudule-daemon-greeting-kotlin` | `recipes/daemons/gudule-daemon-greeting-kotlin/` |
| `Greeting-Daemon-Dart` | `gudule-daemon-greeting-dart` | `recipes/daemons/gudule-daemon-greeting-dart/` |
| `Greeting-Daemon-Python` | `gudule-daemon-greeting-python` | `recipes/daemons/gudule-daemon-greeting-python/` |
| `Greeting-Daemon-Csharp` | `gudule-daemon-greeting-csharp` | `recipes/daemons/gudule-daemon-greeting-csharp/` |
| `Greeting-Daemon-Node` | `gudule-daemon-greeting-node` | `recipes/daemons/gudule-daemon-greeting-node/` |

---

## 3. DRY HostUIs (6)

HostUI names use the **framework name** (not the language name) to
avoid collision with daemon language names.

| `family_name` | binary | directory | Technology |
|---|---|---|---|
| `Greeting-Hostui-Flutter` | `gudule-greeting-hostui-flutter` | `recipes/hostui/gudule-greeting-hostui-flutter/` | Flutter (Dart) |
| `Greeting-Hostui-Swiftui` | `gudule-greeting-hostui-swiftui` | `recipes/hostui/gudule-greeting-hostui-swiftui/` | SwiftUI (Swift) |
| `Greeting-Hostui-Compose` | `gudule-greeting-hostui-compose` | `recipes/hostui/gudule-greeting-hostui-compose/` | Jetpack Compose (Kotlin) |
| `Greeting-Hostui-Web` | `gudule-greeting-hostui-web` | `recipes/hostui/gudule-greeting-hostui-web/` | TypeScript/HTML |
| `Greeting-Hostui-Dotnet` | `gudule-greeting-hostui-dotnet` | `recipes/hostui/gudule-greeting-hostui-dotnet/` | .NET MAUI (C#) |
| `Greeting-Hostui-Qt` | `gudule-greeting-hostui-qt` | `recipes/hostui/gudule-greeting-hostui-qt/` | Qt (C++) |

---

## 4. Assemblies (48)

Assembly naming: `<hostui>-<daemon>` — the HostUI connects to the
daemon via `connect(slug)`.

**Exception: Web assemblies** use `<daemon>-web` — the daemon serves
the web client (Connect protocol, web dist embedded in daemon binary).

### Flutter HostUI × 8 Daemons

| `family_name` | directory |
|---|---|
| `Greeting-Flutter-Go` | `recipes/assemblies/gudule-greeting-flutter-go/` |
| `Greeting-Flutter-Rust` | `recipes/assemblies/gudule-greeting-flutter-rust/` |
| `Greeting-Flutter-Swift` | `recipes/assemblies/gudule-greeting-flutter-swift/` |
| `Greeting-Flutter-Kotlin` | `recipes/assemblies/gudule-greeting-flutter-kotlin/` |
| `Greeting-Flutter-Dart` ★ | `recipes/assemblies/gudule-greeting-flutter-dart/` |
| `Greeting-Flutter-Python` | `recipes/assemblies/gudule-greeting-flutter-python/` |
| `Greeting-Flutter-Csharp` | `recipes/assemblies/gudule-greeting-flutter-csharp/` |
| `Greeting-Flutter-Node` | `recipes/assemblies/gudule-greeting-flutter-node/` |

### SwiftUI HostUI × 8 Daemons

| `family_name` | directory |
|---|---|
| `Greeting-Swiftui-Go` | `recipes/assemblies/gudule-greeting-swiftui-go/` |
| `Greeting-Swiftui-Rust` | `recipes/assemblies/gudule-greeting-swiftui-rust/` |
| `Greeting-Swiftui-Swift` ★ | `recipes/assemblies/gudule-greeting-swiftui-swift/` |
| `Greeting-Swiftui-Kotlin` | `recipes/assemblies/gudule-greeting-swiftui-kotlin/` |
| `Greeting-Swiftui-Dart` | `recipes/assemblies/gudule-greeting-swiftui-dart/` |
| `Greeting-Swiftui-Python` | `recipes/assemblies/gudule-greeting-swiftui-python/` |
| `Greeting-Swiftui-Csharp` | `recipes/assemblies/gudule-greeting-swiftui-csharp/` |
| `Greeting-Swiftui-Node` | `recipes/assemblies/gudule-greeting-swiftui-node/` |

### Compose HostUI × 8 Daemons

| `family_name` | directory |
|---|---|
| `Greeting-Compose-Go` | `recipes/assemblies/gudule-greeting-compose-go/` |
| `Greeting-Compose-Rust` | `recipes/assemblies/gudule-greeting-compose-rust/` |
| `Greeting-Compose-Swift` | `recipes/assemblies/gudule-greeting-compose-swift/` |
| `Greeting-Compose-Kotlin` ★ | `recipes/assemblies/gudule-greeting-compose-kotlin/` |
| `Greeting-Compose-Dart` | `recipes/assemblies/gudule-greeting-compose-dart/` |
| `Greeting-Compose-Python` | `recipes/assemblies/gudule-greeting-compose-python/` |
| `Greeting-Compose-Csharp` | `recipes/assemblies/gudule-greeting-compose-csharp/` |
| `Greeting-Compose-Node` | `recipes/assemblies/gudule-greeting-compose-node/` |

### Web Assemblies × 8 Daemons (reversed: daemon serves web)

| `family_name` | directory |
|---|---|
| `Greeting-Go-Web` | `recipes/assemblies/gudule-greeting-go-web/` |
| `Greeting-Rust-Web` | `recipes/assemblies/gudule-greeting-rust-web/` |
| `Greeting-Swift-Web` | `recipes/assemblies/gudule-greeting-swift-web/` |
| `Greeting-Kotlin-Web` | `recipes/assemblies/gudule-greeting-kotlin-web/` |
| `Greeting-Dart-Web` | `recipes/assemblies/gudule-greeting-dart-web/` |
| `Greeting-Python-Web` | `recipes/assemblies/gudule-greeting-python-web/` |
| `Greeting-Csharp-Web` | `recipes/assemblies/gudule-greeting-csharp-web/` |
| `Greeting-Node-Web` ★ | `recipes/assemblies/gudule-greeting-node-web/` |

### Dotnet HostUI × 8 Daemons

| `family_name` | directory |
|---|---|
| `Greeting-Dotnet-Go` | `recipes/assemblies/gudule-greeting-dotnet-go/` |
| `Greeting-Dotnet-Rust` | `recipes/assemblies/gudule-greeting-dotnet-rust/` |
| `Greeting-Dotnet-Swift` | `recipes/assemblies/gudule-greeting-dotnet-swift/` |
| `Greeting-Dotnet-Kotlin` | `recipes/assemblies/gudule-greeting-dotnet-kotlin/` |
| `Greeting-Dotnet-Dart` | `recipes/assemblies/gudule-greeting-dotnet-dart/` |
| `Greeting-Dotnet-Python` | `recipes/assemblies/gudule-greeting-dotnet-python/` |
| `Greeting-Dotnet-Csharp` ★ | `recipes/assemblies/gudule-greeting-dotnet-csharp/` |
| `Greeting-Dotnet-Node` | `recipes/assemblies/gudule-greeting-dotnet-node/` |

### Qt HostUI × 8 Daemons

| `family_name` | directory |
|---|---|
| `Greeting-Qt-Go` | `recipes/assemblies/gudule-greeting-qt-go/` |
| `Greeting-Qt-Rust` | `recipes/assemblies/gudule-greeting-qt-rust/` |
| `Greeting-Qt-Swift` | `recipes/assemblies/gudule-greeting-qt-swift/` |
| `Greeting-Qt-Kotlin` | `recipes/assemblies/gudule-greeting-qt-kotlin/` |
| `Greeting-Qt-Dart` | `recipes/assemblies/gudule-greeting-qt-dart/` |
| `Greeting-Qt-Python` | `recipes/assemblies/gudule-greeting-qt-python/` |
| `Greeting-Qt-Csharp` | `recipes/assemblies/gudule-greeting-qt-csharp/` |
| `Greeting-Qt-Node` | `recipes/assemblies/gudule-greeting-qt-node/` |

★ = full-stack (same-language ecosystem for both daemon and UI)

---

## 5. Assembly Manifest Template

```yaml
schema: holon/v0
kind: composite
given_name: gudule
family_name: Greeting-Flutter-Go
build:
  runner: recipe
  members:
    - path: ../../daemons/gudule-daemon-greeting-go
    - path: ../../hostui/gudule-greeting-hostui-flutter
```

---

## 6. Composition Recipes

### Workers (Go-only)

| `family_name` | directory |
|---|---|
| `Composition-Worker-Compute` | `recipes/composition/workers/charon-worker-compute/` |
| `Composition-Worker-Transform` | `recipes/composition/workers/charon-worker-transform/` |

### Worker Proto Contracts

**compute-worker** — accepts a number, returns its square:

```protobuf
service ComputeService {
  rpc Compute(ComputeRequest) returns (ComputeResponse);
}
message ComputeRequest { int64 value = 1; }
message ComputeResponse { int64 result = 1; }
```

**transform-worker** — accepts text, returns it reversed:

```protobuf
service TransformService {
  rpc Transform(TransformRequest) returns (TransformResponse);
}
message TransformRequest { string text = 1; }
message TransformResponse { string result = 1; }
```

### Direct Call — 11 orchestrators → Go workers

| `family_name` | directory |
|---|---|
| `Composition-Direct-Go-Go` | `recipes/composition/direct-call/charon-direct-go-go/` |
| `Composition-Direct-Rust-Go` | `recipes/composition/direct-call/charon-direct-rust-go/` |
| `Composition-Direct-Swift-Go` | `recipes/composition/direct-call/charon-direct-swift-go/` |
| `Composition-Direct-Kotlin-Go` | `recipes/composition/direct-call/charon-direct-kotlin-go/` |
| `Composition-Direct-Dart-Go` | `recipes/composition/direct-call/charon-direct-dart-go/` |
| `Composition-Direct-Csharp-Go` | `recipes/composition/direct-call/charon-direct-csharp-go/` |
| `Composition-Direct-Node-Go` | `recipes/composition/direct-call/charon-direct-node-go/` |
| `Composition-Direct-Python-Go` | `recipes/composition/direct-call/charon-direct-python-go/` |
| `Composition-Direct-Ruby-Go` | `recipes/composition/direct-call/charon-direct-ruby-go/` |
| `Composition-Direct-Java-Go` | `recipes/composition/direct-call/charon-direct-java-go/` |
| `Composition-Direct-Cpp-Go` | `recipes/composition/direct-call/charon-direct-cpp-go/` |

### Pipeline — 11 orchestrators → Go workers

| `family_name` | directory |
|---|---|
| `Composition-Pipeline-Go-Go` | `recipes/composition/pipeline/charon-pipeline-go-go/` |
| `Composition-Pipeline-Rust-Go` | `recipes/composition/pipeline/charon-pipeline-rust-go/` |
| `Composition-Pipeline-Swift-Go` | `recipes/composition/pipeline/charon-pipeline-swift-go/` |
| `Composition-Pipeline-Kotlin-Go` | `recipes/composition/pipeline/charon-pipeline-kotlin-go/` |
| `Composition-Pipeline-Dart-Go` | `recipes/composition/pipeline/charon-pipeline-dart-go/` |
| `Composition-Pipeline-Csharp-Go` | `recipes/composition/pipeline/charon-pipeline-csharp-go/` |
| `Composition-Pipeline-Node-Go` | `recipes/composition/pipeline/charon-pipeline-node-go/` |
| `Composition-Pipeline-Python-Go` | `recipes/composition/pipeline/charon-pipeline-python-go/` |
| `Composition-Pipeline-Ruby-Go` | `recipes/composition/pipeline/charon-pipeline-ruby-go/` |
| `Composition-Pipeline-Java-Go` | `recipes/composition/pipeline/charon-pipeline-java-go/` |
| `Composition-Pipeline-Cpp-Go` | `recipes/composition/pipeline/charon-pipeline-cpp-go/` |

### Fan-Out — 11 orchestrators → Go workers

| `family_name` | directory |
|---|---|
| `Composition-Fanout-Go-Go` | `recipes/composition/fan-out/charon-fanout-go-go/` |
| `Composition-Fanout-Rust-Go` | `recipes/composition/fan-out/charon-fanout-rust-go/` |
| `Composition-Fanout-Swift-Go` | `recipes/composition/fan-out/charon-fanout-swift-go/` |
| `Composition-Fanout-Kotlin-Go` | `recipes/composition/fan-out/charon-fanout-kotlin-go/` |
| `Composition-Fanout-Dart-Go` | `recipes/composition/fan-out/charon-fanout-dart-go/` |
| `Composition-Fanout-Csharp-Go` | `recipes/composition/fan-out/charon-fanout-csharp-go/` |
| `Composition-Fanout-Node-Go` | `recipes/composition/fan-out/charon-fanout-node-go/` |
| `Composition-Fanout-Python-Go` | `recipes/composition/fan-out/charon-fanout-python-go/` |
| `Composition-Fanout-Ruby-Go` | `recipes/composition/fan-out/charon-fanout-ruby-go/` |
| `Composition-Fanout-Java-Go` | `recipes/composition/fan-out/charon-fanout-java-go/` |
| `Composition-Fanout-Cpp-Go` | `recipes/composition/fan-out/charon-fanout-cpp-go/` |

---

## 7. Test Matrix

| `family_name` | directory |
|---|---|
| `Greeting-Testmatrix` | `recipes/testmatrix/gudule-greeting-testmatrix/` |

---

## 8. Composition Code Snippets

### Direct Call (Go)

```go
conn := connect("charon-worker-compute")
client := pb.NewComputeServiceClient(conn)
resp, err := client.Compute(ctx, &pb.ComputeRequest{Value: 42})
// resp.Result == 1764
```

### Pipeline (Go)

```go
computeConn := connect("charon-worker-compute")
transformConn := connect("charon-worker-transform")

// Step 1: compute
cResp, _ := pb.NewComputeServiceClient(computeConn).
    Compute(ctx, &pb.ComputeRequest{Value: 5})
// cResp.Result == 25

// Step 2: transform the result
tResp, _ := pb.NewTransformServiceClient(transformConn).
    Transform(ctx, &pb.TransformRequest{Text: fmt.Sprint(cResp.Result)})
// tResp.Result == "52"
```

### Fan-Out (Go)

```go
computeConn := connect("charon-worker-compute")
transformConn := connect("charon-worker-transform")

var wg sync.WaitGroup
wg.Add(2)
go func() { computeResult = compute(ctx, computeConn, input); wg.Done() }()
go func() { transformResult = transform(ctx, transformConn, input); wg.Done() }()
wg.Wait()
// aggregate computeResult + transformResult
```
