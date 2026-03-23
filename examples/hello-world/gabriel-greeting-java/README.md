# Gabriel Greeting Java

Reference implementation of a Java holon — a programmatic creature designed for the agentic age. Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names such as `Marie`, `マリア`, and `Мария`. This example demonstrates proto-based identity, a 4-facet split, committed generated Java stubs, and SDK-backed gRPC serving with `Describe` by default plus optional `--reflect` debugging.

## Facets

| Facet | Visibility | Location | Role |
|-------|-----------|----------|------|
| **Code API** | public | `src/main/java/.../api/PublicApi.java` | Pure functions consuming and returning protobuf types. |
| **CLI** | public | `src/main/java/.../api/Cli.java` | Parses command-line args, calls the Code API, and formats output. |
| **RPC** | internal | `src/main/java/.../internal/GreetingServer.java` | gRPC `GreetingService` implementation delegating to the Code API. |
| **Tests** | test | `src/test/java/...` | One test file per facet for Code API, CLI, and RPC. |

## Structure

```text
src/main/java/.../cmd/Main.java           Entry point — delegates to the CLI.
src/main/java/.../api/PublicApi.java      Code API.
src/main/java/.../api/Cli.java            CLI facade.
src/main/java/.../internal/*.java         RPC server and greeting data.
src/test/java/...                         Code API, CLI, and RPC tests.
api/v1/holon.proto                        Identity manifest.
gen/java/greeting/v1/                     Generated protobuf code (do not edit).
build.gradle                              Gradle build.
```

## How to launch

```bash
gradle test
java -cp build/classes/java/main:build/resources/main org.organicprogramming.gabriel.greeting.javaholon.cmd.Main version
java -cp build/classes/java/main:build/resources/main org.organicprogramming.gabriel.greeting.javaholon.cmd.Main listLanguages --format json
java -cp build/classes/java/main:build/resources/main org.organicprogramming.gabriel.greeting.javaholon.cmd.Main sayHello Bob fr
java -cp build/classes/java/main:build/resources/main org.organicprogramming.gabriel.greeting.javaholon.cmd.Main serve --port 9090 --reflect
grpcurl -plaintext 127.0.0.1:9090 list
grpcurl -plaintext -d '{"name":"Bob","lang_code":"fr"}' 127.0.0.1:9090 greeting.v1.GreetingService/SayHello
```
