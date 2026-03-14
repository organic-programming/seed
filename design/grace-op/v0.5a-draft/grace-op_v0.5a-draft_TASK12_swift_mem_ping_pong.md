# TASK12 — Swift: `swift-mem-ping-pong`

## Objective

Create the Swift implementation under `examples/mem-ping-pong/swift/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Swift.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/swift-hello-world/`

## Scope

- Add the Swift implementation under `examples/mem-ping-pong/swift/`
- Import and use `swift-holons` directly in the example
- Keep the example idiomatic for the current Swift Package Manager
  structure
- Host logical `ping` and `pong` holons in one Swift process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use the Swift SDK `connect` path over `mem://`
- Execute the canonical 1000-turn party
- Print the canonical JSON timing report
- Add README and `swift test` coverage

## Acceptance Criteria

- [ ] `swift test` passes in the example
- [ ] Running the example prints JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Swift
      implementation
- [ ] The implementation remains same-process and same-language
- [ ] The test verifies the exact `1000`-turn contract
- [ ] SDK composition is used instead of a raw gRPC channel shortcut
- [ ] README explains build, run, and test

## Dependencies

v0.5 transport completion.
