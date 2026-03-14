# TASK04 — Dart: `dart-mem-ping-pong`

## Objective

Create the Dart implementation under `examples/mem-ping-pong/dart/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Dart.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/dart-hello-world/`

## Scope

- Add the Dart implementation under `examples/mem-ping-pong/dart/`
- Import and use `dart-holons` directly in the example
- Keep the example idiomatic for Dart package layout
- Host logical `ping` and `pong` holons inside one Dart process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use the Dart SDK `connect` path for peer resolution over `mem://`
- Run the canonical 1000-turn party
- Print the canonical JSON timing report
- Add README and `dart test` coverage

## Acceptance Criteria

- [ ] `dart test` passes in the example directory
- [ ] `dart run ...` completes successfully and prints JSON with
      `"transport":"mem://"` and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Dart
      implementation
- [ ] The test verifies the exact `1000`-turn contract
- [ ] The implementation uses SDK composition rather than a raw dial
- [ ] README explains dependency install, run, and test

## Dependencies

v0.5 transport completion.
