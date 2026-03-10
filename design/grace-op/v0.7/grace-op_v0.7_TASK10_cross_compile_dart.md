# TASK10 — Dart/Flutter Cross-Compilation

## Objective

Implement cross-compilation support in the `flutter` runner for
iOS/Android (Flutter) and browser (`dart compile js`).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `dart-holons`: `github.com/organic-programming/dart-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Per-Language Tooling (Dart row)

## Scope

### Framework mode

- `--target ios` → `flutter build ios-framework`
- `--target android` → `flutter build aar`
- Output: `.xcframework` (iOS) or `.aar` (Android)

### WASM/JS mode

- `--target wasm` → `dart compile js` or `flutter build web`
- Output: JS bundle + assets

## Acceptance Criteria

- [ ] `op build --target ios` → produces iOS framework
- [ ] `op build --target android` → produces `.aar`
- [ ] `op build --target wasm` → produces web build
- [ ] Verify with `op run` (desktop) — no regression
- [ ] `dart test` / `flutter test` — zero failures

## Dependencies

TASK02.
