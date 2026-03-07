# Organic Programming — Constitution

We work using a paradigm called **Organic Programming**.
It is Bio-inspired and Unix-inspired.

Unix is the philosophical inspiration — not the target platform.
Organic Programming targets **any platform where code runs and a contract
can be honored** — desktop (macOS, Linux, Windows), mobile (iOS, Android),
web (browser, WebSocket), embedded, voice, and beyond. The only
requirement is support for Protocol Buffers; transport is pluggable.

---

## Article 1 — The Holon

A **holon** is, at any scale, an independent, composable functional unit
that exposes four facets — from most universal to most specific:

1. **A contract** — a `.proto` file that defines an `rpc` interface with
   typed `message` structures. The contract is the primary facet: it makes
   the holon language-agnostic and universally interoperable — any
   language, any machine, locally or over the network, can invoke it.
   The contract is the schema, the interface, and the formal
   specification — all in one artifact. Beyond services, the proto also
   defines shared message types that other holons may reuse independently
   of any RPC call.
2. **External code** — a thin, minimal API in the implementation language
   (a Go package, a Python module, a Rust crate) that wraps the generated
   stubs. This facet enables same-language holons to compose directly
   in-process — calling proto-defined services without serialization or
   network overhead. It must remain as thin as possible: a convenience
   bridge, not a parallel API. It exposes only what the contract defines,
   nothing more.
3. **A CLI** (when possible or desirable) — a thin system I/O wrapper
   (`stdin`/`stdout`) over the generated client, enabling composition at
   the shell level. Some holons (browser SDKs, embedded libraries) may
   not have a CLI facet. The CLI is a bridge to the existing ecosystem:
   CI pipelines, shell scripts, human operators. Every CLI subcommand
   must mirror an RPC method — they are the same operation, one for the
   terminal, the other for the network. As the serve/dial mechanism
   gains traction, the CLI facet will become less prominent for
   inter-holon composition, but it remains valuable for tooling (e.g.
   `op`, `who`) and legacy integration.
4. **Unit tests** — the executable specification. Two levels:

   **External tests** (mandatory) — verify the exposed surface:
   - **Contract tests**: every RPC → nominal case + error case.
   - **Round-trip tests**: write → read → compare (serialization proof).
   - **Boundary tests**: invalid inputs, edge cases, not-found cases.

   **Internal tests** (best practice) — verify the machinery:
   - Unit tests for helpers, engines, adapters, and supporting code.
   - They are not required by this constitution, but they are strongly
     recommended. Good internal tests prevent regressions and document
     intent at the implementation level.

   For **deterministic holons** (`pure`, `stateful`), Test-Driven Development
   is the recommended workflow: proto → tests → implementation → refactor.
   For **probabilistic holons**, use property-based testing instead of exact
   value assertions. Testing strategies and helpers are provided per-SDK.

A holon is not limited to local code or CLI execution. Through its
contract, every holon can be invoked **locally or remotely** — the same
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
external code facet, bypassing serialization and network overhead.

---

## Article 2 — The Contract: Protocol Buffers

The `.proto` file is the **single source of truth** for a holon's public surface.

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

### gRPC is the default transport

gRPC is the default for distributed communication. Other transports are
permitted when the use case requires it, but the proto definition remains
the source of truth regardless of the transport layer.

### Introspection by default

Every holon's gRPC server **must** enable
[server reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md)
by default. A holon is self-describing: an agent or tool connecting to it
must be able to discover its services, methods, and message types at runtime
without needing the `.proto` file.

This is not a convenience — it is a requirement of the organic paradigm.
Opacity is the enemy of composition. A holon that hides its contract cannot
participate in the ecosystem.

For network-exposed deployments where the API surface should not be
discoverable, reflection can be disabled via the `--no-reflect` flag:

```
<holon> serve                 ← reflection ON (default)
<holon> serve --no-reflect    ← reflection OFF (production/exposed)
```

> [!NOTE]
> A comprehensive security model (authentication, authorization,
> transport encryption) is planned for a future revision of Organic
> Programming. For now, `--no-reflect` is the minimal production
> hardening mechanism.

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

1. **In-process** — via the external code facet. Same-language holons
   call each other directly through generated stubs, with no
   serialization and no network overhead.
2. **At runtime** — via serve & dial. Any holon can connect to any other
   regardless of language, using any supported transport (TCP, stdio,
   Unix socket, WebSocket, in-memory). This is the native composition
   mechanism of Organic Programming.
3. **At the shell** — via CLI piping. Standard Unix-style composition
   for legacy interop and scripting.

---

## Article 6 — The Universal Interface: Text Streams

*"Write programs to handle text streams, because that is a universal interface."*

1. **Text is the default for CLI.** Holon CLIs serialize to JSON, JSONLines, plain text, or any structured text format appropriate to the domain, for shell composability.
2. **Binary is the default for RPC.** Protobuf wire format for performance between holons.
3. **Stream is the default.** Unless told otherwise, every holon CLI must be capable of running as `echo "data" | ./holon`.

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
   stdin/stdout pipes on the same machine, or in-memory within the same
   process (`mem://`), the holon's code does not change. Only the
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
| `mem://` | In-process bufconn | Testing, composite holons, ~1µs latency |
| `ws://<host>:<port>[/path]` | WebSocket | Browser-accessible, NAT-friendly |
| `wss://<host>:<port>[/path]` | WebSocket over TLS | Encrypted WebSocket |

### Transport properties

A holon's transport choice determines two structural properties
(see [PROTOCOL.md §2.7](./PROTOCOL.md#27-transport-properties)):

- **Valence**: `stdio://` and `mem://` are **monovalent** (one
  connection per lifetime). `tcp://`, `unix://`, `ws://`, `wss://`
  are **multivalent** (N concurrent connections). A holon on a
  multivalent transport may still operate as monovalent.
- **Duplex**: all transports are **full-duplex** (both sides send
  concurrently) except `stdio://`, which is **simulated full-duplex**
  (two unidirectional pipes).

### Requirements

1. **gRPC reflection must be enabled** regardless of transport (see Article 2).
2. **The server must respond to `SIGTERM`** for graceful shutdown.
3. **The `--listen` flag accepts a single URI.** Multiple listeners are not
   required for v1.
4. **Backward compatibility**: `--port <port>` must be accepted as a shorthand
   for `--listen tcp://:<port>`.

### URI addressing in OP

OP uses the transport URI as a dispatch mechanism:

```
op grpc://localhost:9090 ListIdentities      TCP (existing server)
op grpc+stdio://who ListIdentities           stdio pipe (ephemeral)
op grpc+unix:///tmp/who.sock ListIdentities   Unix socket
op grpc+ws://host:8080/grpc ListIdentities    WebSocket
op run who:9090                               start holon on TCP
op run who --listen unix:///tmp/who.sock      start holon on Unix socket
```

The contract (`.proto`) defines WHAT a holon does; the transport URI defines
HOW the bytes flow. They are completely orthogonal.

---

## Article 12 — Standard Toolchain

> [!NOTE]
> The toolchain is under active development. See
> [organic-programming/holons/](./holons/) for current status.

Three holons form the canonical toolchain. **Always use them** when creating,
inspecting, or managing holons — they are the standard interface, not
ad hoc file manipulation.

| Tool | Purpose | Binary |
|------|---------|--------|
| **op** | Dispatch — discover, invoke, and run holons | `op` |
| **who** (Sophia Who?) | Identity — create, list, show, and pin holon identities | `who` |
| **atlas** (Marco Atlas) | Dependencies — init, add, remove, pull, verify, graph, vendor *(incubation)* | `atlas` |

### Creating a new holon (canonical workflow)

```bash
# 1. Create the identity (holon.yaml)
op grpc+stdio://who CreateIdentity '{
  "given_name": "my-holon",
  "family_name": "My Project",
  "motto": "What this holon does.",
  "composer": "Author Name",
  "clade": "DETERMINISTIC_PURE",
  "lang": "go",
  "output_dir": "./my-holon"
}'

# 2. Initialize the dependency file (holon.mod)
op grpc+stdio://atlas Init '{
  "directory": "./my-holon",
  "holon_path": "github.com/org/my-holon"
}'

# 3. Add dependencies
op grpc+stdio://atlas Add '{
  "directory": "./my-holon",
  "path": "github.com/organic-programming/go-holons",
  "version": "v0.2.0"
}'

# 4. Inspect the dependency graph
op grpc+stdio://atlas Graph '{"directory": "./my-holon"}'
```

After these steps, the holon directory contains:

```
my-holon/
├── holon.yaml   ← identity + operational manifest
└── holon.mod    ← dependency manifest
```

The agent then creates `protos/` with the `.proto` file, implements the
server in the idiomatic source directory, and adds tests. See
[CONVENTIONS.md](./CONVENTIONS.md) for per-language directory structure.

### Why this matters

Using `op`, `who`, and `atlas` — rather than manually creating files —
ensures identical structure, consistent UUIDs, and machine-verifiable
dependency metadata. An agent creating a holon MUST use this workflow.

---

## Article 13 — Distribution

*"A holon travels as a Git repository."*

A holon is distributable via Git in two forms:

1. **Source distribution** — source code + build recipe per supported
   target (OS, architecture). The consumer clones, builds, and runs.
   The build recipe is part of the holon: a `Makefile`, a `build.sh`,
   or the language's standard build command. Each supported target is
   explicit — there is no "build everywhere" magic.

2. **Binary distribution** — pre-built binary + holon files (`holon.yaml`,
   `.proto`, tests). The consumer clones and runs directly. The binary
   is committed or attached as a release artifact in the Git repository.

### What is always present

Regardless of the distribution form, a holon repository **must** contain:

| File | Why |
|------|-----|
| `holon.yaml` | Identity is never stripped. A holon without a name is not a holon. |
| `*.proto` | The contract is never stripped. A holon without a contract is a black box. |
| Tests | The specification is never stripped. A holon without tests is unverifiable. |

The contract and identity travel with the binary — they are not build
artifacts to be discarded after compilation. They are the holon's
permanent civil documents.

### Why Git

Git is the universal substrate. Every language, every platform, every CI
system speaks Git. Using Git as the distribution channel — rather than a
language-specific package manager — keeps holons language-agnostic and
infrastructure-free. ~~See `DEPENDENCIES.md` (in marco-atlas) for
the resolution strategy~~ currently not public.

---

## Article 14 — SDK-First Development

*"Use the SDK. It is not scaffolding — it is the foundation."*

Every holon **must** be developed using the SDK for its target language,
as much as possible. The SDK provides the canonical implementation of:

- **Transport** — TCP, stdio, Unix, WebSocket, in-memory.
- **Framing** — gRPC and Holon-RPC wire format.
- **Reflection** — server self-description at runtime.
- **Lifecycle** — `serve`, graceful shutdown, signal handling.

Reimplementing these from scratch defeats the purpose of the ecosystem.
A hand-rolled transport may work in isolation, but it risks
incompatibility with the fleet — subtle framing differences, missing
reflection, incorrect shutdown sequences.

### The rule

> **Use the SDK for everything the SDK provides.**
> Write custom code only for the holon's domain logic —
> the part that makes this holon unique.

### When no SDK exists

If no SDK exists yet for a target language, the holon author either:
Contributes a new SDK (see [sdk/](./sdk/)).

---

## Article 15 — Recipes

*"Combine SDKs, not codebases."*

A **recipe** is a cross-language assembly pattern — it shows how to
combine two or more language SDKs into a single application. Unlike a
language SDK (which you `import`), a recipe provides architecture docs,
build scripts, templates, and a working example.

Recipes address a structural gap: language SDKs enable holons to serve
and dial independently, but some applications require two holons — from
different stacks — to ship as one artifact. A Go backend embedded in a
Flutter app, a Rust engine driving a Swift UI, a Python model served
through a JavaScript dashboard: these are recipe territory.

A recipe contains:

1. **Architecture documentation** — how the two stacks connect
   (transport, lifecycle, shutdown).
2. **Build scripts** — compile both sides and bundle them.
3. **Templates** — reusable build phases and code generation scripts.
4. **A working example** — proof that the pattern works end to end.

See [recipes/](./recipes/) for available recipes.
