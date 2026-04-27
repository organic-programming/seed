# `op new` — per-language holon templates

Status: spec — extends [OP.md §5](OP.md) (`op new`) without modifying its CLI surface.

This document specifies the **per-language SDK template catalog** that closes the long-standing gap where only organism-kit composites (`coax-flutter`, `coax-swiftui`) had scaffolding support. The `op new` command itself does not change.

---

## 1. The mechanism is already complete

`op new` already provides everything needed:

```
op help new
Usage:
  op new [flags]

Flags:
      --json string       raw JSON payload for identity creation
      --list              list shipped holon templates
      --set stringArray   template override in key=value form
      --template string   template name to scaffold
```

Backed by:

- RPCs `CreateIdentity`, `ListTemplates`, `GenerateTemplate` declared in [api/v1/holon.proto](api/v1/holon.proto).
- A template framework under [templates/catalog/](templates/) using `template.yaml` + Go-template `.tmpl` files. Proven by the existing `coax-flutter` and `coax-swiftui` templates.
- The `TemplateEntry.lang` field already lets `--list` group templates by language.

**No new flag, no new RPC, no engine change is required.** This spec only adds template directories.

## 2. What is missing

The catalog ships only two composite templates today:

```
holons/grace-op/templates/catalog/
├── coax-flutter/
└── coax-swiftui/
```

There is no single-language SDK holon template for any of the 14 supported SDKs (`go`, `rust`, `swift`, `dart`, `python`, `js`, `js-web`, `kotlin`, `java`, `csharp`, `cpp`, `c`, `ruby`, `zig`).

[CLAUDE.md](../../CLAUDE.md) rule #9 mandates `op new` for all scaffolding. The current state forces hand-assembly for any non-composite holon, directly contradicting the rule. This spec closes the gap by adding 14 template directories.

## 3. Naming convention

```
holons/grace-op/templates/catalog/holon-<lang>/
```

`coax-*` is reserved for composite organisms. `holon-*` is reserved for single-language SDK holons. This is purely a catalog convention — the `op new --template <name>` mechanism is agnostic to it.

End-user invocation:

```bash
op new --template holon-go     --set slug=rob-go     rob-go
op new --template holon-rust   --set slug=rob-rust   rob-rust
op new --template holon-swift  --set slug=rob-swift  rob-swift
# ...
```

(Slug delivery via positional vs `--set` follows whatever the existing template framework expects — confirm against the `coax-*` templates, do not invent.)

## 4. Required output of every `holon-<lang>` template

Each template **must** produce a holon that passes `op check`, `op build`, and `op test` with zero hand-editing, conforming to:

- [CONVENTIONS.md §4](../../CONVENTIONS.md) — source / generated / tests / manifest layout.
- [PROTO.md](../../PROTO.md) — `api/v1/holon.proto` central, manifest options set, RPC contract enumerated.
- [CLAUDE.md](../../CLAUDE.md) rule #5 — `serve`, `--listen <URI>`, `HolonMeta.Describe`, SIGTERM.
- [CLAUDE.md](../../CLAUDE.md) rule #8 — no committed lock files (Cargo.lock, package-lock.json, pubspec.lock, Gemfile.lock, etc., all in `.gitignore`).

### Mandatory files

1. `api/v1/holon.proto` with:
   - `option (holons.v1.manifest)` populated (UUID, name, composer, motto, clade, reproduction, runner, contract).
   - At least one demo RPC service with one method (canonical: `Greeting.SayHello`, mirroring `examples/hello-world/gabriel-greeting-go/`).
   - `contract.rpcs` enumerating the service exhaustively (rule #2 surface symmetry).
   - `contract.runner` matching the language's runner key in `holons/grace-op/internal/runner/`.
2. The language-idiomatic source layout per [CONVENTIONS.md §4](../../CONVENTIONS.md) (table in §5).
3. A working `serve` entry point that:
   - parses `--listen <URI>` (default `stdio://`),
   - registers `HolonMeta.Describe` from the generated static response,
   - binds the demo service,
   - handles SIGTERM with graceful shutdown.
4. At least one minimal test exercising the demo RPC end-to-end (CLI → RPC parity).
5. `README.md` showing `op build`, `op test`, and one `op <slug> SayHello '{"name":"Bob"}'` invocation.
6. `.gitignore` covering build outputs and lock files for that language.

### `template.yaml` schema (per template)

```yaml
name: holon-<lang>
description: SDK holon — <Lang> (<short note>)
lang: <lang>
params:
  - name: slug
    description: Holon slug (kebab-case)
    required: true
  - name: composer
    default: ""
  - name: motto
    default: ""
  - name: clade
    default: deterministic/io_bound
  - name: reproduction
    default: clone
  # ... language-specific params if any
```

Templates may declare additional language-specific params (e.g., `swift_package_target`, `dart_pub_name`, `proto_go_package`). Computed values (`pascal_slug`, `snake_slug`, `uuid`) come from the existing template engine — confirm what `coax-*` already exposes before re-declaring them.

## 5. Per-language matrix

Layout follows [CONVENTIONS.md §4](../../CONVENTIONS.md) exactly. Runner key feeds `contract.runner` and matches the dispatch table in `holons/grace-op/internal/runner/`.

| `lang` | Manifest | Source | Generated | Tests | Runner | SDK dependency |
|---|---|---|---|---|---|---|
| `go` | `go.mod` | `pkg/` | `gen/go/` | co-located `*_test.go` | `go` | `go.work` workspace replace to `sdk/go-holons` |
| `rust` | `Cargo.toml` | `src/` | `src/gen/` | `tests/` + inline | `rust` | path-dep on `sdk/rust-holons` |
| `swift` | `Package.swift` | `Sources/` | `Sources/Gen/` | `Tests/` | `swift` | path-dep on `sdk/swift-holons` |
| `dart` | `pubspec.yaml` | `lib/` | `lib/gen/` | `test/` | `dart` | path-dep on `sdk/dart-holons` |
| `python` | `pyproject.toml` | `<snake_slug>/` | `gen/python/` | `tests/` | `python` | editable install of `sdk/python-holons` |
| `js` | `package.json` | `src/` | `src/gen/` | `tests/` | `node` | local file ref to `sdk/js-holons` |
| `js-web` | `package.json` | `src/` | `src/gen/` | `tests/` | `node-web` | local file ref to `sdk/js-web-holons` |
| `kotlin` | `build.gradle.kts` | `src/main/kotlin/` | `src/main/kotlin/gen/` | `src/test/kotlin/` | `kotlin` | mavenLocal / project ref to `sdk/kotlin-holons` |
| `java` | `build.gradle` | `src/main/java/` | `src/main/java/gen/` | `src/test/java/` | `java` | mavenLocal / project ref to `sdk/java-holons` |
| `csharp` | `*.csproj` | `Holons/` | `Holons/Gen/` | `Holons.Tests/` | `dotnet` | project ref to `sdk/csharp-holons` |
| `cpp` | `CMakeLists.txt` | `src/` | `gen/cpp/` | `tests/` | `cpp` | CMake `add_subdirectory(sdk/cpp-holons)` |
| `c` | `Makefile` | `src/` | `gen/c/` | `tests/` | `c` | link to `sdk/c-holons` |
| `ruby` | `Gemfile` | `lib/` | `lib/gen/` | `test/` | `ruby` | `path:` gemspec ref to `sdk/ruby-holons` |
| `zig` | `build.zig` | `src/` | `gen/` | `tests/` | `zig` | path-dep on `sdk/zig-holons` |

### Per-language idiomatic main file (illustrative — templates may differ)

- **Go** — `pkg/cmd/<slug>/main.go` calling `serve.RunCLIOptions(...)`.
- **Rust** — `src/main.rs` calling `holons::serve::run_with_options(...)`.
- **Swift** — `Sources/<PascalSlug>/main.swift` calling `Serve.runWithOptions(...)`.
- **Dart** — `bin/<slug>.dart` calling `runWithOptions(...)`.
- **Python** — `<snake_slug>/__main__.py` calling `holons.serve.run_with_options(...)`.
- **JS (Node)** — `src/server.js` calling `holons.serve.runWithOptions(...)`.
- **JS (Web)** — `src/index.ts` constructing a dial client (browser cannot serve).
- **Kotlin** — `src/main/kotlin/.../Main.kt`.
- **Java** — `src/main/java/.../Main.java`.
- **C#** — `Holons/Program.cs`.
- **C++** — `src/main.cpp`.
- **C** — `src/main.c` linking the `holons` static lib.
- **Ruby** — `lib/<snake_slug>/server.rb`.
- **Zig** — `src/main.zig` calling `holons.serve.runCliOptions(...)`.

## 6. Verification matrix

A 14-row matrix that must be entirely green to ship the catalog:

| `lang` | scaffold via `op new --template holon-<lang>` | `op check` | `op build` | `op test` | RPC smoke |
|---|:---:|:---:|:---:|:---:|:---:|
| go | ✓ | ✓ | ✓ | ✓ | `op smoke-go SayHello '{"name":"Bob"}'` |
| rust | ✓ | ✓ | ✓ | ✓ | `op smoke-rust SayHello '{"name":"Bob"}'` |
| swift | ✓ | ✓ | ✓ | ✓ | `op smoke-swift SayHello '{"name":"Bob"}'` |
| dart | ✓ | ✓ | ✓ | ✓ | `op smoke-dart SayHello '{"name":"Bob"}'` |
| python | ✓ | ✓ | ✓ | ✓ | `op smoke-python SayHello '{"name":"Bob"}'` |
| js | ✓ | ✓ | ✓ | ✓ | `op smoke-js SayHello '{"name":"Bob"}'` |
| js-web | ✓ | ✓ | ✓ | (build-only) | (dial-only browser demo) |
| kotlin | ✓ | ✓ | ✓ | ✓ | `op smoke-kotlin SayHello '{"name":"Bob"}'` |
| java | ✓ | ✓ | ✓ | ✓ | `op smoke-java SayHello '{"name":"Bob"}'` |
| csharp | ✓ | ✓ | ✓ | ✓ | `op smoke-csharp SayHello '{"name":"Bob"}'` |
| cpp | ✓ | ✓ | ✓ | ✓ | `op smoke-cpp SayHello '{"name":"Bob"}'` |
| c | ✓ | ✓ | ✓ | ✓ | `op smoke-c SayHello '{"name":"Bob"}'` |
| ruby | ✓ | ✓ | ✓ | ✓ | `op smoke-ruby SayHello '{"name":"Bob"}'` |
| zig | ✓ | ✓ | ✓ | ✓ | `op smoke-zig SayHello '{"name":"Bob"}'` |

A per-template smoke test in the `holons/grace-op/` `go test` suite drives `GenerateTemplate` + `op check` + `op build` + `op test` + RPC roundtrip in a tempdir, per language.

ader integration: extend `ader/catalogues/grace-op/integration/new/` with a CLI / API / RPC triplet per [TDD.md](../../TDD.md) for at least one representative language per family (compiled / managed / dynamic).

## 7. Open questions / decisions

These need composer arbitration per [CLAUDE.md](../../CLAUDE.md) "doubt is the method":

1. **`js-web` is not a server.** Should `holon-js-web` produce anything, or be skipped? Proposal: minimal browser dial demo with a stubbed `index.html`, `kind: client` in the manifest. Alternative: skip the template entirely, document `js-web` as dial-from-existing-app only.
2. **SDK dependency wiring (workspace vs release).** Inside this monorepo, scaffolds reference SDKs via path/file/project refs. External consumers of `op new` need release-version refs. Proposal: each template ships both modes, switched via `OP_SDK_SOURCE=workspace|release` (default `workspace` inside the seed repo, `release` elsewhere). Decision needed before external rollout.
3. **UUID provenance.** When `op new --template holon-<lang>` is used directly (no prior `CreateIdentity`), the UUID must come from somewhere. Proposal: the template engine generates one inline if `--set uuid=<v>` is not provided.
4. **`runner` key registry.** Confirm exact runner keys against `holons/grace-op/internal/runner/`. Some entries in §5 are conjectures (e.g., `node-web`, `dotnet`) — verify before writing templates.
5. **Lock-file `.gitignore` policy per language.** Confirm the explicit lock-file list per [CLAUDE.md](../../CLAUDE.md) rule #8: at least `Cargo.lock`, `package-lock.json`, `pubspec.lock`, `Gemfile.lock`, `Package.resolved`, `composer.lock`, `Pipfile.lock`, `poetry.lock`. C/C++/Java/Kotlin/C# do not have a lock-file convention to ignore.
6. **`holon-zig` timing.** Land after Zig SDK chantier P8 (`op` runner registration). Sequencing: 13 templates first, `holon-zig` follows.
7. **Optional `--lang` ergonomic alias.** Not required, but `op new --lang rust foo` reads better than `op new --template holon-rust foo`. If desired, this is a 5-line code addition: in `holons/grace-op/api/cli_identity.go` and `holons/grace-op/internal/cli/who.go`, recognise `--lang <X>` and resolve it to `template = "holon-<X>"`. Decision: ship as-is with `--template`, or add `--lang` as sugar?

## 8. Rollout

Recommended phasing:

1. **Resolve §7** with the composer (decision points).
2. **Wave 1** — `holon-go` + `holon-rust` (most mature SDKs, lowest template risk). Establishes the per-language template pattern and the smoke-test harness.
3. **Wave 2** — `holon-swift`, `holon-dart`, `holon-python` (next mature tier).
4. **Wave 3** — remaining: `js`, `js-web`, `kotlin`, `java`, `csharp`, `cpp`, `c`, `ruby`.
5. **Verification matrix green** (§6).
6. **`holon-zig`** — added once Zig SDK chantier P8 lands.
7. **ader integration** — triplets per [TDD.md](../../TDD.md).
8. **Promote** the `op new` entry in [INDEX.md](../../INDEX.md) `op CLI` table to ✅.

Each wave is an independently mergeable PR series, so the spec does not block on any single language's template difficulty.
