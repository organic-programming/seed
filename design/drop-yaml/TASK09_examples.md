# TASK09 — Active examples → proto-only

## Scope

`examples/hello-world/` only. Do **NOT** touch `examples/legacy/`.

## Changes

### `gabriel-greeting-app-swiftui/`

- `HolonProcess.swift` (~L598): remove code that writes `holon.yaml` to staged directories.
- `EmbeddedSwiftMemHolon.swift` (~L26): remove `holonYAMLPath` parameter.
- `README.md` (~L24): remove mention of `holon.yaml`.
- `DECISIONS.md` (~L34): update rationale to reference `holon.proto` only.

### `gabriel-greeting-c/`

- `internal/server.c` (~L135): remove `holon.yaml` fallback in path resolution.

## Depends on

TASK07 (Swift SDK changes may affect `holonYAMLPath` API used by SwiftUI example).

## Verification

```bash
op build gabriel-greeting-app-swiftui
op build gabriel-greeting-c
grep -rn "holon\.yaml" examples/hello-world/
# Must return zero results.
```
