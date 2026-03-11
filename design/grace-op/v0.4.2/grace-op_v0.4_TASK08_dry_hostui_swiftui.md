# TASK04 — Extract DRY HostUI: SwiftUI

## Summary

Extract the SwiftUI HostUI from `go-swift-holons` into
`recipes/hostui/gudule-greeting-hostui-swiftui/`. Follows the
connect-approach pattern validated by TASK04.

> [!IMPORTANT]
> **Connect approach only.** Must use `swift-holons` SDK
> `connect(slug)` — not raw gRPC channel creation.

> [!IMPORTANT]
> **Always use the language SDK as much as possible.**
> Use `swift-holons` / `grpc-swift` v2 SDK primitives.

## Acceptance Criteria

- [ ] `gudule-greeting-hostui-swiftui` has its own `holon.yaml` + Swift source
- [ ] `family_name: Greeting-Hostui-Swiftui` / binary: `gudule-greeting-hostui-swiftui`
- [ ] Uses `recipes/protos/greeting/v1/greeting.proto` (shared, via `import public`)
- [ ] Uses `swift-holons` SDK `connect(slug)` for daemon resolution
- [ ] Builds standalone (SPM / xcodebuild)
- [ ] Supports macOS, iOS
- [ ] Existing submodule repos NOT modified

## Dependencies

TASK07.
