# Jack-Middle Roadmap

Version plan for Jack Middle (transparent gRPC proxy holon).

---

## v0.1 — Transparent Proxy

A man-in-the-middle gRPC reverse proxy that can impersonate
any holon and apply a middleware chain for observation and
fault injection.

- Transparent frame relay via `UnknownServiceHandler`
- Middleware chain: built-in (logger, metrics, recorder, fault)
- Plugin holon support (`middleware.v1.PluginService` contract)
- CLI-driven configuration (target, middleware, plugins, flags)
- Control service (`middle.v1.MiddleService`) for runtime query
- Port file hijacking for transparent interposition
- Manifest + gRPC reflection

**Tasks:** [v0.1/_TASKS.md](./v0.1/_TASKS.md)

---

## v0.2 — `op` Injection

Integrate Jack into the `op` orchestrator (Grace) so that
proxy interposition can be declared in manifests and recipes.

- `op serve --via jack` flag in Grace
- Stdio pipeline wiring for stdio-based holons
- Recipe-level `proxy` block for per-member interposition
- Automatic plugin discovery via connect algorithm

**Design:** [v0.2/DESIGN_op_injection.md](./v0.2/DESIGN_op_injection.md)
**Tasks:** [v0.2/_TASKS.md](./v0.2/_TASKS.md)
