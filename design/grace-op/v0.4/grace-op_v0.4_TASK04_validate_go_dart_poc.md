# TASK04 — Validate Go + Dart PoC Assembly

## Summary

Create a thin assembly manifest that combines `gudule-daemon-greeting-go`
(TASK02) + `gudule-greeting-hostui-flutter` (TASK03) and validate the
full cycle: build, launch, UI, RPC.

This assembly replaces the role of `go-dart-holons` but uses the
new DRY components. It proves the monorepo assembly pattern works
end-to-end before scaling to other languages.

## Acceptance Criteria

- [ ] `recipes/assemblies/gudule-greeting-flutter-go/holon.yaml` created
- [ ] `family_name: Greeting-Flutter-Go`
- [ ] `op build` succeeds (builds both daemon and Flutter app)
- [ ] `op run` succeeds (daemon starts, Flutter app connects via `connect(slug)`)
- [ ] GreetingService responds correctly through the Flutter UI
- [ ] Works on macOS, Linux, and Windows (same platforms as original)
- [ ] HostUI uses `connect(slug)` — NOT direct gRPC dial

## Dependencies

TASK02, TASK03.
