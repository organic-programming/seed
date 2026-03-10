# TASK04 — `requires.sources` in `holon.yaml`

## Objective

Add `requires.sources` to `holon.yaml` for declaring external
source dependencies that must be cloned and compiled.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_setup.md](./DESIGN_setup.md) — §Source Compilation

## Scope

### Schema

```yaml
requires:
  commands: [cmake, make]
  sources:
    - name: whisper.cpp
      repo: https://github.com/ggerganov/whisper.cpp
      ref: v1.5.4
      build: cmake
    - name: ffmpeg
      repo: https://github.com/FFmpeg/FFmpeg
      ref: a1b2c3d4e5f6
      build: configure-make
      configure_args: [--enable-gpl, --enable-libx264]
```

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `requires.sources` | list | no | Source dependencies to clone + build |
| `[].name` | string | yes | Human-readable name |
| `[].repo` | string | yes | Git repository URL |
| `[].ref` | string | yes | Git tag or commit SHA |
| `[].build` | string | yes | Build system: `cmake`, `configure-make`, `cargo`, `go` |
| `[].configure_args` | list | no | `./configure` arguments |

### Pinning rule

- Tags (`v1.5.4`) and commit SHAs accepted
- Floating branches (`master`) → `op check` warning

### Cache

- `~/.op/cache/sources/<name>/` — cloned once, reused

## Acceptance Criteria

- [ ] Schema parsed in manifest struct
- [ ] `op check` validates pinning rule
- [ ] `op setup` uses sources for installation
- [ ] Cache directory used correctly
- [ ] `go test ./...` — zero failures

## Dependencies

TASK01 (parser foundation).
