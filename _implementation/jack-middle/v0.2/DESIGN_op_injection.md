# Jack-Middle v0.2 — `op` Injection and Plugin Wiring

v0.1 requires the operator to manually launch Jack via CLI.
v0.2 integrates Jack into the `op` orchestrator (Grace) so that
proxy interposition can be declared in manifests and recipes.

---

## Problem

1. **Manual wiring**: `jack-middle --target rob-go --hijack` must
   be run by hand before the caller starts. No automation.
2. **Stdio gap**: Port file hijacking only works for TCP/Unix.
   Stdio-based holons (the default transport) cannot be proxied.
3. **Plugin discovery**: Operators must know plugin slugs and
   install them manually. No declarative composition.

## Solution

Grace learns to inject Jack transparently when requested, handling
all transport wiring including stdio piping.

---

## `op serve --via jack`

New flag on `op serve`:

```bash
op serve rob-go --via jack --middleware logger,metrics --plugin snoopy-inspect
```

Grace's serve handler:
1. Launches the real target holon (e.g. rob-go)
2. Launches Jack with `--target <target_uri> --middleware ... --plugin ...`
3. Advertises Jack's frontend address as the visible endpoint
4. Manages lifecycle: stopping Jack stops the target too

### Stdio Pipeline Mode

When the target uses stdio transport (the OP default), Grace
builds a process pipeline:

```
caller ↔ jack (stdio:in) → (stdio:out) ↔ target (stdio:in) → (stdio:out)
```

Grace spawns:
1. Target with `serve --listen stdio://`
2. Jack with `serve --listen stdio:// --target stdio://<target_fd>`
3. Pipes Jack's backend stdout/stdin to the target's stdin/stdout

---

## Recipe-Level Declaration

Recipes can declare proxy interposition per member:

```yaml
members:
  - id: daemon
    path: rob-go
    proxy:
      middleware: [logger, metrics, recorder]
      plugins: [snoopy-inspect, gate-guard]
      record_dir: .op/traces/
```

Grace interprets the `proxy` block and injects Jack
automatically when building the assembly.

### Semantics

| Field | Description |
|---|---|
| `proxy.middleware` | Built-in middleware chain for Jack |
| `proxy.plugins` | Plugin holon slugs to connect |
| `proxy.record_dir` | Output dir for recorder middleware |

If `proxy` is absent, no interposition occurs (default).

---

## Plugin Discovery

When `--plugin snoopy-inspect` is specified:

1. Jack calls `connect("snoopy-inspect")` using the standard
   OP connect algorithm (discover → start → dial → ready)
2. Jack calls `PluginService.Describe` to validate capabilities
3. If the plugin is not found, Jack fails with a clear error

Plugin holons live alongside other holons in `$OPPATH` and
follow the same discovery rules.

---

## Changes to Grace

### `op serve` handler

- New `--via` flag accepting `jack` (or future proxy holons)
- Pipeline construction for stdio transport
- Lifecycle management (graceful shutdown propagation)

### `op build` runner (recipes)

- Parse `proxy` block in recipe members
- Inject Jack into the assembly graph
- Wire middleware and plugin flags

### Proto changes

None — `op serve --via` is a CLI concern, not an RPC.

---

## What Does Not Change

- **Jack's core proxy** — unchanged from v0.1
- **Plugin contract** — `middleware.v1.PluginService` unchanged
- **Holon manifests** — target holons are unmodified
- **connect() algorithm** — unchanged; the advertised address
  is simply Jack's frontend instead of the target's
