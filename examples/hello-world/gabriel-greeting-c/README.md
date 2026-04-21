# Gabriel Greeting C

Reference implementation of a C holon. This port keeps the Gabriel 4-facet split while using the `c-holons` bridge for public gRPC serving, driven directly from `api/v1/holon.proto` with no YAML compatibility manifest.

The public binary exposes the Gabriel CLI and delegates `serve` to the SDK bridge. A private backend binary implements the internal unary HTTP/JSON server that the bridge forwards to. The greeting catalog covers the full 56 languages with localized default names.

## Discovery

This holon is source-discoverable from the repo root:

```bash
op list --source
```

Programmatically:

```text
Discover(LOCAL, "gabriel-greeting-c", null, SOURCE, NO_LIMIT, NO_TIMEOUT)
```

Today this works in the Go SDK. The other non-browser SDKs will support the same source lookup once their Phase 1 discovery tasks land. The browser SDK is excluded because it has no filesystem-based discovery.

## Facets

| Facet | Visibility | Location | Role |
|-------|-----------|----------|------|
| **Code API** | public | `api/public.*` | Pure functions consuming and returning generated protobuf message types. |
| **CLI** | public | `api/cli.*`, `cmd/main.c` | Parses arguments, formats text or JSON, and launches the bridge-backed gRPC server. |
| **RPC** | internal | `internal/server.*`, `cmd/backend_main.c` | Private HTTP/JSON backend used by the SDK bridge to satisfy the gRPC contract. |
| **Tests** | test | `api/*_test.c`, `internal/server_test.c` | One test file per facet. |

## Structure

```text
cmd/main.c                         Public CLI entry point.
cmd/backend_main.c                 Private backend entry point for the bridge.
api/public.h                       Code API.
api/cli.h                          CLI facade.
api/v1/holon.proto                 Proto manifest.
internal/greetings.*               56-language greeting catalog.
internal/server.*                  Backend server and bridge launcher.
gen/c/greeting/v1/                 Generated upb C code.
scripts/generate_proto.sh          Regenerates stubs and validates holon.proto.
CMakeLists.txt                     Build and test definition.
```

## How to launch

```bash
./scripts/generate_proto.sh
cmake -S . -B build
cmake --build build
ctest --test-dir build --output-on-failure
./build/gabriel-greeting-c version
./build/gabriel-greeting-c listLanguages --format json
./build/gabriel-greeting-c sayHello Bob fr
./build/gabriel-greeting-c serve --port 9090
grpcurl -plaintext 127.0.0.1:9090 list
grpcurl -plaintext -d '{"name":"Bob","lang_code":"fr"}' 127.0.0.1:9090 greeting.v1.GreetingService/SayHello
```
