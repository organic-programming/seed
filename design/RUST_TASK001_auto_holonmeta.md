# RUST_TASK001 — Auto HolonMeta.Describe in serve runner

## Context

go-holons `serve.Run()` automatically calls `describe.Register(s, protoDir, holonYamlPath)`,
which registers the `HolonMeta.Describe` RPC without any holon developer action.
rust-holons `serve::run()` does not — Rust holons are invisible to introspection.

## Goal

When `serve::run()` starts, auto-register `HolonMeta.Describe` if
`holon.yaml` and `protos/` exist in the working directory. Match Go behavior exactly.

## Files to modify

### `src/serve.rs`

Add at startup (before `Server::builder()`):
```rust
if Path::new("holon.yaml").exists() {
    let desc_service = describe::build_service("./protos", "./holon.yaml")?;
    builder = builder.add_service(desc_service);
}
```

### `src/describe.rs` — add `build_service()` function

Currently `describe.rs` can parse protos and build `DescribeResponse`.
Add a function that returns a `tonic` service implementing the
`HolonMeta` gRPC interface, ready for `Server::add_service()`.

## Checklist

- [ ] Add `build_service(proto_dir, holon_yaml_path) → impl Service` to `describe.rs`
- [ ] Call `build_service` in `serve::run()` when `holon.yaml` exists
- [ ] Add test: serve with holon.yaml → Describe RPC responds
- [ ] Add test: serve without holon.yaml → no error, Describe not registered

## Dependencies

- None — self-contained
