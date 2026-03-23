# Gabriel Greeting Rust

Rust implementation of a holon — a programmatic creature designed for the agentic age. Ported from the [Go reference implementation](../gabriel-greeting-go/). Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names (e.g. "Marie" in French, "マリア" in Japanese, "Мария" in Russian). Beyond the classic Hello World, this example demonstrates proto-based identity, layered facets, SDK-managed serving, and the transport cascade across all supported platforms.

This holon is built with the [Rust SDK](https://github.com/organic-programming/rust-holons) (`rust-holons`).

# A Proto + 4 facets is all you need.

## Protos

The holon's `api/v1/holon.proto` imports from two shared `_protos` directories (no copy, no symlink):

| Path | Scope | Content |
|------|-------|---------|
| `../../../_protos/` | Platform | System types (`holons/v1/manifest.proto`, `describe.proto`) |
| `../../_protos/` | Domain | Shared contract (`v1/greeting.proto` — service + messages) |
| `api/v1/holon.proto` | Local | Holon identity manifest + Rust-specific metadata |

## Facets

A holon exposes two kinds of facets:

### 4 X Innate facets

| Facet | Visibility | File | Role |
|-------|-----------|------|------|
| **Code API** | `api/` (public) | [public.rs](api/public.rs) | Pure functions consuming and returning protobuf types. No I/O, no server. The single source of truth for business logic — every other facet delegates here. |
| **CLI** | `api/` (public) | [cli.rs](api/cli.rs) | Parses CLI args, calls the Code API, formats output (text or JSON). Standalone binary entry point via `cmd/main.rs`. |
| **RPC** | `internal/` | [server.rs](internal/server.rs) | gRPC `GreetingService` implementation. Adapts proto request/response to the pure API. Exposed via `serve` to `op`, other holons, or any gRPC client. |
| **Tests** | `api/`, `internal/` | [public_test.rs](api/public_test.rs) | One test file per facet. Validates Code API, CLI args/output, and RPC contract independently. |

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

The `serve` sub-command is provided by the Rust SDK (`holons::serve`). It handles listener negotiation, `Describe`, optional `--reflect` debugging, and graceful shutdown — the holon only registers its gRPC service.

When a user runs:

```bash
op gabriel-greeting-rust SayHello '{"name":"Bob","lang_code":"en"}'
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

```text
Cargo.toml                                  Cargo package manifest.
build.rs                                    Protobuf generation and holon.proto validation.
src/lib.rs                                  Cargo wiring for the top-level facet files.
cmd/main.rs                                 Entry point — delegates to the CLI.
api/public.rs                               Code API — importable functions (say_hello, list_languages).
api/cli.rs                                  CLI facade — human / script interface to the Code API.
api/v1/holon.proto                          Identity manifest — proto-based holon descriptor.
internal/server.rs                          RPC server — gRPC implementation.
internal/greetings.rs                       Greeting data — 56 languages.
gen/rust/greeting/v1/greeting.v1.rs         Generated protobuf and gRPC code (do not edit).
```

`internal/` is not a facet — it is an **encapsulation practice**: private domain data and transport glue, never imported by external consumers.

# Acquired facets

Each acquired facet builds on the previous — from exposing individual RPCs, to composing them into batches, to guiding agents on when and why to use them.

## MCP — expose individual RPCs

`op mcp` turns any holon into an [MCP](https://modelcontextprotocol.io) tool server — with zero code changes.

```bash
op mcp gabriel-greeting-rust
```

`op` connects to the holon, introspects its gRPC contract via `Describe` (falling back to reflection only when needed), and exposes each RPC as an MCP tool over stdio. Proto comments (`@example`, `@required`) become the tool's JSON Schema descriptions and examples automatically.

Tools are fully qualified — `gabriel-greeting-rust.GreetingService.SayHello` — allowing multiple holons to be served from a single `op mcp` instance.

## Sequences — compose RPCs into deterministic batches

A sequence is a deterministic batch of steps declared in the proto manifest. No reasoning between steps — just execute.

```bash
op do gabriel-greeting-rust greeting-fr-ja-ru-en --name=""
op do gabriel-greeting-rust greeting-fr-ja-ru-en --name=Joseph
op do gabriel-greeting-rust greeting-fr-ja-ru-en --name="" --dry-run
op do gabriel-greeting-rust greeting-fr-ja-ru-en --name=Joseph --dry-run
```

The sequence declares a required `name` parameter. Passing `--name=""` is intentional: Gabriel then falls back to the localized default name for each language (`Marie`, `マリア`, `Мария`, `Mary`). Passing a real name such as `Joseph` forces the same name through every step.

## Skills — guide agents on *when* and *why* to act

A skill describes when and why to use a holon. Agents discover skills via `op inspect` or MCP and use them to decide which holon to call.

```bash
op inspect gabriel-greeting-rust
```

Gabriel declares one skill:

| Skill | When | Steps |
|-------|------|-------|
| **multilingual-greeter** | The user wants to greet someone in a specific language. | 1. ListLanguages → 2. Choose name + lang → 3. SayHello |

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
op gabriel-greeting-rust SayHello '{"name":"Maria","lang_code":"en"}'
op grpc://gabriel-greeting-rust SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+stdio://gabriel-greeting-rust SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+tcp://gabriel-greeting-rust SayHello '{"name":"Maria","lang_code":"en"}'
```

## Currently not supported .

unix, ws, ws, sse+rest

```bash
op gabriel-greeting-rust SayHello '{"name":"Maria","lang_code":"fr"}'
```

# How to compile manually the [holon.proto](api/v1/holon.proto)

```bash
cd examples/hello-world/gabriel-greeting-rust
protoc --proto_path=api --proto_path=../../_protos --proto_path=../../../_protos v1/holon.proto --descriptor_set_out=/dev/null
```
<!-- don't modify preeceeding section -->
