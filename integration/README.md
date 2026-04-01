# Local Regression Control Plane

`integration/` is the local regression control plane for this repository.

The point is not to squeeze a giant polyglot system into cheap hosted CI. The
point is to have one local place from which you can run:

- `grace-op` unit tests
- SDK unit tests across languages
- example holon native unit tests
- the black-box `op` integration suite
- one aggregated report that survives the limits of agent context windows

That aggregated local report is the artifact used in the regression loop after
agent changes.

## Why This Exists

This repo is too wide for free CI to carry real regression pressure:

- multiple language toolchains
- platform-specific slices such as Darwin COAX coverage
- long-running build and integration flows
- repeated rebuilds and transport tests
- agent work that can break distant parts of the tree the agent did not model

So the strategy is local-first and deliberate.

`integration/` does **not** mean “rewrite every test as a Go integration test.”
It means:

- native unit tests stay native
- `integration/tests/` stays black-box
- `integration/run-local-suite.sh` is the umbrella runner
- `integration/reports/<timestamp>/` is the regression memory surface

That is what “integration subsumes unit tests” means here.

## The Layers

There are four layers:

1. `grace-op` unit tests
2. SDK unit tests
3. example holon native unit tests
4. real-binary black-box integration tests in
   [`integration/tests`](/Users/bpds/Documents/Entrepot/Git/Compilons/seed/integration/tests)

The runner executes those layers and emits one report.

## Isolation Strategy

There are two different isolation stories.

### `integration/tests`

The black-box suite in
[`integration/tests`](/Users/bpds/Documents/Entrepot/Git/Compilons/seed/integration/tests)
is self-isolated:

- it builds a canonical `op` binary under `integration/.artifacts/run-*/`
- it mirrors `examples/` and `sdk/` into that run directory
- it points `--root`, `cmd.Dir`, `OPPATH`, `OPBIN`, and `TMPDIR` at that copy
- it exposes `integration/.t/` as a short temp alias for Unix socket safety

That suite should not mutate the real `examples/` or `sdk/` trees.

### The umbrella runner

The umbrella runner also creates a mirrored local-suite workspace under:

```text
integration/.artifacts/local-suite/<timestamp>/workspace/
```

It copies the source trees needed for native unit suites:

- `holons/`
- `sdk/`
- `examples/`
- `protos/`
- `scripts/`

The native unit suites run from that mirrored workspace, not the real repo
paths. That keeps build directories, temp files, Python bytecode, and other
toolchain-local artifacts inside `integration/.artifacts/` as much as the
native tools allow.

Shared caches are also redirected under:

```text
integration/.artifacts/tool-cache/
```

## Reports

Each umbrella run writes a report under:

```text
integration/reports/<timestamp>/
```

Artifacts per run:

- `summary.md`
  Human-readable global result
- `summary.tsv`
  Machine-friendly table for post-processing
- `logs/<step>.log`
  Full stdout/stderr for each step

Git ignores:

- `integration/.artifacts/`
- `integration/.t/`
- `integration/reports/`

## Runner

Primary entrypoint:

```bash
./integration/run-local-suite.sh <profile> [step-regex]
```

Examples:

```bash
./integration/run-local-suite.sh quick
```

```bash
./integration/run-local-suite.sh full
```

```bash
./integration/run-local-suite.sh unit 'sdk-go|sdk-rust|example-go'
```

```bash
./integration/run-local-suite.sh full 'integration-|grace-op'
```

## Profiles

### `quick`

Fast Go-first regression loop:

- `grace-op-unit`
- `sdk-go-unit`
- `example-go-unit`
- `integration-short`

Use this after most agent changes.

### `unit`

Runs native unit suites across:

- `holons/grace-op`
- SDKs:
  `c`, `cpp`, `csharp`, `dart`, `go`, `java`, `js`, `js-web`, `kotlin`,
  `ruby`, `rust`, `swift`
- examples:
  `c`, `cpp`, `csharp`, `dart`, `go`, `java`, `kotlin`, `node`, `python`,
  `ruby`, `rust`, `swift`

This is the native-unit layer only.

### `integration`

Runs only the real-binary black-box suite:

```bash
cd integration/tests
go test -count=1 -timeout 30m ./...
```

### `full`

Runs:

1. `unit`
2. then `integration`

This is the intended broad local regression pass before merge or release.

## Step Filters

The second argument is an optional regex applied to step ids.

Examples:

```bash
./integration/run-local-suite.sh unit 'sdk-js|sdk-js-web|example-node'
```

```bash
./integration/run-local-suite.sh full 'integration-|example-python'
```

This is the practical way to run “small parts” without creating more scripts.

List the built-in matrix:

```bash
./integration/run-local-suite.sh list
```

## Manual Narrow Runs

When you want to run a single slice without the umbrella runner, use the native
tool directly.

### `grace-op`

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/holons/grace-op
go test ./...
```

### Go SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/go-holons
go test ./...
```

### Rust SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/rust-holons
cargo test
```

### Swift SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/swift-holons
swift test
```

### Node SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/js-holons
npm test
```

### Dart SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/dart-holons
dart test
```

### Ruby SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/ruby-holons
bundle exec rake test
```

### Java SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/java-holons
gradle test
```

### Kotlin SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/kotlin-holons
gradle test
```

### C# SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed
dotnet test sdk/csharp-holons/csharp-holons.sln
```

### C SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/c-holons
make clean && make test && make clean
```

### C++ SDK

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/sdk/cpp-holons
make clean && make test && make clean
```

### Python Example

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/examples/hello-world/gabriel-greeting-python
python3 -m unittest api.public_test api.cli_test _internal.server_test
```

### Node Example

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/examples/hello-world/gabriel-greeting-node
npm test
```

### Rust Example

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/examples/hello-world/gabriel-greeting-rust
cargo test
```

### Black-box Integration Smoke

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/integration/tests
go test -short -v -count=1 -timeout 15m ./...
```

### Black-box Integration Full

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/integration/tests
go test -v -count=1 -timeout 30m ./...
```

### A small black-box slice

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/integration/tests
go test -v -run 'TestDispatch_|TestInvoke_|TestLifecycle_' -count=1 ./...
```

## Progress And Failure Handling

The umbrella runner prints live step progress:

```text
[03/27] RUN  sdk-rust-unit
[03/27] PASS sdk-rust-unit (12s)
```

If a toolchain is missing, the step is marked `SKIP` and the run continues.

If a step fails, the run continues so the report captures the whole visible
damage, then exits non-zero at the end.

## Prerequisites

This runner does **not** bootstrap every ecosystem for you.

It assumes the machine already has the relevant language toolchains and that any
required package restores have already been done for the modules you care about.
Examples:

- Node projects may need `npm install`
- Ruby projects may need `bundle install`
- Dart projects may need `dart pub get`
- .NET, Gradle, Swift, Cargo, and Go need their normal toolchains present

Missing binaries are reported as `SKIP`. Missing project dependencies usually
show up as `FAIL` in the relevant step log.

## Recommended Local Loop

After an agent change:

1. run `./integration/run-local-suite.sh quick`
2. if the change touches multiple layers, run `./integration/run-local-suite.sh full`
3. inspect the newest `integration/reports/<timestamp>/summary.md`
4. use `logs/*.log` as the factual memory surface for the next fix iteration

That loop is the strategic purpose of this directory.
