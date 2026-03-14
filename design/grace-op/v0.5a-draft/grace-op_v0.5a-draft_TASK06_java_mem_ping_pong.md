# TASK06 — Java: `java-mem-ping-pong`

## Objective

Create the Java implementation under `examples/mem-ping-pong/java/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Java.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/java-hello-world/`

## Scope

- Add the Java implementation under `examples/mem-ping-pong/java/`
- Import and use `java-holons` directly in the example
- Keep the example idiomatic for the current Gradle-based Java layout
- Host logical `ping` and `pong` holons in one JVM process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Reach the peer via the Java SDK `Connect.connect(...)` path over
  `mem://`
- Execute the canonical party with final value `1000`
- Print the canonical JSON timing report
- Add README and automated tests

## Acceptance Criteria

- [ ] `./gradlew test` passes in the example
- [ ] A runnable entry point emits JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Java
      implementation
- [ ] The test verifies the 1000-turn contract
- [ ] The example does not bypass SDK composition with a raw
      `ManagedChannel` dial in the user-facing path
- [ ] README explains build, run, and test

## Dependencies

v0.5 transport completion.
