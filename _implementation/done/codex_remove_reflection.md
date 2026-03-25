# Remove gRPC Reflection — Use `HolonMeta/Describe` Instead

## Context

The Organic Programming constitution (`CONSTITUTION.md`) states:

> gRPC server reflection **may** be enabled as a development convenience
> for tools like `grpcurl`. It is not required by this constitution —
> [Describe](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#11-13) is the canonical introspection mechanism.

Despite this, **every SDK and every holon** currently enables
`grpc.reflection.v1alpha.ServerReflection` by default, and the [op](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/manifest.proto#127-131)
CLI depends on it for dynamic JSON → protobuf serialization.

The `HolonMeta/Describe` service (defined in [_protos/holons/v1/describe.proto](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto))
already returns **everything** needed for dynamic dispatch:
- Field **numbers**, **types**, **labels** (via [FieldDoc](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#61-80))
- Nested message expansion
- Enum values
- Human-readable descriptions and examples

[op](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/manifest.proto#127-131) should call [Describe](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#11-13) **once per connection**, cache the
[DescribeResponse](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#17-24), and use `FieldDoc.number` + `FieldDoc.type`
to dynamically build protobuf messages from JSON — eliminating
the need for gRPC reflection entirely.

## Authoritative References

Read these files first — they define the intended architecture:

- `CONSTITUTION.md` (constitution) — especially the note on reflection
- [COMMUNICATION.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/COMMUNICATION.md) §3.5 — [HolonMeta](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#10-14) specification
- [_protos/holons/v1/describe.proto](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto) — full [DescribeResponse](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#17-24) schema

## Scope

### Phase 1: Update [op](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/manifest.proto#127-131) to use [Describe](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#11-13) instead of reflection

**Target:** `holons/grace-op/`

Currently `op` calls `grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo`
to discover services and build dynamic protobuf messages.

**Change:** When `op` dispatches a gRPC call (any URI scheme):
1. Call `holonmeta.v1.HolonMeta/Describe` (the compiled stubs already exist in `op`)
2. Cache the `DescribeResponse` per connection
3. Look up the target method in `DescribeResponse.services[].methods[]`
4. Use `FieldDoc.number` + `FieldDoc.type` to serialize JSON → protobuf dynamically
5. Use the output `FieldDoc` tree to deserialize the response protobuf → JSON
6. Fall back to reflection **only** if `Describe` is not available (graceful degradation)

**Key files to modify:**
- `holons/grace-op/internal/grpcclient/client.go` — reflection-based dispatch
- `holons/grace-op/internal/cli/stdio_compose.go` — stdio reflection usage
- `holons/grace-op/internal/cli/mem_compose.go` — mem reflection usage
- `holons/grace-op/api/cli_misc.go` — reflection-related CLI logic

**Test:** `holons/grace-op/internal/server/server_test.go`

---

### Phase 2: Make reflection opt-in (OFF by default) in each SDK

For each SDK, change the `Serve` runner so that gRPC reflection is
**disabled by default**. Provide an explicit opt-in flag for developers
who want it for `grpcurl` debugging.

| SDK | File | Current behavior |
|-----|------|-----------------|
| `sdk/go-holons` | `pkg/serve/serve.go` | `Run()` calls `RunWithOptions(..., true)` — reflection ON |
| `sdk/python-holons` | `holons/serve.py` | `reflection.enable_server_reflection(...)` |
| `sdk/ruby-holons` | `lib/holons/serve.rb` | reflection enabled |
| `sdk/js-holons` | `src/serve.js` | reflection loaded from proto |
| `sdk/rust-holons` | `src/serve.rs` | `tonic_reflection::server::Builder` |
| `sdk/csharp-holons` | `Holons/Gen/HolonmetaGrpc.cs` | reflection in generated code |
| `sdk/c-holons` | `cmd/grpc-bridge-go/main.go` | Go bridge with reflection |

**Changes per SDK:**
1. Default `reflect` / `reflection` to `false`
2. Accept an explicit `--reflect` flag or `reflect: true` option to re-enable
3. Update `Serve` runner documentation to note that `Describe` is the
   canonical introspection mechanism

**Do NOT remove the reflection code** — just change the default to OFF.

---

### Phase 3: Remove reflection from each example holon

Each Gabriel greeting holon explicitly enables reflection.
Remove or disable it, since the SDK default will now be OFF.

| Holon | File | What to remove |
|-------|------|---------------|
| `gabriel-greeting-go` | `internal/server.go` | `reflection: true` parameter |
| `gabriel-greeting-swift` | `Sources/.../Server.swift`, `ReflectionProvider.swift`, `ReflectionGRPC.swift`, `grpc/reflection/v1alpha/reflection.pb.swift` | Entire reflection provider + generated stubs |
| `gabriel-greeting-python` | `_internal/server.py` | `grpc_reflection` import + `enable_server_reflection()` |
| `gabriel-greeting-node` | `_internal/server.js` | `reflectionPackageDefinition` loading |
| `gabriel-greeting-rust` | `internal/server.rs` | `tonic_reflection::server::Builder` |
| `gabriel-greeting-java` | `internal/GreetingServer.java` | Reflection service registration |
| `gabriel-greeting-kotlin` | `internal/GreetingServer.kt` | Reflection service registration |
| `gabriel-greeting-csharp` | `_internal/Server.cs` | Reflection service registration |
| `gabriel-greeting-cpp` | `internal/server.cpp` | Reflection service enabling |
| `gabriel-greeting-c` | (uses Go bridge) | Handled by c-holons SDK change |
| `gabriel-greeting-dart` | Check `_internal/` | Remove if present |
| `gabriel-greeting-ruby` | Check `_internal/` | Remove if present |

Also remove from `holons/grace-op/` server if it enables reflection for itself.

---

### Phase 4: Update `op` to remove hard dependency on reflection

After Phase 1 is verified (op uses Describe for dispatch):
- Remove reflection as a **hard requirement** from `op`'s gRPC dispatch
- Keep reflection as an optional fallback (some third-party gRPC
  servers may provide reflection but not Describe)
- Update `op --help` and documentation to reflect this change

---

## Verification

For each example holon, verify:

```bash
# Start the holon
op run gabriel-greeting-go &

# Confirm reflection is OFF
grpcurl -plaintext 127.0.0.1:9090 list
# Expected: "server does not support the reflection API"

# Confirm Describe works
op inspect gabriel-greeting-go
# Expected: full service documentation

# Confirm op dispatch works via Describe
op grpc+tcp://gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
# Expected: greeting response, no reflection errors
```

## Constraints

- **Do NOT remove** the `HolonMeta/Describe` service — it must remain ON by default
- **Do NOT remove** reflection code from SDKs — just change the default to OFF
- **Do NOT modify** `_protos/holons/v1/describe.proto`
- **Do NOT modify** `CONSTITUTION.md` or `COMMUNICATION.md`
- Work **one SDK at a time**, then **one example at a time** — verify each builds before moving to the next
- Follow existing code style in each language
