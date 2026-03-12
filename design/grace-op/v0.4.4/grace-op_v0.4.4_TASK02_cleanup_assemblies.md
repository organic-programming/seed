# TASK02 — Remove Hand-Rolled Codesign from Assemblies

## Objective

Remove the duplicated `exec: codesign` step from all SwiftUI
assembly manifests. The recipe runner now handles it.

## Changes

Remove the `exec` step containing `codesign --force --deep`
from these assemblies:

1. `gudule-greeting-swiftui-go/holon.yaml`
2. `gudule-greeting-swiftui-rust/holon.yaml`
3. `gudule-greeting-swiftui-dart/holon.yaml`
4. `gudule-greeting-swiftui-kotlin/holon.yaml`
5. `gudule-greeting-swiftui-swift/holon.yaml`
6. `gudule-greeting-swiftui-csharp/holon.yaml`
7. `gudule-greeting-swiftui-node/holon.yaml`
8. `gudule-greeting-swiftui-python/holon.yaml`

Also update legacy recipes:
9. `go-swift-holons/examples/greeting/holon.yaml`
10. `rust-swift-holons/examples/greeting/holon.yaml`

Keep the `assert_file` on `_CodeSignature/CodeResources` —
it verifies the runner did its job.

## Acceptance Criteria

- [ ] All 10 manifests updated (exec codesign removed)
- [ ] `op build` still produces signed bundles
- [ ] `assert_file` on CodeResources still passes
- [ ] No bash codesign invocation left in any assembly

## Dependencies

TASK01 (recipe runner must auto-sign first).
