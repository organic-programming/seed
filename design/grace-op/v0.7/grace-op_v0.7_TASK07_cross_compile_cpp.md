# TASK07 — C/C++ Cross-Compilation (NDK + Emscripten)

## Objective

Implement cross-compilation support in the `cmake` / `qt-cmake`
runners for iOS/Android (NDK, Xcode) and browser (Emscripten).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `cpp-holons`: `github.com/organic-programming/cpp-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Per-Language Tooling (C/C++ row)

## Scope

### Framework mode

- `--target ios` → Xcode toolchain, `CMAKE_SYSTEM_NAME=iOS`
- `--target android` → NDK toolchain, `CMAKE_SYSTEM_NAME=Android`
- Output: `.framework` (iOS) or `.so` (Android)

### WASM mode

- `--target wasm` → Emscripten toolchain (`emcmake cmake`)
- Output: `.wasm` + JS glue

## Acceptance Criteria

- [ ] `op build --target ios` → produces iOS artifact
- [ ] `op build --target android` → produces Android artifact
- [ ] `op build --target wasm` → produces `.wasm` via Emscripten
- [ ] Verify with `op run` (desktop) — no regression
- [ ] Tests pass

## Dependencies

TASK02.
