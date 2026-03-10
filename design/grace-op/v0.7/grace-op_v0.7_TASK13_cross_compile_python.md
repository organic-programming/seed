# TASK13 — Python Cross-Compilation (Pyodide / WASM)

## Objective

Implement cross-compilation support in the Python runner for
browser (Pyodide / WASM).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `python-holons`: `github.com/organic-programming/python-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md)

## Scope

### WASM mode

- `--target wasm` → Pyodide bundle or CPython WASM build
- Experimental — Python WASM tooling is evolving

### Framework / Desktop

- Not applicable (Python is interpreted)

## Acceptance Criteria

- [ ] `op build --target wasm` → produces Pyodide bundle
- [ ] `--target ios` / `--target android` → clear "not supported" error
- [ ] Verify with `op run` (desktop) — no regression
- [ ] `pytest` — zero failures

## Dependencies

TASK02.
