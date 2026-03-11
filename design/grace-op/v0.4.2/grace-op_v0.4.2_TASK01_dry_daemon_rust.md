# TASK01 — Extract DRY Rust Daemon

## Summary

Extract the Rust daemon from `rust-dart-holons` into
`recipes/daemons/gudule-daemon-greeting-rust/`. Follows the pattern
validated by TASK04.

> [!IMPORTANT]
> **Always use the language SDK as much as possible.**
> The Rust daemon must use `rust-holons` SDK server primitives.

## Acceptance Criteria

- [ ] `gudule-daemon-greeting-rust` has its own `holon.yaml` + Rust source
- [ ] `family_name: Greeting-Daemon-Rust` / binary: `gudule-daemon-greeting-rust`
- [ ] Uses `recipes/protos/greeting/v1/greeting.proto` (shared, via `import public`)
- [ ] Builds standalone with `op build`
- [ ] Runs standalone with `op run`
- [ ] Supports macOS, Windows, Linux
- [ ] Uses `rust-holons` SDK for server bootstrap
- [ ] Existing submodule repos NOT modified

## Dependencies

TASK04 (PoC validated — pattern is proven).
