# Holon Instance Spec

Status: DRAFT / FREEZING
⚠️ This specification describes how `Discover` targets running instances.

---

## Concept

A **holon instance** is a running process of a holon.
Multiple instances of the same holon can coexist.

Discovery (`DISCOVERY.md`) finds *holons* (packages).
This spec governs running *processes* of those holons.

---

## Identity

To target a specific running process, append the instance UID to the base Address (slug, alias, or path) using a colon `:`.

```
<address>:<uid>

gabriel-greeting-go               // default volatile instance 
gabriel-greeting-go:ea346efb      // specific instance (UID prefix)
op:ea346efb                       // specific instance by alias
```

The `uid` is an identifier (typically a UUID) assigned at spawn time.

| Form | Meaning |
|---|---|
| `gabriel-greeting-go` | default instance |
| `gabriel-greeting-go:ea346efb` | specific instance by UID prefix |

A "default" instance is the one marked `default: true` in the registry,
or the sole running instance, or a fresh volatile one if none exists.

---

## Registry

Instances are tracked in `.op/run/<address>/<uid>/`:

```
.op/run/
  gabriel-greeting-go/
    ea346efb1123.../
      pid          // OS process ID
      socket       // unix socket or tcp port binding
      stdio        // fifo handles if kept alive (persistent mode)
      meta.json    // started-at, last-call, ttl, mode, default flag
```

The holon package itself is never mutated by instance state.

---

## Lifecycle Modes

| Mode | Lifetime | Stdio kept alive? |
|---|---|---|
| **Volatile** | dies after the call | no |
| **Persistent** | survives until explicit stop or TTL expiry | yes |
| **Attached** | lives for the duration of a parent process | inherited |

> **Default Modes**: 
> - If an instance is launched over `stdio://`, the default mode is **Volatile**.
> - If an instance binds a socket (`tcp://` or `unix://`), the default mode is **Persistent**.

*(Note: The 'Attached' mode was previously called 'Session', but was renamed to avoid absolute naming collisions with the `sessions/v0.1` network tracing spec).*

Persistent instances with stdio keepalive skip fork+exec on every call —
this is the intended hot-path optimization.

---

## Ports vs Instances

Instances own ports, not the reverse.

A port (TCP or Unix socket) is a transport detail of an instance.
The instance-id is the stable handle. The same instance may be reachable
over stdio *or* a TCP port depending on how it was started.

Muxing multiple callers to one instance is the responsibility of `op proxy` (see `PROXY.md`).

---

## Cleanup

Stale entries are detected by a PID liveness check on access.
A future reaper mechanism may periodically sweep `.op/run/`.

---

## Design Decisions (Frozen)

- **Targeting Instances via Discover**: Yes, `Discover()` fully supports targeting a single running instance (e.g. `gabriel-greeting-go/ea346efb`). This natively works over `LOCAL` and `DELEGATED` scopes.
- **Op Proxy Muxing**: Yes, `op proxy` fan-ins multiple incoming channels to a single persistent instance.
- **Port Binding**: Dynamic (ephemeral) binding is strongly recommended for local TCP/Unix to avoid `EADDRINUSE` port collisions. The final binding is written to the `.op/run/` registry.
