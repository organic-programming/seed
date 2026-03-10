# OP_run_TASK001 — `op run` (build-if-needed + launch)

## Objective

Make `op run <slug>` the single entry point for running any holon —
service, composite, or application — regardless of its build system
or complexity. Like `go run`, it builds if needed, then launches.

## Design Principle

`op run` does not care what the holon is. It reads the manifest,
ensures the artifact exists, and launches it. `connect()` handles
the rest organically — if the launched holon needs other holons,
it discovers and spawns them.

```
op run rob-go                     → service: build, serve --listen tcp://:9090
op run gudule-greeting-godart     → composite: build all, launch Flutter UI
op run wisupaa-whisper             → native: build, serve
op run my-10-holon-stack           → composite: build, launch entry point
                                     → connect() bootstraps 9 dependencies
```

## Current state

`cmdRun` (commands.go:272) only handles: `op run <slug>:<port>` or
`op run <slug> --listen <URI>`. It resolves the binary and starts
`<binary> serve --listen <URI>`. No auto-build. No composites.

## Proposed behavior

```
op run <slug> [flags]

  0. Resolve binary via OPBIN / PATH → if found, skip to step 4
     (no source code needed — pre-built or installed holons run immediately)
  1. Discover holon by slug → read holon.yaml
  2. Determine artifact path:
     - kind: service    → artifacts.binary
     - kind: composite  → artifacts.primary
  3. If artifact does not exist → op build <slug> (recursive)
  4. Launch:
     - service          → <binary> serve --listen <URI>  (foreground)
     - composite        → exec <primary artifact>        (foreground)
  5. On SIGINT/SIGTERM → graceful shutdown
     - For composites: the frontend exits, connect()
       handles cleanup of spawned dependencies
```

> [!IMPORTANT]
> Source distribution is not a prerequisite. If a holon is already
> installed (`op install`, package manager, manual copy to `$OPBIN`),
> `op run <slug>` works immediately — no source tree, no build step.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--listen <URI>` | `stdio://` | Listen address (services only) |
| `--no-build` | false | Skip auto-build, fail if artifact missing |
| `--target <os>` | current OS | Passed to `op build` if building |
| `--mode <debug\|release>` | `debug` | Passed to `op build` if building |

### Backward compatibility

Current syntax `op run <slug>:<port>` remains as shorthand for
`op run <slug> --listen tcp://:<port>`.

## What makes this work

This is simple because of two things already in place:

1. **`op build`** already knows how to build anything — single holons,
   composites with nested members. It handles runner dispatch (go,
   cargo, flutter, cmake, etc.) recursively.

2. **`connect()`** already knows how to bootstrap dependencies.
   When the launched holon calls `connect("other-slug")`, the SDK
   discovers, starts, and wires the dependency. Recursively. No
   orchestration needed in `op run`.

`op run` is just the bridge: ensure the artifact exists, then exec it.

## Implementation

### Changes to `grace-op`

#### `internal/cli/commands.go` — rewrite `cmdRun`

```go
func cmdRun(format Format, quiet bool, args []string) int {
    // 1. Parse slug + flags (--listen, --no-build, --target, --mode)
    // 2. Resolve holon → read manifest
    // 3. Determine artifact path from kind
    // 4. If artifact missing && !noBuild → cmdLifecycle(Build, slug)
    // 5. If still missing → error
    // 6. Launch:
    //    - service → exec.Command(binary, "serve", "--listen", listenURI)
    //    - composite → exec.Command(primaryArtifact)
    // 7. cmd.Run() (foreground, inherits stdin/stdout/stderr)
}
```

Key difference from today: **foreground, not detached.** The user runs
`op run my-app`, sees the output, Ctrl+C to stop. Like `go run`.

#### `internal/holons/manifest.go` — add `ArtifactPath()` helper

```go
func (m *Manifest) ArtifactPath() string {
    if m.Artifacts.Primary != "" {
        return m.Artifacts.Primary
    }
    return m.Artifacts.Binary
}
```

### No changes to the Runner interface

`op run` is NOT a runner operation. Runners handle `Check/Build/Test/Clean`.
`op run` is a CLI-level orchestration: build (via runners) + launch.

## Interaction with other OP_TASKs

| Task | Relationship |
|------|-------------|
| OP_TASK001–003 (runners) | `op run` calls `op build` which dispatches to runners. No runner changes needed. |
| OP_TASK004 (mesh) | Unrelated. Mesh is about distributed topology; `op run` is local. |
| OP_TASK005 (distribution) | Complementary. Once `op` is installed via Homebrew/npm, `op run` is the first thing a user runs. |
| OP_TASK006 (setup) | `op setup` provisions the host; `op run` launches what's there. Sequential, not conflicting. |
| OP_TASK007 (composition recipes) | `op run` is how users launch composition recipes. Pipeline/fan-out orchestrators bootstrap via `connect()`. |

## Dependencies

- Depends on OP_TASK001 (runner registry) for full runner coverage. But
  `op run` can be implemented now for `go-module` and `recipe` runners
  which already exist.

## Checklist

- [ ] Rewrite `cmdRun` to detect holon kind and auto-build
- [ ] Add `ArtifactPath()` helper to manifest
- [ ] Support foreground execution (not detached)
- [ ] Preserve backward compat for `op run <slug>:<port>` syntax
- [ ] Add `--no-build` flag
- [ ] Pass `--target` and `--mode` to `op build` when auto-building
- [ ] Test: `op run <service-slug>` → builds + serves
- [ ] Test: `op run <composite-slug>` → builds all + launches primary
- [ ] Test: `op run <slug>` with artifact already built → skips build
- [ ] Test: `op run --no-build <slug>` with missing artifact → error
- [ ] `go test ./...` — zero failures
