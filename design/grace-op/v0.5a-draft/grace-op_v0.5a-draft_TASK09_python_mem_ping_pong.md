# TASK09 — Python: `python-mem-ping-pong`

## Objective

Create the Python implementation under `examples/mem-ping-pong/python/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Python.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/python-hello-world/`

## Scope

- Add the Python implementation under `examples/mem-ping-pong/python/`
- Import and use `python-holons` directly in the example
- Keep the example idiomatic for the current Python example layout
- Host logical `ping` and `pong` holons in one Python process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use the Python SDK `connect()` path over `mem://`
- Execute the canonical 1000-turn party
- Print the canonical JSON timing report
- Add README and `pytest` coverage

## Acceptance Criteria

- [ ] `python3 -m pytest -v` passes in the example
- [ ] Running the example prints JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Python
      implementation
- [ ] The example remains same-language and same-process end to end
- [ ] The test verifies the exact `1000`-turn contract
- [ ] The implementation does not replace SDK composition with a raw
      grpcio channel in the user-facing flow
- [ ] README explains setup, run, and test

## Dependencies

v0.5 transport completion.
