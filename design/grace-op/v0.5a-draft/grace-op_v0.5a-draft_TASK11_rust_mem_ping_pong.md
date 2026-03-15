# TASK11 — Rust: `rust-mem-ping-pong`

## Objective

Create the Rust implementation under `examples/mem-ping-pong/rust/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Rust.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/rust-hello-world/`

## Scope

- Add the Rust implementation under `examples/mem-ping-pong/rust/`
- Import and use `rust-holons` directly in the example
- Keep the example idiomatic for the current Cargo workspace style
- Host logical `ping` and `pong` holons in one Rust process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use the Rust SDK `connect` path over `mem://`
- Execute the canonical 1000-turn party
- Print the canonical JSON timing report
- Add README and `cargo test` coverage

## Acceptance Criteria

- [ ] `cargo test` passes in the example
- [ ] Running the example prints JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Rust
      implementation
- [ ] The implementation stays inside same-process SDK composition
- [ ] The success path validates exact turn count and final value
- [ ] README explains build, run, and test

## Dependencies

v0.5 transport completion.
