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
`Gudule Greeting Godart`: one logical holon, multiple build systems,
ordered artifacts, and platform-specific glue.

This draft defines how `op build` should grow without turning `op` into
a replacement for `go build`, `cmake`, `flutter build`, Xcode, or other
toolchains.

## Core Position

`op build` is an orchestrator.

It does not compile source code by itself. It reads the holon manifest
(from `holon.proto` or `holon.yaml` as fallback), selects the declared
runner, executes the minimum required sequence, and reports the primary
artifact path.

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

### 1. Kinds

The build model should formally recognize three kinds:

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

The spec should distinguish between "binary path" and "primary
artifact".

Proposed rule:

- `artifacts.primary` is introduced for non-CLI deliverables (e.g., .app bundles).
- `artifacts.binary` remains for single-binary holons.
- if `artifacts.primary` is set, it is the success contract for
  `op build`
- otherwise `artifacts.binary` is the success contract

Examples:

- `op`: `artifacts.binary: .op/build/bin/op`
- `wisupaa-whisper`: `artifacts.binary: wisupaa-whisper`
- `Gudule Greeting Godart`: `artifacts.primary: greeting-godart/build/macos/.../gudule-greeting-godart.app`

### 4. Build Configs

Named build configurations allow holons to express build-time
variants (license modes, feature sets, linkage strategies) without
exposing runner-specific flags in the manifest.

`op` owns the **envelope**: config names, selection (`--config`),
defaults (`default_config`), and propagation to child builds.
It passes the selected config name to the runner as `OP_CONFIG`.
The holon's build system decides what the name means.

```yaml
build:
  runner: cmake
  configs:
    lgpl:
      description: "LGPL-safe build, no GPL codecs"
    gpl:
      description: "Full GPL build with x264/x265"
  default_config: lgpl
```

Runner injection:
- `cmake`: `-DOP_CONFIG=<config>` define during configure
- `go-module`: `OP_CONFIG` environment variable during build/test
- `recipe`: propagates `--config` to `build_member` children

The `recipe` runner propagates `--config` to child holon builds:

```yaml
steps:
  - build_member: daemon
    config: gpl              # override the child's default config
```

## Recipe Runner

`recipe` is the runner for composite holons.

It orchestrates:

- child holon builds
- structured command execution
- file copy or promotion steps
- artifact assertions

It must not accept raw shell strings.

Commands are represented as argv arrays.

### Proposed Manifest Shape

```yaml
kind: composite
build:
  runner: recipe
  defaults:
    target: macos
    mode: debug
  members:
    - id: daemon
      path: greeting-daemon
      type: holon
    - id: app
      path: greeting-godart
      type: component
  targets:
    macos:
      steps:
        - build_member: daemon
        - copy:
            from: greeting-daemon/gudule-daemon-greeting-godart
            to: build/gudule-daemon-greeting-godart
        - exec:
            cwd: greeting-godart
            argv: ["flutter", "pub", "get"]
        - exec:
            cwd: greeting-godart
            argv: ["flutter", "build", "macos", "--debug"]
        - assert_file:
            path: greeting-godart/build/macos/Build/Products/Debug/gudule-greeting-godart.app/Contents/Resources/gudule-daemon-greeting-godart
artifacts:
  primary: greeting-godart/build/macos/Build/Products/Debug/gudule-greeting-godart.app
```

### Recipe Concepts

`members`

- named build participants
- `type: holon` means the path must contain its own holon manifest (proto or yaml)
- `type: component` means the path is just a working directory used by
  `exec`, `copy`, or `assert_file` steps

`targets`

- selects one platform-specific build plan
- each target owns its ordered `steps`

`defaults`

- provides default `target` and `mode`
- CLI flags override defaults

### Recipe Step Types

`build_member`

- recursively executes `op build` on a member of `type: holon`
- contributes a child report entry

`exec`

- runs a command expressed as argv
- has explicit `cwd`
- no shell interpolation

`copy`

- copies a file from one manifest-relative path to another
- creates destination directories if needed

`assert_file`

- verifies a manifest-relative file exists
- used to validate bundling or packaging side effects

The first version of `recipe` does not need loops, conditionals, or
templating.

## Runner Semantics

### `go-module`

`go-module` remains a leaf runner.

`--target` has no effect unless the caller supplies target-specific
environment explicitly in a future revision.

`--mode` is accepted but informational unless the Go build command is
extended with mode-aware flags later.

### `cmake`

`cmake` may map `--mode` to `Debug`, `Release`, or `RelWithDebInfo`
internally, but the external `op build` vocabulary stays
`debug|release|profile`.

### Future Runners

New leaf runners are added by implementing the runner interface in `op`.
See `manifest.proto` `Build.runner` for the canonical runner taxonomy.

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

- `kind: composite`
- `runner: recipe`
- a primary artifact that may be an app bundle, not only a binary

## Suggested Implementation Order

### Phase 1

- add `--target`, `--mode`, `--dry-run`
- extend the build report with `build_target`, `build_mode`, `artifact`
- keep existing `go-module` and `cmake` behavior

### Phase 2

- teach manifest validation about `kind: composite`
- teach manifest validation about `runner: recipe`
- add `artifacts.primary`

### Phase 3

- implement the `recipe` runner with `build_member`, `exec`, `copy`,
  and `assert_file`
- make `Gudule Greeting Godart` the first full composite target:
  `op build examples/greeting --target macos`

### Phase 4

- add leaf runners such as `dart-package`
- decide whether `OPService` should expose lifecycle RPCs directly or
  continue routing them through CLI/invoke semantics

## Litmus Test

If this spec is correct, the following should become possible without
special-casing Gudule in Go code:

```text
op build organic-programming/recipes/go-dart-holons/examples/greeting --target macos
```

And the result should:

- build the daemon side
- build the Flutter side
- verify the daemon was bundled
- print the final `.app` path for `Gudule Greeting Godart`
