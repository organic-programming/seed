# Gabriel Greeting Go

Reference implementation of a Go holon. Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names (e.g. "Marie" in French, "マリア" in Japanese, "Мария" in Russian). Beyond the classic Hello World, this example demonstrates proto-based identity, layered facets, SDK-managed serving, and the transport cascade across all supported platforms.

This holon is built with the [Go SDK](https://github.com/organic-programming/go-holons) (`go-holons`). Every holon implementation relies on its language SDK for serving, transport negotiation, and discovery integration. SDKs are currently available for Go, Dart, Swift, Kotlin, and Rust — all still in active development.

# A Proto + 4 facets is all you need.

## Protos 

The holon's `api/v1/holon.proto` imports from two shared `_protos` directories (no copy, no symlink):

| Path | Scope | Content |
|------|-------|---------|
| `../../_protos/` | Platform | System types (`holons/v1/manifest.proto`, `describe.proto`) |
| `../_protos/` | Domain | Shared contract (`v1/greeting.proto` — service + messages) |
| `api/v1/holon.proto` | Local | Holon identity manifest + Go-specific options |


## Facets

A holon exposes two kinds of facets:

### 4 Innate facets

| Facet | Visibility | File | Role |
|-------|-----------|------|------|
| **Code API** | `api/` (public) | [public.go](api/public.go) | Pure functions consuming and returning protobuf types. No I/O, no server. The single source of truth for business logic — every other facet delegates here. |
| **CLI** | `api/` (public) | [cli.go](api/cli.go) | Parses `os.Args`, calls Code API, formats output (text or JSON). Standalone binary entry point via `cmd/main.go`. |
| **RPC** | `internal/` | [server.go](internal/server.go) | gRPC `GreetingServiceServer` implementation. Adapts proto request/response to internal logic. Exposed via `serve` sub-command to `op`, other holons, or any gRPC client. |
| **Tests** | `api/`, `internal/` | [*_test.go](api/cli_test.go) | One test file per facet. Internal tests are a standard — TDD is the recommended approach. Validates Code API, CLI args/output, and RPC contract independently. |

### 3 Acquired facets — traits gained through `op`

These facets emerge from the proto contract and manifest. The holon writes no code for them.

| Facet | Provided by | Role |
|-------|------------|------|
| **MCP** | `op mcp` | Exposes RPCs as MCP tools for LLM clients. Proto comments become JSON Schema. |
| **Skills** | manifest | Declared holon capabilities discoverable by agents and orchestrators. |
| **Sequences** | manifest | Multi-step workflows composed from the holon's RPCs. |

## Exposure

Facets split into two contexts:

| Context | Innate | Acquired |
|---------|--------|----------|
| **Dev time** | Code API, Tests | — |
| **Runtime** | CLI, RPC | MCP, Skills, Sequences |


# Serve

The `serve` sub-command is a rich feature provided by the [Go SDK](https://github.com/organic-programming/go-holons) (`pkg/serve`). It handles listener negotiation, reflection, and graceful shutdown — the holon only registers its gRPC service.

When a user runs:

```bash
op gabriel-greeting-go SayHello '{"name":"Alice","lang_code":"en"}'
```

`op` performs the following chain:

1. **Discover** — locates the holon binary via `OPPATH`.
2. **Start** — spawns the holon with `serve`.
3. **Connect** — the SDK selects the best transport automatically, trying each in cascade order until one succeeds.
4. **Call** — sends the `SayHello` RPC and prints the response.

### Connect chain by platform

| Platform | Mode | Connect cascade |
|----------|------|-----------------|
| macOS | binary | `mem → stdio → unix → tcp → rest+sse` |
| Linux | binary | `mem → stdio → unix → tcp → rest+sse` |
| Windows | binary | `mem → stdio → tcp → rest+sse` |
| iOS | framework | `mem → tcp → rest+sse` |
| Android | framework | `mem → tcp → rest+sse` |
| Browser | WASM | `mem → rest+sse` |

The holon itself knows nothing about discovery or transport selection — `serve` and `op` handle it.

## Structure

```
cmd/main.go            Entry point — delegates to the CLI.
api/public.go          Code API — importable functions (SayHello, ListLanguages).
api/cli.go             CLI facade — human / script interface to the Code API.
api/v1/holon.proto     Identity manifest — proto-based holon descriptor.
internal/server.go     RPC server — gRPC implementation (serve sub-command).
internal/greetings.go  Greeting data — 56 languages.
gen/                   Generated protobuf code (do not edit).
```

`internal/` is not a facet — it is an **encapsulation practice**: private domain data and helpers, never imported outside the module.

# MCP exposure

`op mcp` turns any holon into an [MCP](https://modelcontextprotocol.io) tool server — with zero code changes.

```bash
op mcp gabriel-greeting-go
```

`op` connects to the holon, introspects its gRPC contract via reflection, and exposes each RPC as an MCP tool over stdio. Proto comments (`@example`, `@required`) become the tool's JSON Schema descriptions and examples automatically.

Tools are fully qualified — `gabriel-greeting-go.GreetingService.SayHello` — allowing multiple holons to be served from a single `op mcp` instance.

### Advertised tool definition

This is the exact tool definition that `op mcp` advertises to MCP clients — generated entirely from the proto contract:

```json
{
  "name": "gabriel-greeting-go.GreetingService.SayHello",
  "description": "Greets the user in the chosen language.",
  "inputSchema": {
    "properties": {
      "name": {
        "description": "Name to greet. If empty, the daemon falls back to a localized default.",
        "type": "string"
      },
      "lang_code": {
        "description": "ISO 639-1 code chosen by the UI.",
        "type": "string"
      }
    },
    "required": ["lang_code"]
  }
}
```

Every field comes from the proto source — no MCP-specific code, no configuration file:

| MCP field | Proto source |
|-----------|-------------|
| `name` | `<holon-slug>.<service>.<rpc>` |
| `description` | RPC comment in `greeting.proto` |
| `properties` | Message field names + types |
| `description` (per field) | Field comments + `@example` annotations |
| `required` | Fields annotated with `@required` |

<!-- don't modify the following section-->

## How to launch

```bash
op gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
op grpc://gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+stdio://gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+tcp://gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
```

## Currently not supported .

mem, unix, ws, ws, sse+rest 

```bash
op grpc+mem://gabriel-greeting-go SayHello '{"name":"Alice","lang_code":"en"}'
op gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"fr"}'  
```

# How to compile manually the [holon.proto](v1/holon.proto)

```bash
cd examples/hello-world/gabriel-greeting-go/v1
protoc --proto_path=. --proto_path=../../../../_protos --proto_path=../../../_protos holon.proto --descriptor_set_out=/dev/null
```
<!-- don't modify preeceeding section -->

