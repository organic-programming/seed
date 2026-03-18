# `op build` — Draft Specification

Status: draft

Audience:

- `grace-op` implementers
- holon manifest authors
- composite-app recipe authors

## Why This Spec Exists

`op build` already exists, but today its semantics are only partially
specified:

- the canonical manifest spec documents `go-module` and `cmake`
- the implementation already dispatches by runner
- the repository now contains buildable shapes the v0 spec does not yet
  formalize, especially `kind: composite` and `runner: recipe`

The immediate pressure comes from composite applications such as
`Gabriel Greeting App SwiftUI`: one logical holon, multiple build systems,
ordered artifacts, and platform-specific glue.

This draft defines how `op build` should grow without turning `op` into
a replacement for `go build`, `cmake`, `flutter build`, Xcode, or other
toolchains.

## Core Position

`op build` is an orchestrator.

It does not compile source code by itself. It reads the holon manifest
proto (`api/v1/holon.proto` carrying `option (holons.v1.manifest)`),
selects the declared runner, executes the minimum required sequence,
and produces a `.holon` package as output.

Language tools and platform tools remain the actual builders.

## Design Goals

- One manifest-driven command from holon root to primary artifact
- Same CLI shape for native, wrapper, and composite holons
- Explicit target and build mode, never inferred from accident
- Structured execution, not shell snippets
- Actionable failure output with the exact failed step
- JSON/text reports that can be reused later by RPC and higher-level UIs

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

Initial standard modes:

- `debug`
- `release`
- `profile`

Initial standard targets:

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

The existing lifecycle report is the base. `op build` should extend it
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

## Runner Semantics

### `go-module`

`go-module` remains a leaf runner.

`--target` has no effect unless the caller supplies target-specific
environment explicitly in a future revision.

`--mode` is accepted but informational unless the Go build command is
extended with mode-aware flags later.

Output: `.op/build/<slug>.holon/bin/<arch>/<slug>`

### `cmake`

`cmake` may map `--mode` to `Debug`, `Release`, or `RelWithDebInfo`
internally, but the external `op build` vocabulary stays
`debug|release|profile`.

Output: `.op/build/<slug>.holon/bin/<arch>/<slug>`

### `swift-package`

`swift-package` builds Swift Package Manager projects.

Output: `.op/build/<slug>.holon/bin/<arch>/<slug>`

### Future Runners

New leaf runners are added by implementing the runner interface in `op`.
See `manifest.proto` `Build.runner` for the canonical runner taxonomy.

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

## Error Model

`op build` should fail fast with one dominant error.

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

Every error should include the failing path or step.

## Compatibility With Current v0

This draft is intentionally additive.

It does not require changing the meaning of existing working manifests
for:

- `go-module`
- `cmake`
- `artifacts.binary`

But it does require formalizing concepts already present in the
repository and not yet present in the canonical manifest text:

- `kind: "composite"`
- `runner: "recipe"`
- a primary artifact that may be an app bundle, not only a binary

## Suggested Implementation Order

### Phase 1 (done)

- add `--target`, `--mode`, `--dry-run`
- extend the build report with `build_target`, `build_mode`, `artifact`
- keep existing `go-module` and `cmake` behavior
- teach manifest validation about `kind: "composite"` and `runner: "recipe"`
- add `artifacts.primary`
- implement the `recipe` runner with `build_member`, `exec`, `copy`,
  and `assert_file`

### Phase 2 (current)

- `op build` produces `.holon` packages with `bin/<arch>/` layout and
  `.holon.json` cache
- add `copy_artifact` step type for embedding `.holon` packages into bundles
- make `Gabriel Greeting App SwiftUI` the first full composite target:
  `op build gabriel-greeting-app-swiftui --target macos`

### Phase 3

- `op install` copies `.holon` packages into `$OPBIN/`
- `op discover` reads `.holon.json` from packages
- legacy bare binaries in `$OPBIN` remain launchable as fallback

### Phase 4

- add leaf runners such as `flutter`, `dart`
- add `use_installed` and `use_cached` recipe step types
- decide whether `OPService` should expose lifecycle RPCs directly or
  continue routing them through CLI/invoke semantics

## Litmus Test

If this spec is correct, the following should become possible without
special-casing any composite in Go code:

```text
op build gabriel-greeting-app-swiftui --target macos
```

And the result should:

- build the Go and Swift daemon holons (as `.holon` packages)
- build the SwiftUI host app
- embed the daemon `.holon` packages into the `.app` bundle
- codesign the bundle
- print the final `.app` path
