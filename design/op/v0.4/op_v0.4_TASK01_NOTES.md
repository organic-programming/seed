# TASK01 — Implementation Notes

Reference material for TASK01 subtasks. Contains proto contracts,
code snippets, and the full assembly listing.

---

## Shared Worker Proto Contracts

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

Workers are intentionally trivial — the focus is the
**orchestrator's composition logic**, not domain logic.

---

## Composition Code Snippets

### Direct Call (Go)

```go
conn := connect("compute-worker")    // discover → start → dial → ready
client := pb.NewComputeServiceClient(conn)
resp, err := client.Compute(ctx, &pb.ComputeRequest{Value: 42})
// resp.Result == 1764
```

### Pipeline (Go)

```go
computeConn := connect("compute-worker")
transformConn := connect("transform-worker")

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
computeConn := connect("compute-worker")
transformConn := connect("transform-worker")

var wg sync.WaitGroup
wg.Add(2)
go func() { computeResult = compute(ctx, computeConn, input); wg.Done() }()
go func() { transformResult = transform(ctx, transformConn, input); wg.Done() }()
wg.Wait()
// aggregate computeResult + transformResult
```

---

## Full Assembly Listing (48)

8 daemons × 6 HostUIs. Highlighted combinations are "full stack"
(same-language daemon + native UI).

| Daemon | swiftui | flutter | kotlin | web | dotnet | qt |
|---|---|---|---|---|---|---|
| **go** | go-swift | go-dart | go-kotlin | go-web | go-dotnet | go-qt |
| **rust** | rust-swift | rust-dart | rust-kotlin | rust-web | rust-dotnet | rust-qt |
| **python** | python-swift | python-dart | python-kotlin | python-web | python-dotnet | python-qt |
| **swift** | **swift-swift** ★ | swift-dart | swift-kotlin | swift-web | swift-dotnet | swift-qt |
| **kotlin** | kotlin-swift | kotlin-dart | **kotlin-kotlin** ★ | kotlin-web | kotlin-dotnet | kotlin-qt |
| **dart** | dart-swift | **dart-dart** ★ | dart-kotlin | dart-web | dart-dotnet | dart-qt |
| **csharp** | csharp-swift | csharp-dart | csharp-kotlin | csharp-web | **csharp-dotnet** ★ | csharp-qt |
| **node** | node-swift | node-dart | node-kotlin | **node-web** ★ | node-dotnet | node-qt |

★ = Full stack (same-lang ecosystem for both daemon and UI)

---

## Composition Directory Template

Each composition recipe follows this layout:

```
composition/<pattern>/<caller-lang>/
├── holon.yaml          ← composite: members = [orchestrator, worker(s)]
├── orchestrator/
│   ├── holon.yaml
│   ├── cmd/main.go     ← (or src/main.rs, etc.)
│   └── protos/         ← imports worker proto(s)
├── (compute-worker → ../../workers/compute-worker)
└── (transform-worker → ../../workers/transform-worker)
```

Each recipe is self-contained. Worker references use relative
paths to the shared `workers/` directory.
