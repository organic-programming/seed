# HOLON_YAML.md — `requires.sources` Additions (Draft)

> These fields are designed to be added to `HOLON_YAML.md` within the
> existing `requires` field documentation.

---

## Source Dependencies (`requires.sources`)

Declare external source dependencies that must be cloned and built
before the holon can compile.

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
| `requires.sources` | list | no | External source dependencies |
| `[].name` | string | yes | Human-readable name |
| `[].repo` | string | yes | Git repository URL |
| `[].ref` | string | yes | Git tag or commit SHA |
| `[].build` | string | yes | Build system: `cmake`, `configure-make`, `cargo`, `go` |
| `[].configure_args` | list of string | no | Arguments for `./configure` (configure-make only) |

### Pinning rule

`ref` accepts:
- Git tags: `v1.5.4`
- Commit SHAs: `a1b2c3d4e5f6` (full or abbreviated)

Floating branches (`master`, `main`) are rejected by `op check`
with an actionable warning. Pinned references ensure reproducible
builds.

### Cache

Cloned sources are cached in `~/.op/cache/sources/<name>/`.
The cache is reused across `op setup` runs — a clone happens once,
subsequent runs check out the specified ref.

### Build systems

| Value | Steps |
|---|---|
| `cmake` | `cmake -S . -B build && cmake --build build` |
| `configure-make` | `./configure <args> && make && make install` |
| `cargo` | `cargo build --release` |
| `go` | `go build ./...` |
