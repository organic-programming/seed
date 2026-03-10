# TASK08 — Swift Cross-Compilation (Xcode + SwiftWasm)

## Objective

Implement cross-compilation support in the `swift-package` runner
for iOS (Xcode framework) and browser (SwiftWasm, experimental).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `swift-holons`: `github.com/organic-programming/swift-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Per-Language Tooling (Swift row)

## Scope

### Framework mode

- `--target ios` → `xcodebuild -destination 'generic/platform=iOS'`
- Output: `.xcframework`

### WASM mode (experimental)

- `--target wasm` → SwiftWasm toolchain
- Output: `.wasm`
- Mark as experimental in docs

## Acceptance Criteria

- [ ] `op build --target ios` → produces `.xcframework`
- [ ] `op build --target wasm` → produces `.wasm` (experimental)
- [ ] Verify with `op run` (desktop) — no regression
- [ ] `swift test` — zero failures

## Dependencies

TASK02.
