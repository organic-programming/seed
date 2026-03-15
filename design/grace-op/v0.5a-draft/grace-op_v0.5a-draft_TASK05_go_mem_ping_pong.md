# TASK05 — Go: `go-mem-ping-pong`

## Objective

Create the Go implementation under `examples/mem-ping-pong/go/` as the **reference
implementation** for same-language, same-process `mem://`
composition in the v0.5a example fleet.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/go-hello-world/`

## Scope

- Add the Go implementation under `examples/mem-ping-pong/go/`
- Import and use `go-holons` directly in the example
- Reuse the normal Go example structure where helpful
- Host logical `ping` and `pong` holons in one Go process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use Go SDK `connect(slug)` composition over `mem://`
- Execute the canonical 1000-turn party
- Emit the canonical JSON timing report
- Add README and `go test ./...` coverage

## Acceptance Criteria

- [ ] `go test ./...` passes for the example
- [ ] Running the example prints JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Go
      implementation
- [ ] The measured timing uses a monotonic clock and surfaces
      `elapsed_ns`
- [ ] The test verifies the exact `1000`-turn contract
- [ ] The example serves as the pattern that the other language tasks
      follow for SDK usage, output shape, and test intent
- [ ] The example uses SDK composition rather than direct bufconn or
      raw gRPC wiring in the launcher path
- [ ] README explains build, run, and test

## Dependencies

v0.5 transport completion.
