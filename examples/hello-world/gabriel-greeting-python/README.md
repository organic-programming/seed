# Gabriel Greeting Python

Python implementation of a holon — a programmatic creature designed for the agentic age. Ported from the [Go reference implementation](../gabriel-greeting-go/). Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names (e.g. "Marie" in French, "マリア" in Japanese, "Мария" in Russian). Beyond the classic Hello World, this example demonstrates proto-based identity, layered facets, SDK-managed serving, and the transport cascade across all supported platforms.

This holon is built with the [Python SDK](https://github.com/organic-programming/python-holons) (`python-holons`).

# A Proto + 4 facets is all you need.

## Protos

The holon's `api/v1/holon.proto` imports from two shared `_protos` directories (no copy, no symlink):

| Path | Scope | Content |
|------|-------|---------|
| `../../../_protos/` | Platform | System types (`holons/v1/manifest.proto`, `describe.proto`) |
| `../../_protos/` | Domain | Shared contract (`v1/greeting.proto` — service + messages) |
| `api/v1/holon.proto` | Local | Holon identity manifest + Python-specific metadata |

## Facets

A holon exposes two kinds of facets:

### 4 X Innate facets

| Facet | Visibility | File | Role |
|-------|-----------|------|------|
| **Code API** | `api/` (public) | [public.py](api/public.py) | Pure functions consuming and returning protobuf types. No I/O, no server. The single source of truth for business logic — every other facet delegates here. |
| **CLI** | `api/` (public) | [cli.py](api/cli.py) | Parses CLI args, calls the Code API, formats output (text or JSON). Standalone entry point via `cmd/main.py`. |
| **RPC** | `_internal/` | [server.py](_internal/server.py) | gRPC `GreetingServiceServicer` implementation. Adapts proto request/response to the pure API. Exposed via `serve` to `op`, other holons, or any gRPC client. |
| **Tests** | `api/`, `_internal/` | [public_test.py](api/public_test.py) | One test file per facet. Validates Code API, CLI args/output, and RPC contract independently. |

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

The `serve` sub-command is a rich feature provided by the Python SDK (`holons.serve`). It handles listener negotiation, `Describe`, optional `--reflect` debugging, and graceful shutdown — the holon only registers its gRPC service.

When a user runs:

```bash
op gabriel-greeting-python SayHello '{"name":"Alice","lang_code":"en"}'
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

```text
pyproject.toml                              Python project metadata and runtime dependencies.
support.py                                  Shared import-path bootstrap for SDK + generated code.
cmd/main.py                                 Entry point — delegates to the CLI.
api/public.py                               Code API — importable functions (say_hello, list_languages).
api/cli.py                                  CLI facade — human / script interface to the Code API.
api/v1/holon.proto                          Identity manifest — proto-based holon descriptor.
_internal/server.py                         RPC server — gRPC implementation.
_internal/greetings.py                      Greeting data — 56 languages.
gen/python/greeting/v1/greeting_pb2.py      Generated protobuf code (do not edit).
gen/python/greeting/v1/greeting_pb2_grpc.py Generated gRPC code (do not edit).
scripts/generate_proto.sh                   Deterministic protobuf regeneration helper.
```

`_internal/` is the Python equivalent of a private implementation module: domain data and transport glue live there, not in the public API surface.

# Acquired facets

Each acquired facet builds on the previous — from exposing individual RPCs, to composing them into batches, to guiding agents on when and why to use them.

## MCP — expose individual RPCs

`op mcp` turns any holon into an [MCP](https://modelcontextprotocol.io) tool server — with zero code changes.

```bash
op mcp gabriel-greeting-python
```

`op` connects to the holon, introspects its gRPC contract via `Describe` (falling back to reflection only when needed), and exposes each RPC as an MCP tool over stdio. Proto comments (`@example`, `@required`) become the tool's JSON Schema descriptions and examples automatically.

Tools are fully qualified — `gabriel-greeting-python.GreetingService.SayHello` — allowing multiple holons to be served from a single `op mcp` instance.

## Sequences — compose RPCs into deterministic batches

A sequence is a deterministic batch of steps declared in the proto manifest. No reasoning between steps — just execute.

```bash
op do gabriel-greeting-python greeting-fr-ja-ru-en --name=""
op do gabriel-greeting-python greeting-fr-ja-ru-en --name=Joseph
op do gabriel-greeting-python greeting-fr-ja-ru-en --name="" --dry-run
op do gabriel-greeting-python greeting-fr-ja-ru-en --name=Joseph --dry-run
```

The sequence declares a required `name` parameter. Passing `--name=""` is intentional: Gabriel then falls back to the localized default name for each language (`Marie`, `マリア`, `Мария`, `Mary`). Passing a real name such as `Joseph` forces the same name through every step.

## Skills — guide agents on *when* and *why* to act

A skill describes when and why to use a holon. Agents discover skills via `op inspect` or MCP and use them to decide which holon to call.

```bash
op inspect gabriel-greeting-python
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
op gabriel-greeting-python SayHello '{"name":"Maria","lang_code":"en"}'
op grpc://gabriel-greeting-python SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+stdio://gabriel-greeting-python SayHello '{"name":"Maria","lang_code":"en"}'
op grpc+tcp://gabriel-greeting-python SayHello '{"name":"Maria","lang_code":"en"}'
```

## Currently not supported .

ws, sse+rest

```bash
op gabriel-greeting-python SayHello '{"name":"Maria","lang_code":"fr"}'
```

# How to compile manually the [holon.proto](api/v1/holon.proto)

```bash
cd examples/hello-world/gabriel-greeting-python
python3 -m grpc_tools.protoc -I api -I ../../_protos -I ../../../_protos --descriptor_set_out="$(mktemp)" v1/holon.proto
```
<!-- don't modify preeceeding section -->
