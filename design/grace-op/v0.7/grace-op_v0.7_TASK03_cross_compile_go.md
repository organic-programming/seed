# TASK03 — Go Cross-Compilation (gomobile + WASM)

## Objective

Implement cross-compilation support in the `go-module` runner for
iOS/Android (gomobile) and browser (WASM) targets.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `go-holons`: `github.com/organic-programming/go-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Per-Language Tooling (Go row)

## Scope

### Framework mode (gomobile)

- `--target ios` → `gomobile bind -target ios`
- `--target android` → `gomobile bind -target android`
- Output: `.xcframework` (iOS) or `.aar` (Android)
- Verify gomobile is installed, suggest `go install` if missing

### WASM mode

- `--target wasm` → `GOOS=js GOARCH=wasm go build`
- Output: `.wasm` module
- Copy `wasm_exec.js` alongside the module

### Composite pass-through

- `kind: composite` with `--target` passes the flag to each
  member's runner (see §Runner Adaptation in DESIGN doc)

## Acceptance Criteria

- [ ] `op build --target ios` → produces `.xcframework`
- [ ] `op build --target android` → produces `.aar`
- [ ] `op build --target wasm` → produces `.wasm` + `wasm_exec.js`
- [ ] Composite holon: `--target` propagated to members
- [ ] Missing gomobile → helpful error
- [ ] `go test ./...` — zero failures

## Dependencies

TASK02 (`--target` flag must exist).
