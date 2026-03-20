# Migrate HolonMeta stubs to canonical `_protos/holons/v1/` — No Copy

## Goal

Delete every private proto copy and generated `holonmeta/` directory from the
SDKs. Regenerate stubs from the single canonical source
`_protos/holons/v1/{coax,describe,manifest}.proto` (`package holons.v1`).
Add a top-level generation script so any future proto change can be propagated
to all SDKs in one command. Verify every `examples/hello-world/` holon still
builds and runs. Ignore `examples/legacy/`.

## Canonical proto files (DO NOT MODIFY)

| File | Package | Imports |
|------|---------|---------|
| `_protos/holons/v1/manifest.proto` | `holons.v1` | `google/protobuf/descriptor.proto` |
| `_protos/holons/v1/describe.proto` | `holons.v1` | `holons/v1/manifest.proto` |
| `_protos/holons/v1/coax.proto` | `holons.v1` | `holons/v1/manifest.proto` |

> [!CAUTION]
> **Wire-format breaking change**: the canonical `DescribeResponse` uses
> `HolonManifest manifest = 1; repeated ServiceDoc services = 2;` while the
> old SDK copies used flat fields: `string slug = 1; string motto = 2;
> repeated ServiceDoc services = 3; string version = 4;`.
> Every SDK's `BuildResponse` / `buildDescribeResponse` must be rewritten to
> populate the `HolonManifest` message.

## What to delete

### Proto source copies (2 files)
- `sdk/go-holons/protos/holonmeta/` (entire directory)
- `sdk/cpp-holons/protos/holonmeta/` (entire directory)

### Generated code directories (15 directories)
- `sdk/c-holons/gen/c/holonmeta/`
- `sdk/cpp-holons/gen/cpp/holonmeta/`
- `sdk/csharp-holons/Holons/Gen/` (HolonmetaGrpc.cs and related)
- `sdk/dart-holons/lib/gen/holonmeta/`
- `sdk/go-holons/gen/go/holonmeta/`
- `sdk/java-holons/src/main/java/gen/holonmeta/`
- `sdk/js-holons/src/gen/holonmeta/`
- `sdk/js-web-holons/src/gen/holonmeta/`
- `sdk/kotlin-holons/src/main/kotlin/gen/holonmeta/`
- `sdk/python-holons/gen/python/holonmeta/`
- `sdk/ruby-holons/lib/gen/holonmeta/`
- `sdk/rust-holons/src/gen/` (holonmeta.v1.rs)
- `sdk/swift-holons/Sources/Holons/Gen/holonmeta/`

## What to generate

Regenerate stubs from `_protos/holons/v1/*.proto` into new `gen/*/holons/v1/`
directories per SDK. Each proto file imports `manifest.proto`, so manifest
types must also be generated.

### Per-SDK generation (from workspace root)

All commands use `-I _protos` as the import path root.

**Go** (`sdk/go-holons`):
```shell
protoc -I _protos \
  --go_out=sdk/go-holons/gen/go --go_opt=paths=source_relative \
  --go-grpc_out=sdk/go-holons/gen/go --go-grpc_opt=paths=source_relative \
  _protos/holons/v1/manifest.proto \
  _protos/holons/v1/describe.proto \
  _protos/holons/v1/coax.proto
```
Update `go_package` option: each proto needs
`option go_package = "github.com/organic-programming/go-holons/gen/go/holons/v1;v1";`
— add this via a `buf` override or a wrapper proto, NOT by editing the
canonical proto (which must stay SDK-agnostic).

**Swift** (`sdk/swift-holons`): Uses SwiftProtobuf plugin. Check the existing
`Package.swift` configuration.

**Python** (`sdk/python-holons`):
```shell
python -m grpc_tools.protoc -I _protos \
  --python_out=sdk/python-holons/gen/python \
  --grpc_python_out=sdk/python-holons/gen/python \
  _protos/holons/v1/*.proto
```

**C++** (`sdk/cpp-holons`):
```shell
protoc -I _protos \
  --cpp_out=sdk/cpp-holons/gen/cpp \
  --grpc_out=sdk/cpp-holons/gen/cpp \
  --plugin=protoc-gen-grpc=$(which grpc_cpp_plugin) \
  _protos/holons/v1/*.proto
```

**C** (`sdk/c-holons`):
```shell
protoc-c -I _protos \
  --c_out=sdk/c-holons/gen/c \
  _protos/holons/v1/*.proto
```

**C#** (`sdk/csharp-holons`):
```shell
protoc -I _protos \
  --csharp_out=sdk/csharp-holons/Holons/Gen \
  --grpc_out=sdk/csharp-holons/Holons/Gen \
  --plugin=protoc-gen-grpc=$(which grpc_csharp_plugin) \
  _protos/holons/v1/*.proto
```

**Dart** (`sdk/dart-holons`):
```shell
protoc -I _protos \
  --dart_out=grpc:sdk/dart-holons/lib/gen \
  _protos/holons/v1/*.proto
```

**Java** (`sdk/java-holons`): Uses Gradle protobuf plugin — update source set
in `build.gradle` to point to `../../_protos`.

**Kotlin** (`sdk/kotlin-holons`): Same as Java — update Gradle config.

**JS Node** (`sdk/js-holons`):
```shell
grpc_tools_node_protoc -I _protos \
  --js_out=import_style=commonjs:sdk/js-holons/src/gen \
  --grpc_out=grpc_js:sdk/js-holons/src/gen \
  _protos/holons/v1/*.proto
```

**JS Web** (`sdk/js-web-holons`): Same tools, ESM output.

**Ruby** (`sdk/ruby-holons`):
```shell
grpc_tools_ruby_protoc -I _protos \
  --ruby_out=sdk/ruby-holons/lib/gen \
  --grpc_out=sdk/ruby-holons/lib/gen \
  _protos/holons/v1/*.proto
```

**Rust** (`sdk/rust-holons`): Uses `tonic-build` or `prost-build` via
`build.rs` — update the proto path to `../../_protos/holons/v1/`.

## Generation script

Create `scripts/generate-protos.sh` at the workspace root:
- Takes an optional `--sdk=<name>` argument to regenerate a single SDK
- Without arguments, regenerates all SDKs
- Each SDK section is a function so it can be called independently
- Validates that `_protos/holons/v1/describe.proto` exists before running
- Prints a summary of what was generated

## What to update in hand-written code

For each SDK's `describe.*` / `serve.*` / `connect.*` files:

1. **Import paths**: `gen/*/holonmeta/v1` → `gen/*/holons/v1`
2. **Type prefixes** where applicable (e.g. Swift: `Holonmeta_V1_*` → `Holons_V1_*`)
3. **Service name constants**: `"holonmeta.v1.HolonMeta"` → `"holons.v1.HolonMeta"`
4. **Method paths**: `"/holonmeta.v1.HolonMeta/Describe"` → `"/holons.v1.HolonMeta/Describe"`
5. **DescribeResponse construction**: populate `manifest` field (a `HolonManifest`
   message) instead of flat `slug`, `motto`, `version` fields
6. **DescribeResponse reading** (consumers like `grace-op`): read identity from
   `response.manifest.identity` instead of `response.slug` / `response.motto`

### Already migrated (string constants only)
- `sdk/swift-holons`: `HolonMetaGRPC.swift`, `Describe.swift`, `Connect.swift`
- `sdk/go-holons`: `pkg/describe/describe.go` (constant only)
- `sdk/swift-holons/Tests/`: `ServeTests.swift`, `ConnectTests.swift`

### Also update consumers
- `holons/grace-op/internal/grpcclient/describe_catalog.go`: reads `Slug`,
  `Motto`, `Version` from the old flat response — must read from `Manifest`

## Documentation to update

- `HOLON_PACKAGE.md`: replace `holonmeta.v1` references
- `HOLON_COMMUNICATION_PROTOCOL.md`: replace `holonmeta.v1` references

## DO NOT TOUCH

- `examples/legacy/` — historical, leave as-is
- `_implementation/done/` — historical records

## Verification

### 1. Zero remaining references
```shell
grep -r "holonmeta" \
  --include="*.go" --include="*.swift" --include="*.rs" \
  --include="*.py" --include="*.java" --include="*.kt" \
  --include="*.dart" --include="*.js" --include="*.mjs" \
  --include="*.rb" --include="*.cs" --include="*.c" \
  --include="*.h" --include="*.hpp" --include="*.cc" \
  --include="*.proto" --include="*.md" \
  --exclude-dir=legacy --exclude-dir=_implementation \
  --exclude-dir=build --exclude-dir=node_modules \
  --exclude-dir=vendor \
  .
```
Expected: zero matches.

### 2. Per-SDK build check
- **Go**: `cd sdk/go-holons && go build ./...`
- **Swift**: `cd sdk/swift-holons && swift build`
- **Rust**: `cd sdk/rust-holons && cargo build`
- **Python**: `cd sdk/python-holons && python -c "from holons.describe import *"`
- **C++**: `cd sdk/cpp-holons && cmake --build build`
- **Java**: `cd sdk/java-holons && gradle build`
- **Kotlin**: `cd sdk/kotlin-holons && gradle build`
- **Dart**: `cd sdk/dart-holons && dart analyze`
- **JS**: `cd sdk/js-holons && node -e "require('./src/describe')"`
- **Ruby**: `cd sdk/ruby-holons && ruby -e "require_relative 'lib/holons/describe'"`
- **C#**: `cd sdk/csharp-holons && dotnet build`
- **C**: `cd sdk/c-holons && make test`

### 3. Hello-world examples (each must `op build` + `op run` successfully)
For each example in `examples/hello-world/gabriel-greeting-*`:
```shell
op build gabriel-greeting-go
op build gabriel-greeting-swift
op build gabriel-greeting-rust
# ... etc for all 12 + the SwiftUI app
```
Then run one and verify Describe + Greet:
```shell
op run gabriel-greeting-app-swiftui
# In another terminal, with COAX enabled:
op grpc+tcp://127.0.0.1:<PORT> Greet '{"name":"Test"}'
```

### 4. Generation script smoke test
```shell
# Regenerate all SDKs
./scripts/generate-protos.sh

# Verify no diff (should be clean since we just generated)
git diff --stat sdk/
```
