# TASK09 — Kotlin Cross-Compilation (Gradle + Kotlin/JS)

## Objective

Implement cross-compilation support in the `gradle` runner for
Android (native) and browser (Kotlin/JS, limited).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `kotlin-holons`: `github.com/organic-programming/kotlin-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Per-Language Tooling (Kotlin row)

## Scope

### Framework mode

- `--target android` → Gradle Android library plugin
- Output: `.aar`

### WASM/JS mode (limited)

- `--target wasm` → Kotlin/JS compiler
- Output: JS bundle (Kotlin/WASM is maturing)
- Mark as limited in docs

## Acceptance Criteria

- [ ] `op build --target android` → produces `.aar`
- [ ] `op build --target wasm` → produces JS/WASM bundle
- [ ] Verify with `op run` (desktop) — no regression
- [ ] `gradle test` — zero failures

## Dependencies

TASK02.
