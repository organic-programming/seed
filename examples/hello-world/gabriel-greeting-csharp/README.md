# Gabriel Greeting Csharp

Reference implementation of a C# holon — a programmatic creature designed for the agentic age. Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names such as `Marie`, `マリア`, and `Мария`. This example demonstrates proto-based identity, a 4-facet split, committed generated C# stubs, and SDK-backed gRPC serving with `Describe` by default plus optional `--reflect` debugging.

## Facets

| Facet | Visibility | Location | Role |
|-------|-----------|----------|------|
| **Code API** | public | `api/*.cs` | Pure functions consuming and returning protobuf types. |
| **CLI** | public | `api/Cli.cs` | Parses command-line args, calls the Code API, and formats output. |
| **RPC** | internal | `_internal/*.cs` | gRPC `GreetingService` implementation delegating to the Code API. |
| **Tests** | test | `tests/**/*.cs` | One test file per facet for Code API, CLI, and RPC. |

## Structure

```text
cmd/Main.cs                      Entry point — delegates to the CLI.
api/PublicApi.cs                 Code API.
api/Cli.cs                       CLI facade.
_internal/*.cs                   RPC server and greeting data.
tests/**/*.cs                    Code API, CLI, and RPC tests.
api/v1/holon.proto               Identity manifest.
gen/csharp/                      Generated protobuf code (do not edit).
gabriel-greeting-csharp.csproj   Main project.
tests/*.csproj                   Test project.
```

## How to launch

```bash
dotnet test tests/Gabriel.Greeting.Csharp.Tests.csproj
dotnet run --project gabriel-greeting-csharp.csproj -- version
dotnet run --project gabriel-greeting-csharp.csproj -- listLanguages --format json
dotnet run --project gabriel-greeting-csharp.csproj -- sayHello Bob fr
dotnet run --project gabriel-greeting-csharp.csproj -- serve --port 9090 --reflect
grpcurl -plaintext 127.0.0.1:9090 list
grpcurl -plaintext -d '{"name":"Bob","lang_code":"fr"}' 127.0.0.1:9090 greeting.v1.GreetingService/SayHello
```
