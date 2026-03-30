⚠️ Agents must not read this document without an explicit invitation!

# `op run`
This file is a quick book for human developers and testers.



# How `op run` works

1. **Global flags parsed** in [parseGlobalOptions](./api/cli.go#L205-L256) — extracts `--format`, `--quiet`, `--root`, `--bin` before the subcommand.
2. **Dispatch** — `"run"` case in [run()](./api/cli.go#L104-L105) calls `runRunCommand`.
3. **Flag parsing** — [parseRunArgs](./api/cli_lifecycle.go#L208-L243) extracts `--listen` (default `tcp://127.0.0.1:0`), `--no-build`, `--target`, `--mode`.
4. **Request resolution** — [resolveRunRequest](./api/run_helpers.go#L296-L308) builds the `RunRequest` protobuf.
5. **Execution** — [runWithIO](./api/run_helpers.go#L34-L157) does the heavy lifting:
   - Resolves holon target via [ResolveTarget](./api/run_helpers.go#L84)
   - Checks installed binary via [ResolveInstalledBinary](./api/run_helpers.go#L88)
   - Auto-builds if artifact missing (unless `--no-build`)
   - Constructs the final command via [commandForArtifact](./api/run_helpers.go#L189-L225) → `<binary> serve --listen <uri>`
   - Runs in foreground with signal forwarding via [runForeground](./api/run_helpers.go#L243-L271)

Canonical form : `op run <holon> [flags]`

## Run flags

| Flag | Value | Default | Description |
|---|---|---|---|
| `--listen` | `<uri>` | `tcp://127.0.0.1:0` | Transport listen URI |
| `--no-build` | — | `false` | Skip auto-build before running |
| `--target` | `<value>` | `""` | Build target |
| `--mode` | `<value>` | `""` | Build mode |

## Global flags

| Flag | Value | Description |
|---|---|---|
| `--format` | `json` | Output in JSON format |
| `--quiet` / `-q` | — | Suppress progress output |
| `--root` | `<path>` | Override discovery root |
| `--bin` | — | Print resolved binary path and exit |

### --format 
### --quiet
### --root 
### --bin

## Default behaviour `op run gabriel-greeting-go`
`op run gabriel-greeting-go` == `op run gabriel-greeting-go --listen tcp://127.0.0.1:0`

## `op run gabriel-greeting-go` != `op gabriel-greeting-go`
`op gabriel-greeting-go` requires a second arg (a method name). Without one it errors: missing command for holon "gabriel-greeting-go"

## tcp 

```shell
 op run gabriel-greeting-go --listen tcp://127.0.0.1:60001
 ```
---


----

# Useful commands: 

## Reinstall a fresh op : 

```shell
op install op --build
```

## Kill a holon bin: 

```shell
pkill -9 -f gabriel-greeting-go
```

## tcp: 
```shell
# Find the PID
lsof -ti tcp:60001

# Kill it
kill $(lsof -ti tcp:60001)

# Or force-kill if needed
kill -9 $(lsof -ti tcp:60001)
```


