# TASK07 — JavaScript: `js-mem-ping-pong`

## Objective

Create the JavaScript implementation under `examples/mem-ping-pong/js/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Node.js.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/js-hello-world/`

## Scope

- Add the JavaScript implementation under `examples/mem-ping-pong/js/`
- Import and use `js-holons` directly in the example
- Keep the example idiomatic for the existing Node example structure
- Host logical `ping` and `pong` holons in one Node.js process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use the JavaScript SDK composition path over `mem://`
- Execute the canonical 1000-turn party
- Print the canonical JSON timing report
- Add README and `npm test` coverage

## Acceptance Criteria

- [ ] `npm test` passes in the example
- [ ] Running the example prints JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the JavaScript
      implementation
- [ ] The test verifies the exact `1000`-turn contract
- [ ] The example uses the SDK `connect` or official mem composition
      path rather than a raw ad hoc gRPC dial
- [ ] The example remains same-process throughout the run
- [ ] README explains install, run, and test

## Dependencies

v0.5 transport completion.
