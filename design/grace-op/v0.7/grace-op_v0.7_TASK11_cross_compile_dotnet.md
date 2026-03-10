# TASK11 — C# Cross-Compilation (.NET MAUI + Blazor WASM)

## Objective

Implement cross-compilation support in the `dotnet` runner for
iOS/Android (.NET MAUI) and browser (Blazor WASM).

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`
- `dotnet-holons`: `github.com/organic-programming/dotnet-holons`

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — §Per-Language Tooling (C# row)

## Scope

### Framework mode

- `--target ios` → `dotnet build -r ios-arm64`
- `--target android` → `dotnet build -r android-arm64`
- Output: MAUI framework library

### WASM mode

- `--target wasm` → Blazor WASM publish
- Output: `.wasm` + Blazor runtime

## Acceptance Criteria

- [ ] `op build --target ios` → produces iOS artifact
- [ ] `op build --target android` → produces Android artifact
- [ ] `op build --target wasm` → produces Blazor WASM bundle
- [ ] Verify with `op run` (desktop) — no regression
- [ ] `dotnet test` — zero failures

## Dependencies

TASK02.
