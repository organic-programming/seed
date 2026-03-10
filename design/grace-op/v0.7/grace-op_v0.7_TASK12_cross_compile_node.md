# TASK12 — Node.js Cross-Compilation (WASI)

## Objective

Implement cross-compilation support in the `npm` runner for
browser/edge (WASM via WASI or bundler).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `node-holons`: `github.com/organic-programming/node-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md)

## Scope

### WASM mode

- `--target wasm` → bundle for browser via WASI or bundler
- Node.js doesn't produce mobile frameworks — framework mode
  is not applicable

### Desktop cross-compile

- Not applicable (Node.js is interpreted, runs everywhere)

## Acceptance Criteria

- [ ] `op build --target wasm` → produces browser-ready bundle
- [ ] `--target ios` / `--target android` → clear "not supported" error
- [ ] Verify with `op run` (desktop) — no regression
- [ ] `npm test` — zero failures

## Dependencies

TASK02.
