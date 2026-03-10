# TASK14 — Desktop Cross-Compilation (windows / linux / macos)

## Objective

Support `op build --target windows`, `--target linux`, and
`--target macos` for cross-compiling desktop binaries from
any host platform.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §CLI Interface (`--target windows`)

## Scope

### Per-language cross-compile

| Language | Tool |
|---|---|
| Go | `GOOS=<os> GOARCH=<arch> go build` |
| Rust | `cargo build --target <triple>` |
| C/C++ | Cross-toolchain CMake (`CMAKE_SYSTEM_NAME`) |
| Others | Where supported by native toolchain |

### Runner additions

- `--target windows` → set OS/arch for target platform
- `--target linux` → same
- `--target macos` → same
- Runners resolve the correct toolchain settings per language

### Manifest

```yaml
build:
  targets:
    windows:
      mode: binary
      env:
        GOOS: windows
        GOARCH: amd64
```

## Acceptance Criteria

- [ ] `op build --target windows` from macOS → produces `.exe`
- [ ] `op build --target linux` from macOS → produces ELF binary
- [ ] Each compiled-language runner handles desktop targets
- [ ] Interpreted languages (Node, Python) → clear "not applicable" error
- [ ] `go test ./...` — zero failures

## Dependencies

TASK02.
