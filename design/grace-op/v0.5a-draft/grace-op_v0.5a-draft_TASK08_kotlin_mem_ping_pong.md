# TASK08 — Kotlin: `kotlin-mem-ping-pong`

## Objective

Create the Kotlin implementation under `examples/mem-ping-pong/kotlin/`
to validate that `op` can launch same-language, same-process
`mem://` composition in Kotlin.

## Reference

- [DESIGN_mem_ping_pong.md](./DESIGN_mem_ping_pong.md)
- Existing scaffold: `examples/kotlin-hello-world/`

## Scope

- Add the Kotlin implementation under `examples/mem-ping-pong/kotlin/`
- Import and use `kotlin-holons` directly in the example
- Keep the example idiomatic for the current Gradle + Kotlin layout
- Host logical `ping` and `pong` holons in one JVM process
- Ensure `op` can launch the implementation as the user-facing entry
  point
- Reach peers through the Kotlin SDK `Connect.connect(...)` path over
  `mem://`
- Run the canonical 1000-turn party
- Emit the canonical JSON timing report
- Add README and automated tests

## Acceptance Criteria

- [ ] `./gradlew test` passes in the example
- [ ] A runnable entry point emits JSON with `"transport":"mem://"`
      and `"final_value":1000`
- [ ] The documented `op` launch path succeeds for the Kotlin
      implementation
- [ ] The implementation stays within same-language SDK composition
- [ ] The test verifies exact final value and turn count
- [ ] README explains build, run, and test

## Dependencies

v0.5 transport completion.
