# Organic Programming — Constitution

We work using a paradigm called **Organic Programming**.

Organic Programming targets **any platform where code runs and a contract
can be honored** — desktop (macOS, Linux, Windows), mobile (iOS, Android),
web (browser, WebSocket), embedded, voice, and beyond. The only
requirement is support for Protocol Buffers; transport is pluggable.

---

## Article 1 — The Holon

A **holon** is, at any scale, an independent, composable functional unit
built for **coaccessibility** (**COAX**) — equally accessible to humans
and machines through the same structural contracts.


### The proto — the center


The `.proto` is the **gravitational center**from which all facets derive. 
It defines an`rpc` interface with typed `message` structures, making the holon
language-agnostic and universally interoperable — any language, any
machine, locally or over the network, can invoke it. The proto is the
schema, the interface, and the formal specification — all in one
artifact. Beyond services, it also defines shared message types that
other holons may reuse independently of any RPC call.

### Innate facets — code the developer writes

Four facets radiate from the proto. The developer implements them;
each one delegates to or wraps the generated stubs:

1. **Code API** — pure functions consuming and returning protobuf types.
   No I/O, no server. The single source of truth for business logic —
   every other facet delegates here. Same-language holons can import it
   directly for in-process composition without serialization or network
   overhead. It must remain as thin as possible: a convenience bridge,
   not a parallel API. It exposes only what the contract defines,
   nothing more.
2. **CLI** — parses arguments, calls the Code API, formats output
   (text or JSON). A `stdin`/`stdout` bridge to scripts, CI, and human
   operators. Some holons (browser SDKs, embedded libraries) may not
   have a CLI facet. Every CLI subcommand must mirror an RPC method —
   they are the same operation, one for the terminal, the other for
   the network. As the serve/dial mechanism gains traction, the CLI
   facet will become less prominent for inter-holon composition, but
   it remains valuable for tooling (e.g. `op`, `who`) and legacy
   integration.
3. **RPC** — gRPC service implementation. Adapts proto request/response
   to internal logic. Exposed via the SDK-managed `serve` sub-command
   to `op`, other holons, or any gRPC client. The holon registers its
   service; the SDK handles listener negotiation, reflection, and
   graceful shutdown.
4. **Tests** — the executable specification. Two levels:

   **External tests** (mandatory) — verify the exposed surface:
   - **Contract tests**: every RPC → nominal case + error case.
   - **Round-trip tests**: write → read → compare (serialization proof).
   - **Boundary tests**: invalid inputs, edge cases, not-found cases.

   **Internal tests** (best practice) — verify the machinery:
   - Unit tests for helpers, engines, adapters, and supporting code.
   - They are not required by this constitution, but they are strongly
     recommended. Good internal tests prevent regressions and document
     intent at the implementation level.

   One test file per facet is a standard. TDD is the recommended
   workflow for **deterministic holons** (`pure`, `stateful`):
   proto → tests → implementation → refactor.
   For **probabilistic holons**, use property-based testing instead of
   exact value assertions. Testing strategies and helpers are provided
   per-SDK.

### Surface symmetry — the golden rule

The four innate facets must cover **the same external surface**:

> **Code API surface = CLI surface = RPC surface = Test surface**

Every operation the contract exposes is reachable through the Code API,
invocable from the CLI, served over RPC, and verified by tests — no
more, no less. If a function exists in one facet but is absent from
another, the holon is incomplete.

The manifest's `contract.rpcs` list must exhaustively match the service
definition. It is the declared external surface, so it must line up with
the Code API, CLI, RPC server, and tests exactly.

Two CLI affordances are exempt: `serve` (circular — it bootstraps the
RPC facet) and `help` (purely human-facing, register-specific). `version`
is not exempt — the SDK derives it from the manifest and surfaces it
across all facets automatically (built-in CLI subcommand + `Describe`
response).

This symmetry is the primary design constraint: the external surface must
be **as small as possible**. The internal volume behind it — helpers,
engines, data structures — can be arbitrarily complex and composed, but
the skin that the outside world touches remains thin, uniform, and fully
tested.

### Acquired facets — traits gained through `op`

These facets emerge from the proto contract and manifest. The holon
writes **no code** for them — `op` derives them automatically:

| Facet | Provided by | Role |
|-------|------------|------|
| **MCP** | `op mcp` | Exposes RPCs as MCP tools for LLM clients. Proto comments become JSON Schema. |
| **Skills** | manifest | Declared holon capabilities discoverable by agents and orchestrators. |
| **Sequences** | manifest | Multi-step workflows composed from the holon's RPCs. |

Each acquired facet builds on the previous — from exposing individual
RPCs, to composing them into batches, to guiding agents on when and
why to use them. See the
[hello-world example](./examples/hello-world/gabriel-greeting-go/README.md)
for concrete details.

### Exposure

Facets split into two contexts:

| Context | Innate | Acquired |
|---------|--------|----------|
| **Dev time** | Code API, Tests | — |
| **Runtime** | CLI, RPC | MCP, Skills, Sequences |

A holon is not limited to local code or CLI execution. Through its
proto, every holon can be invoked **locally or remotely** — the same
contract serves all modes. This is what makes holons composable across
machines and networks.

Every CLI holon exposes two fundamental mechanics: **serve** (listen for
incoming connections) and **dial** (connect to another holon). A holon
can act as a server, a client, or both. This symmetry is what makes
holons composable at any level — in-process, local, or networked — any
holon can reach any other.
See [Article 11](#article-11--the-serve--dial-convention) for the full
serve & dial convention.

**The unit of decomposition is the domain** (a bounded context
with its own data model and rules)**, not the subcommand or the RPC.**
A holon's CLI subcommands and its gRPC methods cover the
same operations — one for the terminal, the other for the network.
They belong together in a single holon as long as they share the same
domain model. When a subset grows its own data and its own rules, it
becomes its own holon.

The implementation is hidden behind the generated interface.
Encapsulation is mandatory: expose the minimum surface.

### External surface vs. internal surface

A holon's code has two distinct layers:

- **External surface** — a thin, minimal API in the implementation
  language that wraps the generated stubs. It exposes only what the
  contract defines, nothing more. Same-language holons can import it
  directly; cross-language holons invoke the contract. If it is not
  defined in the contract, it is not external.
- **Internal surface** — supporting code (helpers, engines, data
  structures, adapters) that may have public interfaces at the language
  level — Go exported types, Python public classes — but is **not part
  of the external API** and is **not reachable through CLI or RPC**.
  This code exists for composability *within* the holon: between its own
  packages, for testability, for clean separation of concerns. It is
  architecturally private even when syntactically public.

The distinction matters: an agent or another holon connecting to this
holon sees only the external surface. The internal surface is an
implementation detail — it can change freely without breaking any
consumer, as long as the contract holds.

Holons are language-agnostic *through their contract*. The `.proto` file
is the universal bridge — code generation produces native stubs in any
target language. Same-language holons can also compose directly via the
Code API facet, bypassing serialization and network overhead.

---

## Article 2 — The Contract: Protocol Buffers

The `.proto` file is the **single source of truth** for a holon's public
surface — the gravitational center described in Article 1. Every innate
facet delegates to the generated stubs; every acquired facet is derived
from the proto without additional code.

### Document the contract

The `.proto` file is not only the interface — it is the root reference
documentation. Every `service`, `rpc`, `message`, `enum`, and field
**must** carry proto comments explaining its purpose. A contract without
comments is incomplete.

### Every public function is an `rpc`

A function like `GetTime(timeZone) → Time` is modeled as:

```protobuf
syntax = "proto3";
package clock;

service Clock {
  rpc GetTime(TimeZone) returns (Time);
}

message TimeZone {
  string name = 1;
}

message Time {
  int64  unix = 1;
  string formatted = 2;
}
```

Even functions called locally are declared as `rpc` methods. This ensures
the contract is exhaustive: if it's not in the proto, it's not public.

### Transport

gRPC over a pluggable transport is the communication mechanism.
The holon's code is transport-indifferent — the SDK and `op` negotiate
the best available transport automatically via a platform-specific
connect cascade (see
[Article 11](#article-11--the-serve--dial-convention)).
The proto definition remains the source of truth regardless of the
transport layer.

### Self-documentation via `Describe`

A holon is self-describing: an agent or tool connecting to it must be
able to discover its services, methods, and message types at runtime
without needing the `.proto` file.

This is not a convenience — it is a requirement of the organic paradigm.
Opacity is the enemy of composition. A holon that hides its contract
cannot participate in the ecosystem.

Every holon **must** register the SDK's built-in `HolonMeta` service,
which provides a `Describe` RPC. The full service definition lives in
[`_protos/holons/v1/describe.proto`](./_protos/holons/v1/describe.proto):

```
holon.Describe() → manifest, services[], methods[], field docs
```

`Describe` returns the holon's full manifest and its API catalog —
method names, purpose, input/output types with field descriptions, enum
definitions — as a typed protobuf response. It provides:

- **Selective exposure** — the holon controls exactly what it documents.
  Internal or debug RPCs can be omitted.
- **Full type information** — enough for a caller to dynamically
  construct and send requests without compiled stubs.
- **Human-readable descriptions** — curated English, not raw metadata.
- **Examples** — concrete request examples from `@example` tags
  in proto comments, so any caller can see what a valid call looks like.
- **Semantic required/optional** — `@required` tags fill the gap where
  proto3 has no wire-level required keyword.

```go
// ── Usage (Go) ──────────────────────────────────────────
//
// func main() {
//     server := grpc.NewServer()
//
//     // Register the business implementation (written by the developer)
//     greetingv1.RegisterGreetingServiceServer(server, &myGreetingImpl{})
//
//     // Auto-register HolonMeta (single SDK call)
//     // The SDK parses local .proto files and extracts the manifest
//     describe.Register(server, ".", "v1/holon.proto")
//     //                         ↑ proto root  ↑ file carrying the manifest
//
//     server.Serve(listener)
// }
```

For LLM and MCP integration, `op` provides external bridges that read
the `.proto` files directly — see `op inspect`, `op mcp`, `op tools`
in [OP.md §15](./OP.md).

The SDK auto-registers `HolonMeta` when using the standard `serve`
runner. The proto documentation is parsed from the holon's proto
directory lazily at runtime.

> [!NOTE]
> gRPC server reflection **may** be enabled as a development
> convenience for tools like `grpcurl`. It is not required by
> this constitution — `Describe` is the canonical introspection
> mechanism.

### Code generation is the bridge

The generated code provides:

- **Language-native stubs** — public functions in Go, C++, Python, JS, etc.
- **Client and server skeletons** — for distributed deployment.
- **Serialization** — binary (protobuf) or text (JSON) for interoperability.

> **Agent directive**: never write public API code by hand.
> The proto defines the contract; `protoc` generates the interface.
> Implementation code lives behind the generated stubs.

---

## Article 3 — Adoption: Any CLI Can Become a Holon

*"Don't rewrite the world — adopt it."*

In practice, wrapping a CLI — handling process execution, output
parsing, streaming, and lifecycle — can become complex. The
`go-cli-holonization` SDK provides Go tooling to simplify this process.
The longer-term vision is an automated holonizer capable of generating
wrappers from a CLI's documentation, but the SDK is the pragmatic
first step.

Any existing command-line tool can become a holon. The rule is simple:

**Each subcommand is a holon.**

### The adoption process

1. **Write a `.proto`** that describes the subcommand's interface — its inputs
   as request messages, its outputs as response messages.
2. **Implement a thin wrapper** that translates between the generated gRPC
   interface and the original binary's flags, stdin, and stdout.
3. **The original tool is untouched.** The wrapper delegates to it.

### Example: adopting `ffprobe`

`ffprobe` inspects media files. As a holon:

```protobuf
service FFProbe {
  rpc Probe(ProbeRequest) returns (MediaInfo);
}

message ProbeRequest {
  string path = 1;
}

message MediaInfo {
  string   format = 1;
  double   duration = 2;
  repeated Stream streams = 3;
}

message Stream {
  string codec = 1;
  string type = 2;   // "video", "audio", "subtitle"
  int32  width = 3;
  int32  height = 4;
}
```

The implementation is a wrapper that calls `ffprobe -v quiet -print_format json`
and maps the JSON output to the `MediaInfo` message.

### Example: adopting `git`

`git` is itself a holon — and each of its subcommands is also a holon.
Holons are fractal:

| Subcommand | Holon |
|---|---|
| `git commit` | `rpc Commit(CommitRequest) returns (CommitResponse)` |
| `git log` | `rpc Log(LogRequest) returns (stream LogEntry)` |
| `git diff` | `rpc Diff(DiffRequest) returns (DiffResponse)` |
| `git` (whole) | A composite holon that orchestrates the above |

Each subcommand does one thing well (Article 4). The proto describes it.
The wrapper delegates to the `git` binary.

### Why this matters

The command-line ecosystem across all platforms — `ffmpeg`, `git`, `curl`,
`jq`, `grep`, `imagemagick`, `PowerShell` cmdlets — can join the organic
ecosystem **without rewriting a single line** of their source code.
Adoption is additive, not invasive.

---

## Article 4 — Extreme Modularity

*"Make each holon do one thing well."*

To do a new job:

1. Compose existing holons.
2. Build a fresh holon rather than complicate old ones by adding new "features."

---

## Article 5 — Composition

*"Expect the output of every holon to become the input to another, as yet unknown, holon."*

Holons are composable at three levels:

1. **In-process** — via the Code API facet. Same-language holons
   call each other directly through generated stubs, with no
   serialization and no network overhead.
2. **At runtime** — via serve & dial. Any holon can connect to any other
   regardless of language, using any supported transport (TCP, stdio,
   Unix socket, WebSocket). This is the native composition
   mechanism of Organic Programming.
3. **At the shell** — via CLI piping. Standard Unix-style composition
   for legacy interop and scripting.

---

---

## Article 7 — Separation of Concerns: Mechanism vs. Policy

*"Separate mechanism from policy."*

- **Mechanism** (the engine): *"How do I process this data?"* — the agent or holon.
- **Policy** (the rules): *"What rules apply?"* — the specification or configuration file.

Do not hardcode business rules into agent prompts, code logic, or holon internals.
Keep the holon as a pure engine that reads policy from external configuration.
YAML is the conventional format for policy files; alternatives are
acceptable when justified by the domain.

---

## Article 8 — Guardrails

1. **One job rule**: if a holon's description requires "and" to explain what it does, stop and propose splitting it into two holons.
2. **No premature implementation**: when the specification is incomplete or ambiguous, ask — do not guess.
3. **Progressive adoption**: Organic Programming is adopted incrementally in existing codebases. Do not refactor code that is not part of the current task.
4. **Specification first**: the `.proto` file is the source of truth for *what* to build. This constitution is the source of truth for *how* to build it.

---

## Article 9 — Mimesis

*"Invent only what is truly new; for everything else, imitate what works."*

Organic Programming embraces **memetics** — the replication and adaptation
of proven patterns across contexts. When a mechanism has survived natural
selection in another ecosystem, replicate it faithfully, then adapt it to
the holon substrate.



---

## Article 10 — Navigation and Provenance

Each project that adopts Organic Programming should provide:

1. A **project-level `AGENT.md`** that references this constitution and provides project-specific context.
2. An **`INDEX.md`** per folder that lists all documents with a one-line purpose, so that agents can navigate efficiently without opening every file.

Agents must read `INDEX.md` upon entering any folder.

---

## Article 11 — The Serve & Dial Convention

*"Every holon must be reachable."*

A holon communicates through two fundamental mechanics — think of a
telephone:

- **Serve** — you pick up and wait for calls. You make yourself
  available at an address. The holon starts a gRPC server and listens
  for incoming RPC calls on a transport (TCP, stdio, Unix socket,
  WebSocket, in-memory…).
- **Dial** — you call someone else. You connect to a holon that is
  serving. The holon becomes a client and can invoke the RPC methods
  defined in the other holon's contract.

Three properties make this mechanism powerful:

1. **Symmetry** — a holon can serve and dial at the same time: server
   and client simultaneously.
2. **Transport indifference** — whether over TCP on the network, via
   stdin/stdout pipes on the same machine, or through a WebSocket
   tunnel, the holon's code does not change. Only the
   address changes.
3. **Composition** — this is the fundamental mechanism that lets holons
   plug into each other at runtime. Two holons from different technology
   stacks (Go + Dart, Rust + Python) form a coherent whole simply
   because one serves and the other dials — like Lego bricks whose
   studs (serve) and holes (dial) interlock regardless of color or size.

Every holon binary **must** implement the `serve` command to start its
gRPC facet:

```
<holon> serve --listen <transport-URI>
```

If `--listen` is omitted, the default transport is `tcp://:9090`.

### Mandatory transports

| URI scheme | Transport | Description |
|-----------|-----------|-------------|
| `tcp://<host>:<port>` | TCP socket | Network transport, classic gRPC |
| `stdio://` | stdin/stdout pipe | Zero overhead, pipe-based IPC |

`stdio://` is mandatory because it is the **universal baseline** — every
OS, every language, every platform has stdin/stdout. A Swift holon on iOS,
a Rust holon on a Raspberry Pi, a Python holon in a container — they all
have stdio.

### Optional transports

| URI scheme | Transport | Description |
|-----------|-----------|-------------|
| `unix://<path>` | Unix domain socket | Local, fast, no port conflicts |
| `ws://<host>:<port>[/path]` | WebSocket | Browser-accessible, NAT-friendly |
| `wss://<host>:<port>[/path]` | WebSocket over TLS | Encrypted WebSocket |

### Transport properties

A holon's transport choice determines two structural properties
(see [HOLON_COMMUNICATION_PROTOCOL.md §2.7](./HOLON_COMMUNICATION_PROTOCOL.md#27-transport-properties)):

- **Valence**: `stdio://` is **monovalent** (one
  connection per lifetime). `tcp://`, `unix://`, `ws://`, `wss://`
  are **multivalent** (N concurrent connections). A holon on a
  multivalent transport may still operate as monovalent.
- **Duplex**: all transports are **full-duplex** (both sides send
  concurrently) except `stdio://`, which is **simulated full-duplex**
  (two unidirectional pipes).

### Requirements

1. **`HolonMeta.Describe` must be registered** regardless of transport (see Article 2).
2. **The server must respond to `SIGTERM`** for graceful shutdown.
3. **The `--listen` flag accepts a single URI.** Multiple listeners are not
   required for v1.
4. **Backward compatibility**: `--port <port>` must be accepted as a shorthand
   for `--listen tcp://:<port>`.

### URI addressing in OP

OP uses the transport URI as a dispatch mechanism:

```
op grpc://localhost:9090 SayHello '{}'                         TCP (existing server)
op grpc+stdio://gabriel-greeting-go SayHello '{}'              stdio pipe (ephemeral)
op grpc+unix:///tmp/gabriel.sock SayHello '{}'                 Unix socket
op grpc+ws://host:8080/grpc SayHello '{}'                      WebSocket
op run gabriel-greeting-go:9090                                start holon on TCP
op run gabriel-greeting-go --listen unix:///tmp/gabriel.sock    start holon on Unix socket
```

The contract (`.proto`) defines WHAT a holon does; the transport URI defines
HOW the bytes flow. They are completely orthogonal.

### Connect — Name-Based Resolution

`Dial` requires a transport address. `Connect` requires only a holon
name — the SDK handles the rest.

Every SDK **should** provide a `connect` primitive that composes
three lower-level operations:

1. **Discover** — resolve a holon slug to a filesystem location
   (scan `api/v1/holon.proto` manifests in known roots).
2. **Start** (if needed) — find the built binary, launch it with
   `serve --listen stdio://`, and wire the parent's pipes directly.
3. **Dial** — open a gRPC client channel to the running holon.

```
connect("rob-go")          →  discover → start → dial → ready gRPC channel
connect("localhost:9090")  →  dial directly (host:port bypass)
```

#### Transport cascade

When `connect` starts a holon (ephemeral mode), it uses a transport
cascade ordered by efficiency:

| Priority | Transport | When |
|:--------:|-----------|------|
| **1** | `stdio://` | Default — parent owns child's pipes, zero overhead |
| **2** | `tcp://127.0.0.1:0` | Explicit override via `ConnectOptions` |

`stdio://` is the default because it is the universal baseline — every
OS, every language has stdin/stdout. The pipe is already wired at spawn
time: no port allocation, no loopback TCP overhead, no race conditions.

When `connect` finds an **already-running** holon (via port/socket file),
there is no cascade — it dials whatever address the file advertises.

`Disconnect` reverses the process: close the channel and, if the
SDK started the holon (ephemeral mode), stop it.

Connect is a **SDK facility**, not a protocol extension — the wire
format is unchanged. It bridges the gap between "I know the address"
(dial) and "I know the name" (connect), enabling autonomous
holon-to-holon composition without `op` as an intermediary.

---

## Article 12 — Standard Toolchain

> [!NOTE]
> The toolchain is under active development. See
> [organic-programming/holons/](./holons/) for current status.

The `op` binary orchestrator forms the canonical toolchain. **Always use
it** when creating, inspecting, managing holons, or handling
dependencies — it provides the standard interface, fully integrating
identity and dependency management, rather than relying on ad hoc file
manipulation.

| Tool | Purpose | Binary |
|------|---------|--------|
| **op** | Dispatch, Identity, Dependencies — discover, invoke, init, and run holons | `op` |

### Creating a new holon (canonical workflow)

The identity and operational manifest live in a `.proto` file carrying
`option (holons.v1.manifest)`. `op new` scaffolds this file and the
surrounding directory structure:

```bash
# 1. Create the holon scaffold (proto manifest)
op new --json '{
  "given_name": "my-holon",
  "family_name": "My Project",
  "motto": "What this holon does.",
  "composer": "Author Name",
  "clade": "deterministic/pure",
  "lang": "go",
  "output_dir": "./my-holon"
}'

# 2. Initialize the dependency file (holon.mod)
cd ./my-holon
op mod init github.com/org/my-holon

# 3. Add dependencies
op mod add github.com/organic-programming/go-holons v0.2.0

# 4. Inspect the dependency graph
op mod graph
```

After these steps, the holon directory contains:

```
my-holon/
├── api/v1/holon.proto   ← identity + operational manifest (proto)
└── holon.mod            ← dependency manifest
```

The agent then implements the Code API, CLI, RPC server, and tests.
See [CONVENTIONS.md](./CONVENTIONS.md) for per-language directory
structure and the
[hello-world example](./examples/hello-world/gabriel-greeting-go/)
for a complete reference.

### Why this matters

Using `op` — rather than manually creating files —
ensures identical structure, consistent UUIDs, and machine-verifiable
dependency metadata. An agent creating a holon MUST use this workflow.

---

## Article 13 — Distribution

*"A holon travels as a `.holon` package."*

The `.holon` package is the universal distribution unit. It wraps
source, binaries, and metadata into one structure that `op` understands
at every stage: development, build, cache, install, and runtime.

See [HOLON_PACKAGE.md](./HOLON_PACKAGE.md) for the full specification.

### What is always present

Regardless of distribution mode, a holon **must** carry:

| File | Why |
|------|-----|
| `*.proto` (with manifest) | Identity and contract are never stripped. A holon without a name or a contract is not a holon. |
| Tests | The specification is never stripped. A holon without tests is unverifiable. |

The contract and identity travel with the binary — they are not build
artifacts to be discarded after compilation. They are the holon's
permanent civil documents.

### Why Git

Git is the universal substrate. Every language, every platform, every CI
system speaks Git. Using Git as the distribution channel — rather than a
language-specific package manager — keeps holons language-agnostic and
infrastructure-free.

---

## Article 14 — SDK-First Development

*"Use the SDK. It is not scaffolding — it is the foundation."*

Every holon **must** be developed using the SDK for its target language,
as much as possible. The SDK provides the canonical implementation of:

- **Transport** — TCP, stdio, Unix, WebSocket.
- **Framing** — gRPC and Holon-RPC wire format.
- **Reflection** — server self-description at runtime.
- **Lifecycle** — `serve`, graceful shutdown, signal handling.

Reimplementing these from scratch defeats the purpose of the ecosystem.
A hand-rolled transport may work in isolation, but it risks
incompatibility with the fleet — subtle framing differences, missing
self-documentation, incorrect shutdown sequences.

### The rule

> **Use the SDK for everything the SDK provides.**
> Write custom code only for the holon's domain logic —
> the part that makes this holon unique.

### When no SDK exists

If no SDK exists yet for a target language, the holon author either:
Contributes a new SDK (see [sdk/](./sdk/)).
