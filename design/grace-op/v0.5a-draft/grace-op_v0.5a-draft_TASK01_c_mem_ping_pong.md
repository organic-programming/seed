# TASK01 — C: `c-mem-ping-pong`

## Objective

Create the C implementation under `examples/mem-ping-pong/c/` to
validate that `op` can launch same-language, same-process `mem://`
composition in C.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/c-hello-world/`

## Scope

- Add the C implementation under `examples/mem-ping-pong/c/`
- Import and use `c-holons` directly in the example
- Host two logical C holons, `ping` and `pong`, in one OS process
- Use `mem://` for both logical listeners
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Route peer calls through the official C SDK `connect` surface or the
  closest official C helper, not raw gRPC dialing
- Run the canonical party: start at `0`, perform `1000` total turns,
  finish at `1000`
- Emit the canonical JSON timing report to stdout
- Add a README and an automated test

## Acceptance Criteria

- [ ] `examples/mem-ping-pong/c/` builds with the language-native
      C toolchain and the repo's normal example conventions
- [ ] Running the example exits successfully and prints JSON with
      `"transport":"mem://"` and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the C
      implementation
- [ ] The example performs exactly `1000` turns
- [ ] The example never falls back to `tcp://`, `unix://`,
      `stdio://`, `ws://`, or `wss://`
- [ ] A language-native test covers the success path
- [ ] README explains build, run, and test

## Dependencies

v0.5 transport completion.
