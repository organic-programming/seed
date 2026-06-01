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
| `--limit <n>` | limit discovery results | `op list` only; `0` means no limit |
| `--connect-timeout <ms>` | discovery/connect timeout | applies to all commands; `0` means no timeout; hidden flag |
| `--timeout <ms>` | RPC execution timeout | invoke calls only (per call); `0` means no timeout; env `OPTIMEOUT` |
| `--no-cache` | resolution cache | bypasses resolution cache reads and refreshes cache on success |
| `--purge-cache` | resolution cache | deletes `$OPPATH/resolutions/` before continuing; bare `op --purge-cache` exits 0 |

⚠️ Phase 1 note: Only `LOCAL` scope is supported. `PROXY`, `DELEGATED`, and instance targeting (`:uid`) will be available in a future release.

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
op list     → Discover(LOCAL, null, root, specifiers, limit, timeout)     // DiscoverResult
op build    → Resolve(LOCAL, expression, root, SOURCE, timeout)           // HolonRef
op install  → Resolve(LOCAL, expression, root, BUILT, timeout)
op run      → Connect(LOCAL, expression, root, INSTALLED | BUILT | SIBLINGS, timeout)
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

- **Path Expression**: `op /path/to/binary <method>` — resolves to the local binary instantly. When the target exposes a slug through `.holon.json` or `Describe`, `op` also refreshes the global slug cache for that slug.
- **Direct URL Expression**: `op tcp://127.0.0.1:4000 <method>` — dials the transport URL directly.

> **A Note on Performance & Caching**  
> The `op` CLI keeps an internal on-disk resolution cache for local discovery snapshots. It preserves the dynamic behavior expected by Organic Programming: hot-swapped built or installed targets are checked with per-entry stat data before a cached ref is reused; missing slugs are never cached negatively, so the auto-build fallback chain can still discover newly created source holons; and mutating `op` commands invalidate snapshots through a dirty marker. Manual edits to `holon.proto` made outside `op new` are intentionally not watched; use `op --purge-cache` after those edits when immediate cache invalidation is required.

*(Note: Meta-commands like `op serve`, `op new`, `op env`, and `op version` scaffold internal states and do not accept discovery expressions).*

---

## Resolution Cache

The resolution cache is private to `op`. SDKs do not read or write it directly; SDK source-layer discovery that delegates to a local `op` benefits from the cache through the existing local RPC path.

### Flags

- `--no-cache` bypasses cache reads for one invocation, then writes successful results back to the cache. This is a refresh, not a permanent cache opt-out.
- `--purge-cache` removes `$OPPATH/resolutions/` before the command continues. With no subcommand, `op --purge-cache` purges and exits successfully.

Both flags are root-level flags and must appear before any subcommand or dispatch expression, for example:

```bash
op --no-cache list --source
op --purge-cache gabriel-greeting-go ListGreetings '{}'
```

### Layout

```text
$OPPATH/resolutions/
├── .dirty
├── global.json
├── <hash16>.json
└── ...
```

`global.json` is tier 1: a global slug-to-`HolonRef` map shared across roots and specifier sets. It is used only for slug-driven resolution, not enumeration.

`<hash16>` is `sha256(canonical_root_path + "|" + specifiers_bitmask)` truncated to 16 lowercase hex characters. The specifier bitmask is stored and hashed in the same `0xNN` form used in snapshots.

### Tier 1 Schema

```jsonc
{
  "version": 1,
  "entries": {
    "gabriel-greeting-go": {
      "url": "file:///path/to/holon",
      "info": { /* HolonInfo as JSON */ },
      "target_path": "/path/to/binary_or_holon_dir",
      "target_mtime_ns": 1731234567890123456
    }
  }
}
```

### Tier 2 Snapshot Schema

```jsonc
{
  "version": 1,
  "root": "/canonical/path/to/root",
  "specifiers": "0x06",
  "scanned_at": "2026-05-09T14:00:00Z",
  "entries": [
    {
      "url": "file:///path/to/holon",
      "info": { /* HolonInfo as JSON */ },
      "target_path": "/path/to/binary_or_holon_dir",
      "target_mtime_ns": 1731234567890123456
    }
  ]
}
```

Writes are atomic: `op` writes a temporary file next to the cache file and renames it into place. There is no lock file; concurrent writers are allowed and the last completed rename wins.

### Lookup Order

Slug-driven resolution uses both tiers:

1. If `--no-cache` is set, skip cache reads.
2. Check tier 1 (`global.json`). If the slug entry exists and its `target_path` still has the recorded `target_mtime_ns`, return immediately.
3. Check tier 2 (`<hash16>.json`) for the current root and specifier set. A valid slug hit returns immediately.
4. Walk the requested layers. If the walk finds exactly one match, write tier 1 and tier 2. If it finds zero or multiple matches, write tier 2 only.

Path-expression resolution (`/abs/path`, relative paths, and `file://...`) bypasses discovery scans. When it successfully reads `.holon.json` or falls back to `Describe`, it extracts the slug and refreshes tier 1.

Enumeration queries such as `op list` and shell-completion candidate collection use tier 2 only. They do not consult tier 1 because tier 1 intentionally forgets root and specifier context.

### Invalidation

On a cache hit, every entry is stat-checked against `target_path` and `target_mtime_ns`. If a target disappeared or its mtime changed, `op` drops or ignores the cached entry and performs a fresh walk.

`$OPPATH/resolutions/.dirty` is touched by:

- `op build`
- `op install`
- `op clean`
- `op uninstall`
- `op new`

If `.dirty` is newer than a cache file, that cache file is ignored. The marker invalidates tier 1 wholesale and invalidates stale tier 2 snapshots. Read-only commands (`op list`, `op test`, `op inspect`, dispatch, and `op serve`) do not touch `.dirty`.

There is no TTL and no negative caching. A missing slug always triggers a fresh walk, even when a snapshot exists for the same root and specifiers.

### Completion Ambiguity

Shell completion collects candidates from the contextual tier 2 snapshot, or from a fresh contextual walk when no valid snapshot exists. A single matching slug completes as that slug. When the same slug or alias matches multiple holons, completion emits absolute path candidates instead so the selected item is an explicit path expression.

---

## The `--origin` Flag

> Replaces the former `--bin` flag.

**VERY IMPORTANT** — `op <holon> <command> --origin` shows the origin (resolved path, layer) in stderr. Operational during build.
