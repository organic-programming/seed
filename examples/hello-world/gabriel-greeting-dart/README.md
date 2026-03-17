# Gabriel Greeting Dart

Reference implementation of a Dart holon — a programmatic creature designed for the agentic age. Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names such as `Marie`, `マリア`, and `Мария`. This example shows proto-based identity, a 4-facet split, Dart protobuf code generation, and SDK-managed serving.

This holon is built with the [Dart SDK](../../../sdk/dart-holons) (`holons`).

# A Proto + 4 facets is all you need.

## Protos

The holon's `api/v1/holon.proto` imports from two shared `_protos` directories (no copy, no symlink):

| Path | Scope | Content |
|------|-------|---------|
| `../../../_protos/` | Platform | System types (`holons/v1/manifest.proto`) |
| `../../_protos/` | Domain | Shared contract (`v1/greeting.proto` — service + messages) |
| `api/v1/holon.proto` | Local | Holon identity manifest + Dart-specific metadata |

## Facets

| Facet | Visibility | File | Role |
|-------|-----------|------|------|
| **Code API** | `api/` (public) | [public.dart](api/public.dart) | Pure functions consuming and returning protobuf types. |
| **CLI** | `api/` (public) | [cli.dart](api/cli.dart) | Parses command-line args, calls the Code API, and formats output. |
| **RPC** | `_internal/` | [server.dart](_internal/server.dart) | gRPC `GreetingService` implementation delegating to the Code API. |
| **Tests** | `api/`, `_internal/` | [*_test.dart](api/public_test.dart) | One test file per facet for Code API, CLI, and RPC. |

## Structure

```text
cmd/main.dart                  Entry point — delegates to the CLI.
api/public.dart                Code API — importable functions.
api/cli.dart                   CLI facade — human / script interface.
api/v1/holon.proto             Identity manifest — proto-based holon descriptor.
_internal/server.dart          RPC server — gRPC implementation.
_internal/greetings.dart       Greeting data — 56 languages with default names.
gen/dart/greeting/v1/          Generated protobuf code (do not edit).
scripts/generate_proto.sh      Regenerates protobuf code and validates holon.proto.
```

`_internal/` keeps the server and greeting catalog off the public import surface.

## How to launch

```bash
dart pub get
./scripts/generate_proto.sh
dart cmd/main.dart version
dart cmd/main.dart listLanguages --format json
dart cmd/main.dart sayHello Alice fr
dart cmd/main.dart serve --port 9090
grpcurl -plaintext \
  -import-path ../../_protos \
  -proto v1/greeting.proto \
  -d '{"name":"Alice","lang_code":"fr"}' \
  127.0.0.1:9090 \
  greeting.v1.GreetingService/SayHello
```
