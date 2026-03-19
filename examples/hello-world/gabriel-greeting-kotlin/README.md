# Gabriel Greeting Kotlin

Reference implementation of a Kotlin holon — a programmatic creature designed for the agentic age. Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names such as `Marie`, `マリア`, and `Мария`. This example demonstrates proto-based identity, a 4-facet split, committed generated Kotlin/Java stubs, and SDK-backed gRPC serving with `Describe` by default plus optional `--reflect` debugging.

## Facets

| Facet | Visibility | Location | Role |
|-------|-----------|----------|------|
| **Code API** | public | `src/main/kotlin/.../api/PublicApi.kt` | Pure functions consuming and returning protobuf types. |
| **CLI** | public | `src/main/kotlin/.../api/Cli.kt` | Parses command-line args, calls the Code API, and formats output. |
| **RPC** | internal | `src/main/kotlin/.../internal/GreetingServer.kt` | gRPC `GreetingService` implementation delegating to the Code API. |
| **Tests** | test | `src/test/kotlin/...` | One test file per facet for Code API, CLI, and RPC. |

## Structure

```text
src/main/kotlin/.../cmd/Main.kt           Entry point — delegates to the CLI.
src/main/kotlin/.../api/PublicApi.kt      Code API.
src/main/kotlin/.../api/Cli.kt            CLI facade.
src/main/kotlin/.../internal/*.kt         RPC server and greeting data.
src/test/kotlin/...                       Code API, CLI, and RPC tests.
api/v1/holon.proto                        Identity manifest.
gen/kotlin/greeting/v1/                   Generated protobuf code (do not edit).
build.gradle.kts                          Gradle build.
```

## How to launch

```bash
gradle test
gradle -q run --args='version'
gradle -q run --args='listLanguages --format json'
gradle -q run --args='sayHello Alice fr'
gradle -q run --args='serve --port 9090 --reflect'
grpcurl -plaintext 127.0.0.1:9090 list
grpcurl -plaintext -d '{"name":"Alice","lang_code":"fr"}' 127.0.0.1:9090 greeting.v1.GreetingService/SayHello
```
