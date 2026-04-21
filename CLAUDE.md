# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

The **seed** of Organic Programming — a bio-inspired paradigm where every functional unit (a *holon*) is defined by a `.proto` at its gravitational center, radiates four innate facets (Code API, CLI, RPC, Tests), and is reachable identically by humans and machines (the **COAX** principle). This repo holds the spec, the toolchain, the SDKs, and 13 reference examples — not a single application.

**This repo is work-in-progress elaboration, not a stable spec.** `op` (the `grace-op` holon) is the instrument building Organic Programming, which in turn shapes `op` back — a mutual-construction process advancing by trial and error. Root-level specs (`CONSTITUTION.md`, `CONVENTIONS.md`, `PROTO.md`, `PACKAGE.md`, `DISCOVERY.md`, `COMMUNICATION.md`, `COAX.md`, `TDD.md` — mapped by `INDEX.md`) describe the intended shape, but they were authored by the composer and multiple agents at different moments. Docs drift from each other and from the code; sometimes a document leads the implementation, sometimes the code is ahead. The `.proto` files sit structurally at the center of every holon (identity, contract, manifest), without being universally authoritative. Contradictions are expected, not bugs. An agent working here participates in the elaboration. When stakes matter, **cross-check doc vs code vs proto**; when still ambiguous, **ask the composer** — do not guess and do not paper over conflicts.

## Layout

- `holons/grace-op/` — `op`, the orchestrator CLI. Canonical `holons/v1` protos live under `_protos/` and are embedded into the binary.
- `holons/clem-ader/` — `ader`, the integration proof engine.
- `sdk/<lang>-holons/` — 13 language SDKs (`sdk/go-holons` is the reference, full Dial/Serve on every transport).
- `examples/` — shared contracts implemented per-language (reference hello-world: `gabriel-greeting-go`).
- `ader/` — live proof root: `catalogues/<name>/{ader.yaml,checks.yaml,suites/,reports/,archives/}` + `bouquets/`.
- `organism_kits/` — scaffolding kits for composite apps (Flutter, SwiftUI).

Go modules are unified by `go.work` at the root; every member `replace`s `github.com/organic-programming/go-holons => ../../sdk/go-holons` so local SDK edits propagate immediately. Go 1.26.1 in `go.work`, 1.25.1 in module files.

## Two first-class tools

- **`op`** — orchestrator for every lifecycle op on a holon (build, test, check, clean, install, run, inspect, do, mcp, tools, new, mod, serve, discover). Prefer `op` over ad-hoc shell.
- **`ader`** — integration proof engine. `go test` / GitLab CI cover the unit level; `ader` covers the integration level. Complementary, not alternatives.

## Using `op`

**The canonical argument is the slug**, resolved through the discovery layers in `DISCOVERY.md` (`siblings`, `cwd`, `source`, `built`, `installed`, `cached`):

```bash
op build   gabriel-greeting-go       # discovery resolves the slug
op test    gabriel-greeting-go
op check   gabriel-greeting-go       # manifest + prereqs, no build
op run     gabriel-greeting-go:9090  # shorthand for --listen tcp://:9090
op inspect gabriel-greeting-go       # static manifest + Describe schema

op gabriel-greeting-go SayHello '{"name":"Bob","lang_code":"en"}'   # RPC via auto-connect
op tcp://127.0.0.1:9090 SayHello '{"name":"Bob"}'                    # force transport
op stdio://gabriel-greeting-go SayHello '{…}'
```

Path expressions (`../sibling`, `./subdir`, absolute) go through the same discovery — reserve them for the parent-traversal case. Build flags that matter: `--target`, `--mode <debug|release|profile>`, `--config <name>` (sets `OP_CONFIG`), `--dry-run`, `--no-sign`.

Rebuild `op` itself (canonical, when `op` already works):

```bash
op build op --install
```

**Escape hatch only — use `go run` solely when `op` is broken on a dev machine** (or on a fresh checkout with nothing installed yet):

```bash
go run ./holons/grace-op/cmd/op env --init               # first-time only: creates OPPATH, OPBIN
go run ./holons/grace-op/cmd/op build op --install       # rebuild op via the canonical flow
eval "$(op env --shell)"                                  # activate OPBIN on PATH
```

Outside this case, never reach for `go run ./holons/grace-op/cmd/op ...` in scripts or documentation — `op build op --install` is the way.

## Using `ader`

**Why it is not optional.** A holon exposes the same operations through Code API, CLI, and RPC. Because agents and humans drive the same apps interchangeably over RPC, a change that passes unit tests can still break symmetry — the CLI keeps working while the RPC silently drifts. `ader` proves the symmetry **runtime-wise** by running the same scenario through each facet and comparing outputs.

| Level | Tool | Scope |
|---|---|---|
| Unit | `go test`, per-language equivalents | Machinery: helpers, engines, adapters. Runs on GitLab CI. |
| Integration | `ader test` | End-to-end CLI ↔ API ↔ RPC equivalence, snapshot-based. |

Dominant pattern in `ader/catalogues/grace-op/`: every `op` subcommand has a suite with a **CLI / API / RPC triplet** chained by `needs:`:

```yaml
steps:
  invoke-cli: { check: op-invoke-cli, lane: progression }
  invoke-api: { check: op-invoke-api, lane: progression, needs: [invoke-cli] }
  invoke-rpc: { check: op-invoke-rpc, lane: progression, needs: [invoke-api] }
```

Each check is a `go test -tags e2e` under `ader/catalogues/grace-op/integration/<cmd>/`, using the shared package `github.com/organic-programming/seed/ader/catalogues/grace-op/integration` (`SetupIsolatedOP`, `assertLifecycleEqual`). Run the suite whenever a change touches `op`, `ader`, an SDK, or a holon's external contract:

```bash
ader test ader/catalogues/grace-op --suite op-invoke --profile smoke --source workspace
```

`ader test` never mutates the tree or the YAML — only `ader promote` / `ader downgrade` rewrite `steps.<id>.lane`. Promotion is suite-local. Full command surface: `TDD.md`, `holons/clem-ader/README.md`. The suites are advanced R&D (self-bootstrapping chains, long timeouts). Use them, flag awkwardness, don't route around them.

## Architectural rules

Non-negotiable. Violations break the ecosystem, not just the code.

1. **The `.proto` is the gravitational center.** Every public function is an `rpc`, every type a `message`. Never hand-write public API — `protoc` generates it. Each holon has one `api/v1/holon.proto` with `option (holons.v1.manifest)`; shared domain contracts live in a neighboring `_protos/` (no copy, no symlink).
2. **Surface symmetry.** `Code API = CLI = RPC = Tests`. `contract.rpcs` matches the service exhaustively (exempt: `serve`, `help`).
3. **External surface minimal; internal volume free.** Go: `internal/` for impl, `pkg/<name>/Register(gs)` as the only exported bridge for same-language composition.
4. **One-job rule.** If describing a holon needs "and", split it. Unit of decomposition = domain.
5. **Serve & Dial.** Every binary implements `serve`, accepts `--listen <URI>` (default `stdio://`), registers `HolonMeta.Describe`, handles `SIGTERM`. `stdio://` mandatory; `tcp://` standard; others optional.
6. **No gRPC reflection** — `HolonMeta/Describe` is the canonical introspection.
7. **Generated code is committed** under `gen/` (or language-idiomatic) and never hand-edited — change the `.proto` and regenerate.
8. **No lock files** (holons are libraries): `Cargo.lock`, `package-lock.json`, `pubspec.lock`, `Gemfile.lock` stay in `.gitignore`.
9. **`op new` for scaffolding** — never hand-assemble a holon tree; UUID and structure consistency matter.
10. **Use the SDK** — never reimplement transport, framing, reflection, or lifecycle. If no SDK exists, contribute one (`sdk/<lang>-holons/`).

## Conventions per language

See `CONVENTIONS.md §4` for the per-language mapping of source dir / generated dir / tests dir / manifest file. Every holon has `api/v1/holon.proto` at its root and `protos/<pkg>/<version>/*.proto` for proto sources.

## Environment

- `OPPATH` (default `~/.op`) — user-local runtime home.
- `OPBIN` (default `$OPPATH/bin`) — install dir for holon binaries; must be on `PATH`.
- `OPROOT` — overrides discovery root (set by `--root`).
- `.op/` inside a holon = build outputs, `.gitignore`-d.

## Status signals

`INDEX.md` legend: ✅ validated, ? draft / unverified, ⚠️ known issues, `-` not implemented, ❌ ignore. Specs marked ⚠️/? are moving — re-read the current file rather than trust remembered detail. `op proxy` is **not yet implemented** (v0.6).
