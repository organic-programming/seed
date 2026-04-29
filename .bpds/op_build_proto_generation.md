# `op build` — Proto Code Generation Stage

Status: accepted RFC — refines and details the "Future direction"
paragraph in `OP_BUILD.md` §Stub Generation. Does not supersede it;
specifies how to deliver it.

`op` = [`grace-op`](../holons/grace-op/).

## Terminology

This RFC uses **"distribution"** for what `OP_SDK.md` and
`sdk/PREBUILTS.md` currently call "distribution". A distribution
bundles, per `(language, version, target)`:

- **sources / runtime libs** — stdlib, headers, language-level deps
- **build toolchain** — compiler, linker, archiver
- **proto codegen plugin(s)** — `protoc-gen-go`, `protoc-gen-prost`, …
  (introduced by this RFC)
- scoped to a single **platform × arch** host triplet

The conceptual rename of `OP_SDK.md` → `OP_DISTRIBUTION.md` and the
verb `op sdk` → `op dist` is **out of scope for this RFC** and left
to a follow-up. Until that lands, file paths (`$OPPATH/sdk/<lang>/...`)
and command names (`op sdk install`) remain as documented in
`OP_SDK.md` and `sdk/PREBUILTS.md`. The term used throughout this
document is "distribution".

### Two pre-existing codegen flows in the repo

The repository already has codegen happening in two distinct places.
This RFC must distinguish them — only the second is its target.

- **Flow A — SDK-internal bindings.** Each SDK contains pre-generated
  bindings of the canonical `holons.v1` protos (`manifest.proto`,
  `describe.proto`, `coax.proto`). Maintainers refresh these via
  `sdk/scripts/generate-protos.sh`; output is committed in
  `sdk/<lang>-holons/`. See `sdk/README.md` §"Canonical Protocol
  Generation".
- **Flow B — Per-holon stubs.** Each holon's `api/v1/holon.proto`
  generates language-specific stubs under `<holon>/gen/<lang>/`.
  Today driven by per-holon `before_commands` shelling out to
  `protoc`. **This is what this RFC replaces.**

Both flows use the same plugin set per language. Once distributions
ship plugins, A and B can converge onto the same dispatch
mechanism — the maintainer script becomes a thin caller of the same
driver that `op build` uses. Convergence is in §13 (open question);
Flow A stays outside the `op build` driver until a follow-up RFC.

### 0.1. Flow A compatibility when `manifest.proto` changes

Although the new `op build` codegen driver targets Flow B, any change
to canonical protos used by SDKs still touches Flow A. In particular,
evolving `holons.v1.HolonManifest` or nested messages such as
`HolonManifest.Build` requires maintainers to run
`sdk/scripts/generate-protos.sh` and commit the refreshed SDK
bindings in the same change.

The `Build` field set is a forward-compatibility concern for SDKs that
construct generated messages with explicit struct or object literals.
When a field is added to `HolonManifest.Build`, those hand-written
initializers must include the language's zero value (`None`, `null`,
empty list, or equivalent) so older construction sites keep compiling
and keep preserving unknown future fields where the runtime supports
them.

### Scope expansion of the prebuilt layer

Today only four languages — `c`, `cpp`, `ruby`, `zig` — have an
`$OPPATH/sdk/<lang>/` entry; the other ten are explicitly out of
scope for v1 prebuilts (`sdk/PREBUILTS.md` lines 4-7). For codegen
to cover all 14 SDKs uniformly, **this RFC requires extending the
distribution machinery to all 14 languages**. For the four "heavy"
SDKs that already ship native libs (gRPC, protobuf-c, abseil,
c-ares), the distribution gains its codegen plugin alongside what's
already there. For the ten "light" SDKs that ship nothing today, a
minimal distribution is created — at minimum the codegen plugin
binary plus a `manifest.json` — installable via the same `op sdk
install` verb. The two-tier reality (heavy vs light) survives as an
implementation detail of what's *inside* the distribution; the
discovery, install, and verify surfaces are uniform across all 14.

This expansion is a non-trivial extension of `sdk/PREBUILTS.md` v1
scope and lands in parallel with this RFC. See §11.

## 1. Problem

The `.proto` is the gravitational center of every holon. Keeping
language stubs in sync with the proto is part of the build
(Architectural rule #7: *generated code is committed under `gen/`
and never hand-edited*).

Today, holons that need regenerated stubs declare a `before_commands`
hook that shells out to `protoc` and per-language plugins. See
`examples/hello-world/*/scripts/generate_proto.sh` and
`holons/grace-op/tools/generate/main.go`.

Two costs follow:

- Every contributor must install `protoc` and N language plugins on
  PATH before they can build a holon — contradicting the "one command
  from source to artifact" goal.
- Each holon ships a bespoke `tools/generate` program. Drift between
  them is inevitable; bugs are fixed N times.

A previous draft of this RFC proposed to write all language emitters
in Go, in-process, inside `op` itself. A per-language audit (14 SDKs:
`c`, `cpp`, `csharp`, `dart`, `go`, `java`, `js`, `js-web`, `kotlin`,
`python`, `ruby`, `rust`, `swift`, `zig`) showed that:

- Only `protoc-gen-go` and `protoc-gen-go-grpc` are written in Go.
- Java, Kotlin, C#, Python, Ruby, C++, and the legacy JS emitters are
  built into `protoc` itself as C++. No Go port exists.
- Rust (`prost-build`), Swift (`protoc-gen-swift`), Dart
  (`protoc-gen-dart`) are written in their target language.
- Re-implementing all of them in Go is ~50-80 kLOC of careful work
  that re-attempts decades of upstream edge-case handling per runtime.
  Insoutenable.

This RFC therefore takes the opposite stance: **`op` does not own
the emitters; the distributions do.** Each distribution embeds the plugin
binaries needed to generate stubs for its language. `op sdk install
<lang>` lays them down under `$OPPATH/sdk/<lang>/`. `op build`
parses the `.proto`, builds a standard `CodeGeneratorRequest`, and
invokes the plugin as a subprocess via the standard plugin protocol.

## 2. Goals

- Zero developer-installed tooling: nothing on PATH, no manual
  `protoc` or plugin installs. Plugins arrive via
  `op sdk install <lang>`.
- Single source of truth: the holon's `.proto` files drive every
  generated artifact under `gen/`.
- One driver, no per-holon generators: `op` parses, orchestrates,
  writes; plugins emit. No `tools/generate` programs.
- Reproducibility: pinning an distribution version pins the plugin
  version, which pins the generated bytes for given proto inputs.
- Replaces (does not coexist with) the per-holon `before_commands`
  generator pattern for languages whose distribution declares
  codegen plugins.

## 3. Non-Goals

- Not a replacement for language compilers. Generated `.rs` still
  needs `cargo` to become a library; generated `.swift` still needs
  `swift build`. Codegen produces source, not binaries.
- Not a generic protoc plugin host. `op` does not invoke
  user-supplied `protoc-gen-*` binaries from PATH. Plugins are only
  resolved from installed distributions.
- Not a fork of the upstream emitters. `op` runs them as-is.

## 4. Pipeline Placement

The proto stage in `OP_BUILD.md` is currently:

```
Embed FS → .op/protos/ → protoparse → .op/pb/descriptors.binpb → Runner
```

Codegen inserts one phase between descriptor production and the
runner:

```
Embed FS
   ↓
.op/protos/                                  (stage)
   ↓
protocompile                                 (parse + link)
   ↓
.op/pb/descriptors.binpb                     (descriptor artifact)
   ↓
codegen dispatch → gen/<lang>/...            (NEW)
   ↓
preflight (requires.commands, sdk_prebuilts)
   ↓
runner (go-module, cmake, recipe, …)
```

Codegen runs **before preflight and before the runner**, so a holon
whose runner consumes generated sources (e.g. `go-module` reading
`gen/go/`) sees fresh stubs at compile time. It runs **after** the
descriptor write so `op inspect` and `op mcp` keep working from the
same artifact.

`--dry-run` prints the planned codegen outputs and the resolved
plugin binary paths, then skips both write and runner.

## 5. Library Choice

Replace `jhump/protoreflect` with `bufbuild/protocompile` for
parsing.

Rationale:

- `jhump/protoreflect` is on maintenance footing; Joshua Humphries
  joined Buf and the active successor is `bufbuild/protocompile`.
- `protocompile` produces `protoreflect.FileDescriptor` values
  directly compatible with `protodesc.ToFileDescriptorProto`, which
  is what we need to build a `pluginpb.CodeGeneratorRequest` to
  feed plugins via stdin.

The migration is broader than this stage: `internal/inspect/parser.go`
and `internal/holons/doc_gen.go` also depend on `jhump/protoreflect`
v1.18.0, plus `proto_stage.go` itself. They migrate in the same PR
so the dependency drops cleanly from `go.mod`.

`google.golang.org/protobuf/types/pluginpb` provides the
`CodeGeneratorRequest` / `CodeGeneratorResponse` types. `op` does
not import `google.golang.org/protobuf/compiler/protogen` — that
library is a plugin-side helper; `op` is the driver, not the plugin.

## 6. Codegen Architecture

Two parties:

```
+----------------------------+        +-----------------------------+
|   op (driver)              |        |   plugin (in distribution)  |
|   - parse with protocompile|        |   - reads CodeGenRequest    |
|   - build CodeGenRequest   |  stdin |     from stdin              |
|   - resolve plugin binary  | -----> |   - emits files into        |
|     under $OPPATH/sdk/...  |        |     CodeGenResponse         |
|   - spawn subprocess       | <----- |   - writes response to      |
|   - read CodeGenResponse   | stdout |     stdout                  |
|   - write files to gen/    |        |                             |
+----------------------------+        +-----------------------------+
```

This is the **standard `protoc` plugin protocol** — the same wire
contract upstream plugins already implement. `op` plays the role
that `protoc` plays in a normal `protoc --go_out=...` invocation:
parser + dispatcher, not emitter.

### 6.1. Plugin discovery

For each entry in `build.codegen.languages`:

1. The entry name is interpreted as `<sdk>` or `<sdk>-<variant>`
   (e.g. `go`, `go-grpc`, `rust`, `swift`).
2. The distribution slug is `<sdk>` (the prefix before the first `-`),
   if any. The slug remains spelled `<sdk>` in path and command
   surfaces until the rename RFC lands.
3. `op` reads the distribution manifest at
   `$OPPATH/sdk/<sdk>/<version>/<host_target>/manifest.json` and
   matches the entry name against `codegen.plugins[].name`.
4. The plugin's `binary` path resolves relative to the distribution
   root.

If the distribution is missing → preflight fails with the action
`op sdk install <sdk>`.

If the distribution exists but does not declare a plugin matching
the name → preflight fails with `unsupported codegen language` and
lists the plugin names the distribution does declare.

### 6.2. distribution manifest extension

The existing prebuilt `manifest.json` (per `OP_SDK.md`) gains a
`codegen` block:

```json
{
  "lang": "go",
  "version": "1.26.1",
  "target": "darwin_arm64",
  "...": "...",
  "codegen": {
    "plugins": [
      {
        "name": "go",
        "binary": "bin/protoc-gen-go",
        "out_subdir": "go"
      },
      {
        "name": "go-grpc",
        "binary": "bin/protoc-gen-go-grpc",
        "out_subdir": "go"
      }
    ]
  }
}
```

Fields:

- `name` — the identifier the holon manifest uses in
  `build.codegen.languages`.
- `binary` — path relative to the prebuilt root. Must be executable
  on the host platform.
- `out_subdir` — directory under `gen/` where the plugin's output
  is written. Multiple plugins may share an `out_subdir` (e.g.
  `go` and `go-grpc` both write under `gen/go/`).

Multiple plugins per distribution are common (messages + gRPC).
Plugins from different distributions can share an `out_subdir` (e.g.
a future `protoc-gen-grpc-web` shipped in `js-web` could write into
`gen/js-web/`).

### 6.3. Plugin invocation

Standard `protoc` plugin protocol:

1. `op` serializes the `CodeGeneratorRequest` as binary protobuf.
2. `op` spawns the plugin binary with stdin = serialized request,
   stdout = pipe, stderr = inherited (so plugin logs surface
   alongside `op build` output).
3. Plugin emits a `CodeGeneratorResponse` to stdout, then exits.
4. `op` reads stdout fully, deserializes, processes `resp.File[*]`.
5. Each `resp.File.Name` is treated as relative to the language's
   `out_subdir` under `gen/`. Paths must not escape `gen/<out_subdir>/`
   (see §10).

The plugin's working directory is an ephemeral tmpdir.
Output is collected from stdout, never from disk.

`op` invokes plugins concurrently within a single `op build`. File
writes are serialized at the sink; output ordering inside `gen/` is
alphabetical regardless of plugin completion order.

## 7. Manifest Surface

Codegen is opt-in per holon via a new `build.codegen` block.
Deliberately minimal — one field:

```protobuf
build: {
  runner: "go-module"
  main:   "./cmd/op"
  codegen: {
    languages: ["go", "go-grpc"]
  }
}
```

Rules:

- If `build.codegen` is absent, no codegen runs — backward
  compatible with holons still using `before_commands` hooks during
  the migration window.
- `languages` is an ordered list. Each entry must resolve to a
  plugin in some installed distribution; otherwise preflight fails.
- Once `build.codegen` is set, declaring the same languages in
  `before_commands` is a manifest validation error to prevent
  double-generation.

### 7.1. Frozen by design

The following are **not** configurable in the manifest. `op` chooses
them for every holon:

| Concern | Frozen value |
|---|---|
| Output root | `gen/` at the holon root |
| Per-language subdir | `gen/<out_subdir>/...` from distribution manifest |
| Proto discovery | `api/v1/*.proto` and `_protos/**/*.proto` under the holon root |
| Plugin location | `$OPPATH/sdk/<sdk>/<version>/<target>/<binary>` only — never PATH |
| Per-plugin parameters | None — plugins use their built-in defaults |

Rationale: one layout across the entire ecosystem, no per-holon
divergence, no bikeshedding. If a future need forces customization,
this section is the place to revisit — but a concrete, named use
case must precede any new option.

## 8. Output Location and Idempotency

Generated code is **committed**, per Architectural rule #7:

```
<holon-root>/
  api/v1/holon.proto                     # source of truth
  gen/
    go/op/v1/holon.pb.go                  # committed
    go/op/v1/holon_grpc.pb.go             # committed
    rust/op/v1/holon.rs                   # committed
    swift/op/v1/Holon.pb.swift            # committed
```

`op build` writes only paths it owns:

- A path is "owned by codegen" iff it was produced by a plugin on
  the previous run, or it does not yet exist.
- `op` records owned paths in `.op/codegen-manifest.json` (machine
  cache, gitignored). The next run uses this manifest to detect
  files that should be removed (e.g. a `.proto` was deleted, so its
  generated stub should disappear).
- Files under `gen/<out_subdir>/` not in the manifest are left
  alone. Preflight surfaces them as a warning during migration.

`--dry-run` prints the diff (added / changed / removed paths)
without writing.

If post-codegen the working tree under `gen/` differs from the
index, `op build` proceeds; the diff shows up in `git status` so
the contributor commits it alongside source changes. CI runs
`op build && git diff --exit-code gen/` to detect missed
regenerations.

## 9. Determinism

The success contract grows by one clause:

> A successful `op build` produces byte-identical files under
> `gen/<out_subdir>/` given the same `op` version, the same pinned
> distribution versions, and the same proto inputs.

Two-level responsibility:

- **`op` (driver)** — guarantees byte-identical input to plugins
  (sorted file list, stable `CodeGeneratorRequest` field order),
  alphabetical output ordering at the sink, no host-derived metadata
  written into generated files.
- **Plugins (upstream)** — responsible for their own determinism.
  Out of scope for this RFC; assumed to be a property of the pinned
  plugin version.

The `op` version and the distribution versions used are recorded in
`.op/codegen-manifest.json`, not inside generated files.

## 10. Errors

New error categories under `OP_BUILD.md` §Error Model:

- `missing distribution for codegen` — language `X` declared but
  `$OPPATH/sdk/<X>/...` is absent. Action: `op sdk install <X>`.
- `unsupported codegen language` — distribution installed but does
  not declare a plugin matching the language name. Lists the names
  the distribution does declare.
- `codegen plugin failed` — plugin exited non-zero, or the response
  contains an `error` field, or stdout is not a valid
  `CodeGeneratorResponse`.
- `codegen path escaped out_dir` — plugin returned a `File.Name`
  that resolves outside `gen/<out_subdir>/`. Security guard.
- `codegen deleted unexpected file` — the codegen manifest claims
  ownership of a file that has since been hand-edited.

## 11. Migration

Order of operations:

0. **Distribution layer extends to all 14 languages.** A parallel
   change to `sdk/PREBUILTS.md` and `internal/sdkprebuilts/` adds
   `go`, `rust`, `swift`, `dart`, `java`, `kotlin`, `csharp`,
   `python`, `js`, `js-web` to the install/build/verify surface.
   For each, the minimum content is the codegen plugin binary plus
   a `manifest.json` declaring it. The four heavy SDKs (`c`, `cpp`,
   `ruby`, `zig`) gain `codegen.plugins` alongside their existing
   native libs. This is the prerequisite — no holon can opt in
   before its language's distribution exists.
1. **Distributions ship plugins.** For each language, add
   `protoc-gen-*` binaries to the distribution and declare them in
   `manifest.json`. Lands per language; order independent.
2. **Holons opt in.** For every reference holon currently using a
   `before_commands` generator:
   - Add `build.codegen.languages` to the manifest.
   - Run `op build`; verify the diff under `gen/` matches what the
     old `protoc` invocation produced (modulo deterministic
     differences like header comments).
   - Remove the `before_commands` block and the `tools/generate`
     program.
   - Remove `requires.commands: ["protoc", "protoc-gen-*"]` entries
     (`requires.sdk_prebuilts` now covers the dependency).
3. **Cleanup.** Once all reference holons are migrated, delete
   `holons/grace-op/tools/generate` and the per-example
   `scripts/generate_proto.sh`.

The migration lands one distribution at a time. Per Architectural
rule #7, generated code is committed; per the No Lock Files rule,
no lockfile changes either way.

## 12. Bootstrap (`op build op --install`)

The chicken-and-egg case: `op` itself uses
`bufbuild/protocompile` and consumes `gen/go/op/v1/`. If `op` is
being built from a clean checkout on a fresh machine, the Go distribution
prebuilt may not yet be installed.

Resolution: **`op` does not regenerate its own stubs at build
time.** The `gen/` tree is committed (rule #7). A clean
`op build op --install` reads existing `gen/go/op/v1/*.pb.go` from
the working tree and compiles. Codegen is only triggered when a
proto changes, which is a developer action — at that point the
developer is expected to have the Go distribution installed.

Concretely, `op` itself does not set `build.codegen` on its own
manifest. Regenerating `op`'s stubs is a separate explicit
`op build op --regen-stubs`-style invocation, or a `make
regen-stubs` target. Out of scope for this RFC to specify.

## 13. Open Questions

- **Sharing protos across holons.** `_protos/` at repo root holds
  shared contracts. When holon A imports `holons/v1/manifest.proto`,
  does codegen emit the manifest stubs into A's `gen/`, or are
  shared stubs centralized? Bias: emit per-holon for now;
  centralization can come later.
- **Plugin-name namespacing.** Today plugin names like `go`,
  `go-grpc` are distribution-scoped (declared inside the Go
  distribution manifest). Two distributions declaring a plugin
  with the same name would currently not collide, since the holon
  manifest names a plugin by the language entry which carries its
  own distribution prefix. Worth a sanity test before locking in.
- **Cross-distribution plugins.** Some plugins
  (`protoc-gen-grpc-web`) could serve multiple distributions.
  Allow declaring the same plugin in multiple distribution
  manifests with different `out_subdir`? Defer until a concrete
  case appears.
- **Convergence with Flow A (SDK-internal bindings).** Once
  distributions ship plugins for all 14 languages, the maintainer
  script `sdk/scripts/generate-protos.sh` can be retired — it
  becomes a thin wrapper invoking the same dispatch driver `op build`
  uses, with `<repo>/sdk/<lang>-holons/...` as the output root
  instead of `<holon>/gen/<lang>/`. Out of scope for this RFC;
  worth a follow-up.

## 14. Out of Scope

- Documentation generation (`.op/doc/REFERENCE.md`) is already
  produced from the descriptor and stays untouched.
- `op test` does not invoke codegen; it expects the working tree to
  contain up-to-date `gen/`. CI gates that with
  `git diff --exit-code`.
- Hot-reload of generated code in long-running `op serve` sessions.
- Vendoring or forking upstream plugins. `op` runs them as-is from
  the distribution.

---

## Appendix A — Driver Sketch

The `op`-side driver: parse with `protocompile`, build the
`CodeGeneratorRequest`, dispatch to plugins.

```go
package codegen

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "os/exec"

    "github.com/bufbuild/protocompile"
    "google.golang.org/protobuf/proto"
    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/pluginpb"
)

type Plugin struct {
    Name      string // matches build.codegen.languages entry
    Binary    string // absolute path resolved via distribution manifest
    OutSubdir string // e.g. "go", "rust"
}

type EmittedFile struct {
    Lang    string // == Plugin.OutSubdir
    Path    string // relative to gen/<lang>/
    Content []byte
}

func Run(ctx context.Context, importPaths, entries []string, plugins []Plugin) ([]EmittedFile, error) {
    compiler := protocompile.Compiler{
        Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
            ImportPaths: importPaths,
        }),
    }

    linked, err := compiler.Compile(ctx, entries...)
    if err != nil {
        return nil, fmt.Errorf("parse: %w", err)
    }

    req := &pluginpb.CodeGeneratorRequest{}
    visited := map[string]bool{}

    var addFile func(fd protoreflect.FileDescriptor)
    addFile = func(fd protoreflect.FileDescriptor) {
        if visited[fd.Path()] {
            return
        }
        visited[fd.Path()] = true
        imports := fd.Imports()
        for i := 0; i < imports.Len(); i++ {
            addFile(imports.Get(i))
        }
        req.ProtoFile = append(req.ProtoFile, protodesc.ToFileDescriptorProto(fd))
    }
    for _, f := range linked {
        addFile(f)je pen
        req.FileToGenerate = append(req.FileToGenerate, f.Path())
    }

    reqBytes, err := proto.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    var out []EmittedFile
    for _, p := range plugins {
        files, err := invoke(ctx, p, reqBytes)
        if err != nil {
            return nil, fmt.Errorf("plugin %s: %w", p.Name, err)
        }
        out = append(out, files...)
    }
    return out, nil
}

func invoke(ctx context.Context, p Plugin, reqBytes []byte) ([]EmittedFile, error) {
    cmd := exec.CommandContext(ctx, p.Binary)
    cmd.Stdin = bytes.NewReader(reqBytes)

    var stdout bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = nil // inherited by parent; surfaces in op build output

    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("exec: %w", err)
    }

    resp := &pluginpb.CodeGeneratorResponse{}
    if err := proto.Unmarshal(stdout.Bytes(), resp); err != nil {
        return nil, fmt.Errorf("unmarshal response: %w", err)
    }
    if errMsg := resp.GetError(); errMsg != "" {
        return nil, fmt.Errorf("plugin error: %s", errMsg)
    }

    files := make([]EmittedFile, 0, len(resp.File))
    for _, f := range resp.File {
        files = append(files, EmittedFile{
            Lang:    p.OutSubdir,
            Path:    f.GetName(),
            Content: []byte(f.GetContent()),
        })
    }
    return files, nil
}

// Caller is responsible for:
// - reading manifest.json from $OPPATH/sdk/<sdk>/<version>/<target>/
// - resolving Plugin.Binary as an absolute path
// - validating each EmittedFile.Path stays within gen/<Lang>/
// - writing files to disk and updating .op/codegen-manifest.json
func _ReadmeForCallers() {}
```
