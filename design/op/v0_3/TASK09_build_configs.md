# TASK09 — Build Configs (`--config` + `OP_CONFIG`)

## Context

OP.md §4, §7, §8 and OP_BUILD_SPEC §4 specify a build configuration
mechanism. It is fully designed but has zero implementation.

No dependency on TASK01–08. Future runners (TASK01–03) will inherit
`OP_CONFIG` support once this task is done.

## Objective

Allow holons to declare named build variants in `holon.yaml` and
let the user select one with `op build --config <name>`.

## Changes

### 1. Manifest parser (`internal/holons/manifest.go`)

Add to the `Build` struct:

```go
Configs       map[string]BuildConfig `yaml:"configs,omitempty"`
DefaultConfig string                 `yaml:"default_config,omitempty"`
```

```go
type BuildConfig struct {
    Description string `yaml:"description,omitempty"`
}
```

Validation in `op check`:
- If `configs` is set, `default_config` must reference an existing key.
- If `configs` is empty, `default_config` must be absent.

### 2. CLI flag (`internal/cli/lifecycle.go`)

Parse `--config` in `cmdLifecycle`, store in `BuildOptions`:

```go
case args[i] == "--config" && i+1 < len(args):
    opts.Config = args[i+1]
    i++
```

If `--config` is provided but the manifest has no `build.configs`,
fail with an actionable error.

If `--config` is omitted and `build.configs` exists, use
`build.default_config`.

### 3. Runner injection (`internal/holons/lifecycle.go`)

For `go-module`: set `OP_CONFIG` in `cmd.Env` during build and test.

For `cmake`: append `-DOP_CONFIG=<config>` to the configure argv.

For `recipe`: propagate `--config` to `build_member` child builds;
allow per-member `config:` override in recipe steps.

### 4. Report struct (`internal/holons/lifecycle.go`)

Add `BuildConfig string` to the `Report` struct:

```go
BuildConfig string `json:"build_config,omitempty"`
```

Populate it after config resolution.

## Acceptance Criteria

- [ ] `holon.yaml` with `build.configs` + `build.default_config` parses
- [ ] `op check` validates config consistency
- [ ] `op build --config gpl` sets `OP_CONFIG=gpl` for go-module
- [ ] `op build --config gpl` passes `-DOP_CONFIG=gpl` for cmake
- [ ] `op build` without `--config` uses `default_config`
- [ ] `op build --config unknown` fails with actionable error
- [ ] Report includes `build_config` field
- [ ] `--dry-run` shows resolved config
- [ ] `go test ./...` — zero failures

## Dependencies

None. Independent of TASK01–08.
