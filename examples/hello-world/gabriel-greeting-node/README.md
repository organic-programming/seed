# Gabriel Greeting Node

Reference implementation of a Node.js holon — a programmatic creature designed for the agentic age. Strict layered architecture, fully tested.

Gabriel is a multilingual greeting service. It exposes two RPCs — `SayHello` and `ListLanguages` — over a shared protobuf contract. The greeting table covers 56 languages with localized templates and culturally appropriate default names (for example `Marie` in French, `マリア` in Japanese, and `Мария` in Russian). This example demonstrates proto-based identity, layered facets, SDK-managed serving, and a fully symmetric surface across code, CLI, RPC, and tests.

This holon is built with the [Node SDK](../../../sdk/js-holons) (`@organic-programming/holons`).

# A Proto + 4 facets is all you need.

## Protos

The holon's `api/v1/holon.proto` imports from two shared `_protos` directories (no copy, no symlink):

| Path | Scope | Content |
|------|-------|---------|
| `../../../_protos/` | Platform | System types (`holons/v1/manifest.proto`) |
| `../../_protos/` | Domain | Shared contract (`v1/greeting.proto` — service + messages) |
| `api/v1/holon.proto` | Local | Holon identity manifest + Node-specific metadata |

## Facets

### 4 X Innate facets

| Facet | Visibility | File | Role |
|-------|-----------|------|------|
| **Code API** | `api/` (public) | [public.js](api/public.js) | Pure functions consuming and returning protobuf types. No I/O, no server. The single source of truth for business logic. |
| **CLI** | `api/` (public) | [cli.js](api/cli.js) | Parses command-line args, calls the Code API, and formats text or JSON output. |
| **RPC** | `_internal/` | [server.js](_internal/server.js) | gRPC `GreetingService` implementation. Adapts transport details and delegates to the Code API. |
| **Tests** | `api/`, `_internal/` | [*_test.js](api/public_test.js) | One test file per facet validating Code API, CLI, and RPC independently. |

### 3 X Acquired facets

| Facet | Provided by | Role |
|-------|------------|------|
| **MCP** | `op mcp` | Exposes RPCs as MCP tools for LLM clients. |
| **Skills** | manifest | Declared holon capabilities discoverable by agents and orchestrators. |
| **Sequences** | manifest | Multi-step workflows composed from the holon's RPCs. |

## Structure

```text
cmd/main.js                    Entry point — delegates to the CLI.
api/public.js                  Code API — importable functions.
api/cli.js                     CLI facade — human / script interface.
api/v1/holon.proto             Identity manifest — proto-based holon descriptor.
_internal/server.js            RPC server — gRPC implementation.
_internal/greetings.js         Greeting data — 56 languages with default names.
gen/node/greeting/v1/          Generated protobuf code (do not edit).
scripts/generate_proto.js      Regenerates protobuf code and validates holon.proto.
```

`_internal/` is a privacy convention: server glue and greeting data stay out of the public import surface.

## How to launch

```bash
npm install
npm run generate-proto
node cmd/main.js version
node cmd/main.js listLanguages --format json
node cmd/main.js sayHello Alice fr
node cmd/main.js serve --port 9090
```
