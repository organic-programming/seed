# TASK04 — Rust Cross-Compilation (cargo targets + wasm-pack)

## Objective

Implement cross-compilation support in the `cargo` runner for
iOS/Android (cargo + target triples) and browser (wasm-pack).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `rust-holons`: `github.com/organic-programming/rust-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Per-Language Tooling (Rust row)

## Scope

### Framework mode

- `--target ios` → `cargo build --target aarch64-apple-ios`
- `--target android` → `cargo build --target aarch64-linux-android`
- Output: `.a` / `.so` static/shared library

### WASM mode

- `--target wasm` → `wasm-pack build`
- Output: `.wasm` + JS bindings

## Acceptance Criteria

- [ ] `op build --target ios` → produces iOS library
- [ ] `op build --target android` → produces Android library
- [ ] `op build --target wasm` → produces `.wasm` via wasm-pack
- [ ] Verify with `op run` (desktop) to confirm no regression
- [ ] `cargo test` — zero failures

## Dependencies

TASK02.
