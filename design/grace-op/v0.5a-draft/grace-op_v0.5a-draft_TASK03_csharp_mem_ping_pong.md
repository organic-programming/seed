# TASK03 — C#: `csharp-mem-ping-pong`

## Objective

Create the C# implementation under `examples/mem-ping-pong/csharp/`
to validate that `op` can launch same-language, same-process
`mem://` composition in C#.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/csharp-hello-world/`

## Scope

- Add the C# implementation under `examples/mem-ping-pong/csharp/`
- Import and use `csharp-holons` directly in the example
- Follow the existing solution-and-test-project style used by the C#
  examples
- Host logical `ping` and `pong` holons in one .NET process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Reach the peer through the C# SDK connect path, not direct channel
  construction
- Execute the canonical party with final value `1000`
- Emit the canonical JSON timing report
- Add README and test project coverage

## Acceptance Criteria

- [ ] `dotnet test` passes for the example
- [ ] A runnable entry point produces JSON with
      `"transport":"mem://"` and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the C#
      implementation
- [ ] The test verifies the 1000-turn rule
- [ ] The example remains same-process and same-language end to end
- [ ] README explains restore, run, and test

## Dependencies

v0.5 transport completion.
