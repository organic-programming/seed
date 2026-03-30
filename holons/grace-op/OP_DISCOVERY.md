# `op` Discovery

Complement to [DISCOVERY.md](../../DISCOVERY.md) — the universal spec.
This document covers `op`-specific CLI behavior.

---

## Discovery Flags

| Flag | Scope | Notes|
|------|-------|-------|
| *(none)* | same as `--all` | |
| `--all` | everything across all layers | |
| `--siblings` | e.g. bundle for app's bundles | |
| `--cwd` | the execution directory | |
| `--source` | source holons in workspace | |
| `--built` | `.op/build/` packages | |
| `--installed` | `$OPBIN` packages | |
| `--cached` | `$OPPATH/cache` packages | |
| `--root <path>` | override scan root | **preempts any other scoping flag** |

---

## Working Samples

```bash
# List all discovered holons across all layers
op list

# List only source holons and installed packages
op list --source --installed

# Run a holon, forcing a scan from a specific root
op run gabriel-greeting-go --root /path/to/my/app

# Ensure resolution prefers locally built packages before checking installed ones
# (Order of flags does not matter; layer priority is fixed)
op inspect gabriel-greeting-go --built --installed
```

---

## Command Special Cases

```shell
op list     → Discover(LOCAL, null, root, specifiers)                     // DiscoverResult
op build    → resolve(LOCAL, expression, root, --source)                  // HolonRef
op install  → resolve(LOCAL, expression, root, --built)
op run      → connect(LOCAL, expression, root, --installed | --built | --siblings)
#               ↳ if only source found → auto-build, then connect
```

1. **`op build <expression>`** — forces `--source`, ignores other specifiers. `<expression>` can be a path. If `--root` is set, builds within that root (if it contains sources recursively).
2. **`op install <expression> --build`** — composition: `build --source` then `install --installed`. Without `--build`, uses the already-built binary.
3. **`op run <expression>`** — installed → built → auto-build fallback. Add `--build` to force a build. When only a source holon is found, auto-build kicks in.

---

## Commands That Use Discovery

Every command accepts an `<expression>` — any valid discovery expression (slug, alias, uuid, path, direct-url, or a specific running instance targeted via `:<uid>`).

| Command | Notes |
|---|---|
| `op <expression> <command> [args]` | dispatch via auto-connect chain |
| `op run <expression>` | |
| `op build [<expression>]` | forces `--source` specifier |
| `op check [<expression>]` | |
| `op test [<expression>]` | |
| `op clean [<expression>]` | |
| `op install [<expression>]` | |
| `op uninstall <expression>` | |
| `op do <expression> <sequence>` | |
| `op tools <expression>` | |
| `op mcp <expression>` | |
| `op show <expression>` | |
| `op inspect <expression>` | |

### Exceptions

**`op list [root]`** — the positional argument is a *directory to scan*, not a `<holon>` to resolve. It answers "what's here?" not "where is X?".

**`op inspect <holon>`** — also accepts bare `host:port`, which is not a holon identity key but a network address[^1].

### Direct Dispatch (Fast Paths)

Because the CLI evaluates the `expression` exactly as defined in [DISCOVERY.md](../../../DISCOVERY.md), passing a raw path or transport URL resolves instantly without triggering a filesystem scan.

- **Path Expression**: `op /path/to/binary <method>` — resolves to the local binary instantly.
- **Direct URL Expression**: `op tcp://127.0.0.1:4000 <method>` — dials the transport URL directly.

> **A Note on Performance & Caching**  
> The `op` CLI **strictly refuses** to cache discovery results (such as mapping a slug permanently to a binary path). The ecosystem is intentionally dynamic; caching ruins the ability to hot-swap binaries or gracefully plummet through the auto-build fallback chain.  
> Performance in Organic Programming is strictly targeted at the *RPC processing layer* (via `Persistent` instances routing multiplexed pipes). The marginal 5-millisecond cost of re-scanning a local directory on connection is utterly irrelevant to the lifecycle of an application.

*(Note: Meta-commands like `op serve`, `op new`, `op env`, and `op version` scaffold internal states and do not accept discovery expressions).*

---

## The `--origin` Flag

> Replaces the former `--bin` flag.

**VERY IMPORTANT** — `op <holon> <command> --origin` shows the origin (resolved path, layer) in stderr. Operational during build.