# RUST_TASK001 — Auto HolonMeta.Describe in serve runner

Status: Complete on `op-v0.4-dev`

## Context

go-holons `serve.Run()` automatically calls `describe.Register(s, protoDir, holonYamlPath)`,
which registers the `HolonMeta.Describe` RPC without any holon developer action.
rust-holons initially did not — Rust holons were invisible to introspection.

## Goal

When the Rust serve runner starts, auto-register `HolonMeta.Describe`
from the current holon root. `holon.yaml` is required; `protos/` is
consumed when present. This is the readiness contract expected by the
other SDKs and desktop HostUIs.

## Files to modify

### `src/describe.rs`

`describe::service(proto_dir, holon_yaml_path)` returns the
manually-registrable `HolonMetaServer<MetaService>` used by the serve
runner.

### `src/serve.rs`

`serve::run_single_with_options()` and `serve::run_with_options()`
both call an internal auto-registration helper before the primary
service is attached. The helper:
- checks for `./holon.yaml`
- uses `./protos` as the proto root
- attaches `describe::service(...)` when the holon manifest exists
- stays quiet when the holon manifest is absent

## Checklist

- [x] Expose a manually registrable `describe::service(proto_dir, holon_yaml_path)`
- [x] Auto-register HolonMeta in `run_single_with_options()`
- [x] Auto-register HolonMeta in `run_with_options()`
- [x] Add a serve-runner regression: serve with `holon.yaml` → `Describe` responds
- [x] Preserve the no-manifest path: serve without `holon.yaml` does not fail
- [x] Validate the real desktop path: SwiftUI + Rust assembly now starts successfully

## Dependencies

- None — self-contained

## Notes

- This task was closed out as part of the SwiftUI + Rust startup fix.
- The regression lives in the Rust SDK test suite and exercises
  `HolonMeta.Describe` through the actual serve runner instead of
  registering the meta service manually in the test.
