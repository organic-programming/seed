# Greeting HostUI Flutter

This is the canonical Flutter HostUI for the v0.4 recipe monorepo.

Runtime resolution order:

1. If `GUDULE_DAEMON_TARGET` is set, the app connects to that slug or URI
   through `dart-holons connect()`.
2. Otherwise it stages a local `greeting-daemon` manifest that points at a
   packaged daemon binary named `gudule-greeting-daemon`.
3. For local development, the fallback daemon comes from
   `recipes/daemons/greeting-daemon-go`.

Useful commands:

```bash
go run ./holons/grace-op/cmd/op build recipes/daemons/greeting-daemon-go
go run ./holons/grace-op/cmd/op build recipes/hostui/greeting-hostui-flutter
```
