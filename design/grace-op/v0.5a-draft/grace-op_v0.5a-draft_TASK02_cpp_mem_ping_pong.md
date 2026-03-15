# TASK02 — C++: `cpp-mem-ping-pong`

## Objective

Create the C++ implementation under `examples/mem-ping-pong/cpp/` to
validate that `op` can launch same-language, same-process `mem://`
composition in C++.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/cpp-hello-world/`

## Scope

- Add the C++ implementation under `examples/mem-ping-pong/cpp/`
- Import and use `cpp-holons` directly in the example
- Keep the example idiomatic for the current CMake-based C++ examples
- Host two logical C++ holons in one OS process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use the C++ SDK's `mem://` listener and official connect path
- Implement the canonical 1000-turn ping-pong party
- Print the canonical JSON timing report
- Add README and automated coverage

## Acceptance Criteria

- [ ] `cmake -S . -B build && cmake --build build` succeeds in the
      example directory
- [ ] `ctest --test-dir build` includes a passing ping-pong validation
- [ ] The runtime report contains `"transport":"mem://"` and
      `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the C++
      implementation
- [ ] The validation asserts the exact `1000`-turn contract
- [ ] No raw gRPC dial bypass is used in place of SDK composition
- [ ] README explains build, run, and test

## Dependencies

v0.5 transport completion.
