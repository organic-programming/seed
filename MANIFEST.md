# holon.yaml v0

`holon.yaml` standardizes how `op` checks, builds, tests, cleans, and locates
holon binaries.

## Scope

`holon.yaml` is strictly operational metadata.

- In scope: build, test, clean, preflight checks, artifact location.
- Out of scope: runtime transport, `serve --listen`, SDK identity, protocol
  semantics, free-form shell recipes, dependency graphs, and install hooks.

`HOLON.md` remains the human identity card.

## Schema

```yaml
schema: holon/v0
kind: native | wrapper
platforms: [darwin, linux]    # optional
build:
  runner: go-module | cmake
  main: ./cmd/who             # optional, go-module only
requires:
  commands: [go, cmake]
  files: [go.mod, CMakeLists.txt]
delegates:
  commands: [npm, node]       # wrapper holons only
artifacts:
  binary: .op/build/bin/sophia-who
```

## Fields

- `schema`: manifest version. v0 requires `holon/v0`.
- `kind`: `native` or `wrapper`. This is semantic and not inferred.
- `platforms`: optional list of supported operating systems.
- `build.runner`: selects the `op` Go runner.
- `build.main`: optional Go package path for `go-module` when the default
  `./cmd/<holon-dir>` convention does not apply.
- `requires.commands`: commands that must exist on `PATH` before build/test.
- `requires.files`: files that must exist relative to the manifest directory.
- `delegates.commands`: wrapper-only commands delegated to by the holon.
- `artifacts.binary`: path to the primary runnable binary, relative to the
  manifest directory.

All manifest paths are relative to the directory containing `holon.yaml`.

## Binary Naming

The primary binary should use the canonical full holon slug:

- `sophia-who`
- `rob-go`
- `wisupaa-whisper`
- `megg-ffmpeg`

Aliases remain valid for discovery and dispatch, but they do not rename the
primary artifact.

The single exception is `op`, which is the root entrypoint and keeps `op` as
its primary binary name.

## v0 Runner Semantics

### `go-module`

- Build: `go build -o <artifacts.binary> <build.main or ./cmd/<dir>>`
- Test: `go test ./...`
- Clean: remove `.op/`

### `cmake`

- Configure: `cmake -S . -B .op/build/cmake -DCMAKE_RUNTIME_OUTPUT_DIRECTORY=.op/build/bin`
- Build: `cmake --build .op/build/cmake`
- Test: `ctest --test-dir .op/build/cmake --output-on-failure`
- Clean: remove `.op/`

`cmake` holons are expected to register tests with CTest. If no tests are
registered, `op test` fails with an actionable `no tests configured` error.

## `op check`

`op check` validates the manifest and preflights the build contract without
compiling anything.

It verifies:

- schema validity
- platform support
- required files
- required commands
- delegated commands for wrappers
- runner-specific entrypoint expectations

## Design Constraints

- Go is the scripting language, not shell.
- Runners own the build directory under `.op/`.
- `serve` stays Article 11: `op` still launches `<binary> serve --listen <uri>`.
- `holon.yaml` is exhaustive for v0. If a field does not help `op` check, build,
  test, clean, or locate the primary binary, it does not belong here.
