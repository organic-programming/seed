# TASK07 — swift-holons SDK → proto-only

## Scope

`sdk/swift-holons/`

## Changes

### `Sources/Holons/Describe.swift`
- Remove `holonYAMLPath` parameter from `buildDescribeResponse()` and `DescribeService.init()`.
- Identity must be read from proto only.

### `Sources/Holons/Serve.swift`
- Remove `holonYAMLPath` from `Options` struct.
- Remove YAML-based describe registration path. Keep proto-only path.

### `Sources/Holons/Discover.swift`
- Remove `holon.yaml` filename check. Scan for `holon.proto` only.

### `Sources/Holons/Identity.swift`
- Remove YAML-based `parseHolon()` if it reads YAML. Replace with proto-based parsing.

### Tests
- `DescribeTests.swift`, `ServeTests.swift`, `ConnectTests.swift`, `HolonsTests.swift`: update all to use `holon.proto`.

### Dependencies
- Remove `Yams` from `Package.swift` if no other code uses it.

### `README.md`
- Update description.

## Verification

```bash
cd sdk/swift-holons && swift build && swift test
```
