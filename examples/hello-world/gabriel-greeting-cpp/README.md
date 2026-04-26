# Gabriel Greeting Cpp

Reference implementation of a C++ holon. This port mirrors the Gabriel greeting shape from the Go reference: proto manifest, 4-facet split, committed generated stubs, and SDK-managed gRPC serving with reflection.

Gabriel exposes `SayHello` and `ListLanguages` over the shared greeting contract and ships the full 56-language catalog with localized default names such as `Marie`, `マリア`, and `Мария`.

## Discovery

This holon is source-discoverable from the repo root:

```bash
op list --source
```

Programmatically:

```text
Discover(LOCAL, "gabriel-greeting-cpp", null, SOURCE, NO_LIMIT, NO_TIMEOUT)
```

Today this works in the Go SDK. The other non-browser SDKs will support the same source lookup once their Phase 1 discovery tasks land. The browser SDK is excluded because it has no filesystem-based discovery.

## Facets

| Facet | Visibility | Location | Role |
|-------|-----------|----------|------|
| **Code API** | public | `api/public.*` | Pure functions consuming and returning protobuf types. |
| **CLI** | public | `api/cli.*`, `cmd/main.cpp` | Parses arguments, calls the Code API, formats text or JSON, and serves gRPC. |
| **RPC** | internal | `internal/server.*` | gRPC `GreetingService` implementation delegating to the Code API. |
| **Tests** | test | `api/*_test.cpp`, `internal/server_test.cpp` | One test file per facet. |

## Structure

```text
cmd/main.cpp                       Entry point.
api/public.hpp                     Code API.
api/cli.hpp                        CLI facade.
api/v1/holon.proto                 Proto manifest.
internal/greetings.*               56-language greeting catalog.
internal/server.*                  gRPC service implementation.
gen/cpp/greeting/v1/               Generated protobuf and gRPC C++ code.
scripts/generate_proto.sh          Regenerates stubs and validates holon.proto.
CMakeLists.txt                     Build and test definition.
```

## How to launch

```bash
op sdk install cpp
op sdk verify cpp
export OP_SDK_CPP_PATH="$(op sdk path cpp)"
./scripts/generate_proto.sh
cmake -S . -B build
cmake --build build
ctest --test-dir build --output-on-failure
./build/gabriel-greeting-cpp version
./build/gabriel-greeting-cpp listLanguages --format json
./build/gabriel-greeting-cpp sayHello Bob fr
./build/gabriel-greeting-cpp serve --port 9090
grpcurl -plaintext 127.0.0.1:9090 list
grpcurl -plaintext -d '{"name":"Bob","lang_code":"fr"}' 127.0.0.1:9090 greeting.v1.GreetingService/SayHello
```
