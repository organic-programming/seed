# `op build` — Specification

Audience:

- `grace-op` implementers
- holon manifest authors
- composite-app recipe authors

## Core Position

`op build` is an orchestrator.

It does not compile source code. It reads the holon manifest proto
(`option (holons.v1.manifest)`), stages and validates the proto schema,
selects the declared runner, executes the minimum required sequence,
and produces a `.holon` package as output.

Language tools and platform tools remain the actual builders.

## Design Goals

- One manifest-driven command from holon root to primary artifact
- Same CLI shape for native, wrapper, and composite holons
- Explicit target and build mode, never inferred from accident
- Structured execution, not shell snippets
- Actionable failure output with the exact failed step
- JSON/text reports reusable by RPC and higher-level UIs

## Non-Goals

- Dependency solving across package managers
- Replacing native toolchains
- Deployment, installation, release signing, notarization, or publishing
- Implicit graph discovery from directory layout alone
- Hiding runner-specific reality from the user

## CLI Contract

```text
op build [<holon-or-path>] [--target <target>] [--mode <mode>] [--config <config>] [--dry-run] [--no-sign]
```

Rules:

- `<holon-or-path>` defaults to `.`
- `--target` selects the platform recipe or runner target
- `--mode` defaults to `debug` (also: `release`, `profile`).
- `--config` selects a named build configuration from `build.configs`. Defaults to `build.default_config`.
- `--dry-run` prints the resolved build plan without executing it.
- `--no-sign` skips automatic ad-hoc signing for `.app` and `.framework` bundle artifacts.
- `op build` does not run tests
- `op test` remains a separate command

Standard modes:

- `debug`
- `release`
- `profile`

Standard targets:

- `macos`
- `linux`
- `windows`
- `ios`
- `ios-simulator`
- `tvos`
- `tvos-simulator`
- `watchos`
- `watchos-simulator`
- `visionos`
- `visionos-simulator`
- `android`
- `all` (recipe runner only)

Runner behavior on unsupported mode or target must fail with an
actionable error, not silently degrade.

## Proto Stage

Every `op build` runs a proto stage **before** anything else.
This stage is the structural gate: if the proto schema is invalid,
the build stops.

### Pipeline

```
Embed FS → .op/protos/ → protoparse (pure Go) → .op/pb/descriptors.binpb → Runner
```

### Embed Origin

The canonical holon protos (`holons/v1/manifest.proto`,
`holons/v1/describe.proto`, `holons/v1/coax.proto`) are compiled
into the `op` binary via Go `embed.FS`.

Because the protos come from the embed FS, they are **immutable
for a given version of `op`**. Schema evolution is a deliberate
`op` release event.

### Three Proto Locations

| Location | Role | Mutable? |
|----------|------|----------|
| `_protos/holons/v1/` (repo root) | Canonical source, human-edited | Yes |
| `grace-op/_protos/` (embed FS) | Snapshot baked into `op` binary | No — immutable per version |
| `.op/protos/` (per-holon) | Build staging, written by `op build` | Ephemeral (wiped per build) |

### Stage

`op build` writes embedded protos plus the holon's own proto files
(manifest and contract) to `.op/protos/`. This staging area is
machine-managed and ephemeral — wiped on every build, gitignored.
It does not violate the No Copy No Symlink rule: there is still
one human-authored proto per contract.

### Parse

`op` compiles the staged protos in-process using `protoparse`
(pure Go, `jhump/protoreflect`). No `protoc`, no `buf`, no
external binary is required. Manifest validation, schema checking,
and breakage detection happen here.

### Descriptor

`op` writes a `FileDescriptorSet` to `.op/pb/descriptors.binpb`.
This artifact is reused by `op inspect`, `op mcp`, and the
`HolonMeta/Describe` RPC.

### Stub Generation

Generating language-specific stubs (`.pb.go`, `.pb.swift`,
`.pb.dart`, etc.) is not `op`'s responsibility. Runners that
need generated stubs use their own toolchain (`protoc-gen-go`,
`protoc-gen-swift`, etc.) — the same way they need `go`, `swift`,
or `xcodebuild` on PATH.

## Manifest Model

The proto file `_protos/holons/v1/manifest.proto` is the canonical
schema. All examples below use proto textproto syntax as it appears
inside `option (holons.v1.manifest) = { ... }`.

### 1. Kinds

The build model formally recognizes three kinds:

- `native`
- `wrapper`
- `composite`

`composite` means the holon is a single logical deliverable assembled
from multiple buildable parts or glue steps.

### 2. Runners

Leaf runners (compiled):

- `go-module`
- `cargo`
- `cmake`
- `swift-package`

Leaf runners (interpreted/transpiled):

- `python`
- `node`
- `ruby`
- `typescript`

Leaf runners (mobile):

- `dart`
- `flutter`

Orchestration runners:

- `recipe`

### 3. Primary Artifact

The spec distinguishes between "binary path" and "primary artifact".

Rules:

- `artifacts.primary` is used for non-CLI deliverables (e.g., .app bundles).
- `artifacts.binary` is used for single-binary holons.
- if `artifacts.primary` is set, it is the success contract for
  `op build`
- otherwise `artifacts.binary` is the success contract

Examples:

- `op`: `artifacts: { binary: "op" }`
- `gabriel-greeting-c`: `artifacts: { binary: "gabriel-greeting-c" }`
- `Gabriel Greeting App`: `artifacts: { primary: "build/GabrielGreetingApp.app" }`

### 4. Build Configs

Named build configurations allow holons to express build-time
variants (license modes, feature sets, linkage strategies) without
exposing runner-specific flags in the manifest.

`op` owns the **envelope**: config names, selection (`--config`),
defaults (`default_config`), and propagation to child builds.
It passes the selected config name to the runner as `OP_CONFIG`.
The holon's build system decides what the name means.

```protobuf
build: {
  runner: "cmake"
  configs: [
    {name: "lgpl"  description: "LGPL-safe build, no GPL codecs"}
    {name: "gpl"   description: "Full GPL build with x264/x265"}
  ]
  default_config: "lgpl"
}
```

Runner injection:
- `cmake`: `-DOP_CONFIG=<config>` define during configure
- `go-module`: `OP_CONFIG` environment variable during build/test
- `recipe`: propagates `--config` to `build_member` children

## Versioning

`op build` auto-increments the **patch** component of `identity.version`
on every successful build. Major and minor versions are only changed by
human or agent action — `op` never touches them.

### Semantics

```
identity.version: "1.4.7"
                   │ │ └── patch: auto-incremented by op build
                   │ └──── minor: human/agent sets (new feature, backward-compatible)
                   └────── major: human/agent sets (breaking change)
```

### Build Flow

```
proto: version: "1.4.7"
        ↓
op build:
  1. Read version from proto   → 1.4.7
  2. Increment patch           → 1.4.8
  3. Write new version to proto
  4. Process build templates   ({{ .Version }} → "1.4.8")
  5. Run the language build
  6. On SUCCESS → proto keeps "1.4.8"
     On FAILURE → proto restored to "1.4.7" (patch not burned)
  7. Source template files restored (always)
```

### Rules

| Rule | Detail |
|------|--------|
| **Patch = build counter** | Monotonically incremented by `op build`. |
| **Major/minor = human decision** | A human writes `version: "2.0.0"` in the proto. The next successful build makes it `2.0.1`. |
| **No dry-run increment** | `op build --dry-run` does not bump the version. |
| **Failure-safe** | On build failure, the proto is restored to the original version. |
| **Git-friendly** | The version bump shows up in `git status` — commit it alongside the build. |
| **Universal** | Applies to all runners and all languages. |

### Resetting the Base

A human or agent sets a new major or minor version by editing the proto:

```protobuf
identity: {
  version: "2.0.0"   // human sets the base
}
```

The next `op build` produces `2.0.1`, then `2.0.2`, etc.

### No Version Constants in Code

With build-time templates, no holon maintains a hand-written
version constant. The version flows from the proto through templates:

```
proto (source of truth) → op build (auto-patch) → template ({{ .Version }}) → built artifact
```

## Build-Time Templates

Source files listed in `build.templates` are processed as Go templates
before the language build. Template expressions are resolved with
identity data from the holon's proto manifest, then the originals are
restored after the build completes (success or failure).

### Mechanism

1. For each file in `build.templates`: read original bytes into memory
2. Process as a Go template with identity data
3. Write resolved content to disk
4. Run the language build (compiled or packaged)
5. Restore original bytes from memory (via `defer` — always runs)

This works for all runners: compiled holons get the resolved values baked
into the binary; interpreted holons get them in the packaged `.holon` output.

### Manifest Shape

```protobuf
build: {
  runner: "go-module"
  main: "./cmd/op"
  templates: ["api/version.go"]
}
```

### Template Variables

All identity fields are available:

| Variable       | Source                    | Example      |
|---------------|--------------------------|--------------|
| `{{ .Version }}`  | `identity.version`  | `0.5.2`      |
| `{{ .UUID }}`     | `identity.uuid`     | `28f22ab5-…` |
| `{{ .GivenName }}`| `identity.given_name`| `Grace`     |
| `{{ .FamilyName }}`| `identity.family_name`| `OP`       |
| `{{ .Motto }}`    | `identity.motto`    | `One command…`|
| `{{ .Composer }}` | `identity.composer` | `B. ALTER`   |
| `{{ .Status }}`   | `identity.status`   | `draft`      |
| `{{ .Born }}`     | `identity.born`     | `2026-02-12` |

### Example: Go Version File

```go
// api/version.go
package api

func VersionString() string { return "{{ .Version }}" }
```

After `op build`, the binary returns `"0.5.2"`.
The source file is restored to contain `{{ .Version }}`.

## Runner Semantics

### `go-module`

`go-module` is a leaf runner.

`--mode` is accepted but informational.

Output: `.op/build/<slug>.holon/bin/<arch>/<slug>`

### `cmake`

`cmake` maps `--mode` to `Debug`, `Release`, or `RelWithDebInfo`
internally, but the external `op build` vocabulary stays
`debug|release|profile`.

Output: `.op/build/<slug>.holon/bin/<arch>/<slug>`

### `swift-package`

`swift-package` builds Swift Package Manager projects.

Output: `.op/build/<slug>.holon/bin/<arch>/<slug>`

### Adding Runners

New leaf runners are added by implementing the runner interface in `op`.
See `manifest.proto` `Build.runner` for the canonical runner taxonomy.

## Recipe Runner

`recipe` is the runner for composite holons.

It orchestrates:

- child holon builds
- structured command execution
- file copy or promotion steps
- artifact assertions
- holon package embedding into bundles

It must not accept raw shell strings.

Commands are represented as argv arrays.

### Manifest Shape

```protobuf
kind: "composite"
build: {
  runner: "recipe"
  defaults: {
    target: "macos"
    mode: "debug"
  }
  members: {id: "greeting-go"    path: "../gabriel-greeting-go"    type: "holon"}
  members: {id: "greeting-swift" path: "../gabriel-greeting-swift" type: "holon"}
  members: {id: "app"            path: "."                         type: "component"}
  targets: {
    key: "macos"
    value: {
      steps: {build_member: "greeting-go"}
      steps: {build_member: "greeting-swift"}
      steps: {
        exec: {
          cwd: "."
          argv: ["xcodebuild", "-scheme", "GabrielGreetingApp", "-configuration", "Debug", "-destination", "generic/platform=macOS", "-derivedDataPath", ".build/xcode/macos", "build"]
        }
      }
      steps: {
        copy_artifact: {
          from: "greeting-go"
          to: "build/GabrielGreetingApp.app/Contents/Resources/Holons/gabriel-greeting-go.holon"
        }
      }
      steps: {
        copy_artifact: {
          from: "greeting-swift"
          to: "build/GabrielGreetingApp.app/Contents/Resources/Holons/gabriel-greeting-swift.holon"
        }
      }
      steps: {assert_file: {path: "build/GabrielGreetingApp.app"}}
    }
  }
}
artifacts: {
  primary: "build/GabrielGreetingApp.app"
}
```

### Recipe Concepts

`members`

- named build participants
- `type: "holon"` means the path must contain its own holon manifest proto
- `type: "component"` means the path is just a working directory used by
  `exec`, `copy`, or `assert_file` steps

`targets`

- selects one platform-specific build plan
- each target owns its ordered `steps`

`defaults`

- provides default `target` and `mode`
- CLI flags override defaults

### Recipe Step Types

`build_member`

- recursively executes `op build` on a member of `type: "holon"`
- produces a `.holon` package under the member's `.op/build/` directory
- contributes a child report entry

`exec`

- runs a command expressed as argv
- has explicit `cwd`
- no shell interpolation

`copy`

- copies a file from one manifest-relative path to another
- creates destination directories if needed

`copy_artifact`

- copies a built member's `.holon` package to a destination path
- `from` references a member id (must be `type: "holon"`)
- `to` is a manifest-relative destination path
- copies the entire `.holon` package directory (`.holon.json`, `bin/<arch>/`, etc.)
- used to embed child holons into composite bundles (see `HOLON_PACKAGE.md` "Bundle Integration")

`assert_file`

- verifies a manifest-relative file exists
- used to validate bundling or packaging side effects

The first version of `recipe` does not need loops, conditionals, or
templating.

### Bundle Embedding Pattern

Composite holons that produce `.app` bundles embed child holons as
`.holon` packages under `Contents/Resources/Holons/`.

```protobuf
steps: {build_member: "daemon"}
steps: {
  copy_artifact: {
    from: "daemon"
    to: "build/MyApp.app/Contents/Resources/Holons/gabriel-greeting-go.holon"
  }
}
```

The codesigning step runs automatically before any `assert_file` step
when the primary artifact is a `.app` or `.framework` bundle.

See `HOLON_PACKAGE.md` "Bundle Integration" for the full embedded
package layout and discovery semantics.

## Build Output

`op build` produces a `.holon` package as its standard output
(see `HOLON_PACKAGE.md` for the full format specification).

For compiled holons:

```
.op/build/<slug>.holon/
  .holon.json                          # generated cache
  bin/<arch>/<slug>                     # compiled binary
```

For composite holons with bundle artifacts:

```
.op/build/<App>.app                    # the bundle artifact
.op/build/<App>.app.holon.json         # optional package cache
```

Where `<arch>` follows Go convention: `<os>_<cpu>` (e.g.,
`darwin_arm64`, `linux_amd64`).

The `.op/` directory as a whole is a build output directory.
A single `.gitignore` entry (`.op/`) covers everything:

```
.op/
  protos/        # staged proto sources (from embed FS)
  pb/            # compiled descriptors
  build/         # build artifacts (above)
  doc/           # generated documentation
```

## Generated Documentation

`op build` extracts proto comments from the `FileDescriptorSet`
(via `IncludeSourceCodeInfo` in `protoparse`) and produces a
per-holon reference document:

### Content

**Commands** — derived from manifest identity, skills, and sequences:

```
op build <slug>
op run <slug>
op <slug> <command>
```

**RPC catalog** — service and method names with descriptions from
proto comments, plus invocation examples:

```
op grpc+stdio://<slug> SayHello '{"name":"jesus","lang_code":"fr"}'
```

### Comment Annotations

Proto comments support structured annotations:

- `@required` — marks a field as mandatory
- `@example <value>` — provides a usage example for a field or RPC

### Output

`.op/doc/REFERENCE.md` — machine-generated, gitignored with the
rest of `.op/`.

## Success Contract

A successful `op build` must guarantee all of the following:

1. The manifest was valid.
2. The current or requested target was supported.
3. All declared prerequisites existed.
4. The runner completed all of its declared build steps.
5. The primary artifact exists at the end of the build.

If the runner exits zero but the primary artifact is missing,
`op build` fails.

## Report Contract

The lifecycle report is the base. `op build` extends it
with build-specific fields rather than inventing a second output shape.

Required report fields:

- `operation`
- `target`
- `holon`
- `dir`
- `manifest`
- `kind`
- `runner`
- `commands`
- `notes`

Additional `op build` fields:

- `build_target`
- `build_mode`
- `build_config`
- `artifact`
- `children` (for composite builds)

`artifact` is the final primary artifact path reported to the user.

`children` is a list of nested reports for sub-builds performed by a
composite runner.

## Error Model

`op build` fails fast with one dominant error.

Preferred failure categories:

- invalid manifest
- unsupported runner
- unsupported target
- unsupported mode
- missing prerequisite command
- missing prerequisite file
- failed child build
- failed command step
- missing primary artifact

Every error includes the failing path or step.