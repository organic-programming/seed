# Gabriel Greeting Go

Reference implementation of a Go holon — a programmatic creature designed for the agentic age. Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names (e.g. "Marie" in French, "マリア" in Japanese, "Мария" in Russian). Beyond the classic Hello World, this example demonstrates proto-based identity, layered facets, SDK-managed serving, and the transport cascade across all supported platforms.

This holon is built with the [Go SDK](https://github.com/organic-programming/go-holons) (`go-holons`).

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

### 4 X Innate facets

| Facet | Visibility | File | Role |
|-------|-----------|------|------|
| **Code API** | `api/` (public) | [public.go](api/public.go) | Pure functions consuming and returning protobuf types. No I/O, no server. The single source of truth for business logic — every other facet delegates here. |
| **CLI** | `api/` (public) | [cli.go](api/cli.go) | Parses `os.Args`, calls Code API, formats output (text or JSON). Standalone binary entry point via `cmd/main.go`. |
| **RPC** | `internal/` | [server.go](internal/server.go) | gRPC `GreetingServiceServer` implementation. Adapts proto request/response to internal logic. Exposed via `serve` sub-command to `op`, other holons, or any gRPC client. |
| **Tests** | `api/`, `internal/` | [*_test.go](api/cli_test.go) | One test file per facet. Internal tests are a standard — TDD is the recommended approach. Validates Code API, CLI args/output, and RPC contract independently. |

> **Surface symmetry** — Gabriel exposes two RPCs (`SayHello`,
> `ListLanguages`). Both are callable through the Code API, the CLI, the
> RPC server, and covered by tests — same surface, four projections.
> The internal greeting table (56 languages) is private volume; the
> external skin stays minimal and uniform.

### 3 X Acquired facets — traits gained through `op`

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

The `serve` sub-command is a rich feature provided by the [Go SDK](https://github.com/organic-programming/go-holons) (`pkg/serve`). It handles listener negotiation, `Describe`, optional `--reflect` debugging, and graceful shutdown — the holon only registers its gRPC service.

When a user runs:

```bash
op gabriel-greeting-go SayHello '{"name":"Bob","lang_code":"en"}'
```

`op` performs the following chain:

1. **Discover** — locates the holon binary via `OPPATH`.
2. **Start** — spawns the holon with `serve`.
3. **Connect** — the SDK selects the best transport automatically, trying each in cascade order until one succeeds.
4. **Call** — sends the `SayHello` RPC and prints the response.

### Connect chain by platform

| Platform | Mode | Connect cascade |
|----------|------|-----------------|
| macOS | binary | `stdio → unix → tcp → rest+sse` |
| Linux | binary | `stdio → unix → tcp → rest+sse` |
| Windows | binary | `stdio → tcp → rest+sse` |
| iOS | framework | `tcp → rest+sse` |
| Android | framework | `tcp → rest+sse` |
| Browser | WASM | `rest+sse` |

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

# Acquired facets

Each acquired facet builds on the previous — from exposing individual RPCs, to composing them into batches, to guiding agents on when and why to use them.

## MCP — expose individual RPCs

`op mcp` turns any holon into an [MCP](https://modelcontextprotocol.io) tool server — with zero code changes.

```bash
op mcp gabriel-greeting-go
```

`op` connects to the holon, introspects its contract via the `Describe` RPC, and exposes each RPC as an MCP tool over stdio. Proto comments (`@example`, `@required`) become the tool's JSON Schema descriptions and examples automatically.

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

## Sequences — compose RPCs into deterministic batches

A sequence is a deterministic batch of steps declared in the proto manifest. No reasoning between steps — just execute.

```bash
op do gabriel-greeting-go greeting-fr-ja-ru-en --name=""
op do gabriel-greeting-go greeting-fr-ja-ru-en --name=Joseph
op do gabriel-greeting-go greeting-fr-ja-ru-en --name="" --dry-run
op do gabriel-greeting-go greeting-fr-ja-ru-en --name=Joseph --dry-run
```

The sequence declares a required `name` parameter. Passing `--name=""` is intentional: Gabriel then falls back to the localized default name for each language (`Marie`, `マリア`, `Мария`, `Mary`). Passing a real name such as `Joseph` forces the same name through every step.

Dry run:

```
[1/5] op gabriel-greeting-go ListLanguages
[2/5] op gabriel-greeting-go SayHello '{"name":"","lang_code":"fr"}'
[3/5] op gabriel-greeting-go SayHello '{"name":"","lang_code":"ja"}'
[4/5] op gabriel-greeting-go SayHello '{"name":"","lang_code":"ru"}'
[5/5] op gabriel-greeting-go SayHello '{"name":"","lang_code":"en"}'
```

Real run with `--name=Joseph` will greet Joseph in all four languages. Flags: `--dry-run` prints steps without executing, `--continue-on-error` skips failures.

## Skills — guide agents on *when* and *why* to act

A skill describes when and why to use a holon. Agents discover skills via `op inspect` or MCP and use them to decide which holon to call.

```bash
op inspect gabriel-greeting-go
```

Gabriel declares one skill:

| Skill | When | Steps |
|-------|------|-------|
| **multilingual-greeter** | The user wants to greet someone in a specific language. | 1. ListLanguages → 2. Choose name + lang → 3. SayHello |

An LLM agent reads the skill, decides it matches the user's intent, and calls the RPCs step by step with reasoning between each call.

## MCP vs Sequences vs Skills

| | MCP | Sequence | Skill |
|-|-----|----------|-------|
| **Granularity** | Single RPC | Batch of RPCs | Advisory guide |
| **Execution** | Agent calls one tool | `op do` runs all steps | Agent reasons between steps |
| **Determinism** | One call, one result | Linear, no branching | Adaptive, agent decides |
| **Invocation** | MCP tool call | `op do` or MCP | Agent reads, then acts |

All three are exposed via `op mcp`. The agent chooses the right level: **MCP** for single calls, **sequence** for fast deterministic batches, **skill** for adaptive reasoning.

<!-- don't modify the following section-->

## How to launch

```bash
op gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
op grpc://gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+stdio://gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+tcp://gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"en"}'
```

## Currently not supported .

unix, ws, ws, sse+rest 

```bash
op gabriel-greeting-go SayHello '{"name":"Maria","lang_code":"fr"}'  
```
<!-- don't modify preeceeding section -->
