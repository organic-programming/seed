# TASK10 — Ruby: `ruby-mem-ping-pong`

## Objective

Create the Ruby implementation under `examples/mem-ping-pong/ruby/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Ruby.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/ruby-hello-world/`

## Scope

- Add the Ruby implementation under `examples/mem-ping-pong/ruby/`
- Import and use `ruby-holons` directly in the example
- Keep the example idiomatic for the current Ruby example structure
- Host logical `ping` and `pong` holons in one Ruby process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Use the Ruby SDK `connect` path over `mem://`
- Execute the canonical 1000-turn party
- Print the canonical JSON timing report
- Add README and automated coverage

## Acceptance Criteria

- [ ] A language-native Ruby test passes for the example
- [ ] Running the example prints JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Ruby
      implementation
- [ ] The implementation uses SDK composition rather than direct ad hoc
      gRPC channel wiring
- [ ] The 1000-turn rule is explicitly validated
- [ ] README explains bundle install, run, and test

## Dependencies

v0.5 transport completion.
