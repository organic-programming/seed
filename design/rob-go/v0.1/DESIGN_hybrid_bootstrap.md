# Rob-Go v0.1 — Toolchain Holon

Rob-Go is not a CLI wrapper for `go`. It is the **Go script engine**
of Organic Programming — a self-contained holon that embeds the Go
toolchain, manages its own environment, and exposes Go's full power
as gRPC RPCs.

---

## Problem

The bootstrap implementation treats Rob-Go as `kind: wrapper` —
it calls `go` on `$PATH` via `os/exec`. This creates three issues:

1. **External dependency**: Rob-Go requires the user to have Go
   installed and correctly configured. If `go` is missing or the
   wrong version, Rob-Go cannot function.
2. **Environment pollution**: The user's `GOPATH`, `GOMODCACHE`,
   `GOCACHE` etc. leak into Rob's operations. Holon builds can
   interfere with user projects and vice versa.
3. **Identity confusion**: `kind: wrapper` implies Rob is a thin
   proxy for an external tool. But Rob is foundational — every Go
   holon in OP depends on him. He should own his runtime.

## Solution

Rob-Go becomes a **toolchain holon** (`kind: toolchain`): a holon
that provisions, owns, and operates a language runtime. He carries
Go inside himself.

### Architecture

```
                 ┌──────────────────────────────────┐
                 │          Rob-Go holon             │
                 │                                   │
                 │  ┌─────────────┐  ┌────────────┐  │
                 │  │  Exec mode  │  │ Lib mode   │  │
                 │  │  (os/exec)  │  │ (go/*)     │  │
                 │  └──────┬──────┘  └────────────┘  │
                 │         │                          │
                 │  ┌──────┴──────────────────────┐   │
                 │  │    Managed Environment       │   │
                 │  │  GOROOT  GOPATH  GOCACHE     │   │
                 │  │  GOMODCACHE  GOBIN  GOTMPDIR │   │
                 │  └──────┬──────────────────────┘   │
                 │         │                          │
                 │  ┌──────┴──────┐                   │
                 │  │  Embedded   │                   │
                 │  │  Go Distro  │                   │
                 │  │  (pinned)   │                   │
                 │  └─────────────┘                   │
                 └──────────────────────────────────┘
```

---

## Toolchain Provisioning

### Storage Layout

Rob-Go stores its embedded toolchain under `$OPPATH`:

```
$OPPATH/toolchains/go/
├── versions/
│   ├── go1.24.0/           ← extracted Go distribution
│   │   ├── bin/go
│   │   ├── bin/gofmt
│   │   ├── src/
│   │   ├── pkg/
│   │   └── ...
│   └── go1.25.0/           ← future versions
├── cache/                   ← GOCACHE (shared across versions)
├── modcache/                ← GOMODCACHE (shared across versions)
└── current -> go1.24.0     ← symlink to active version
```

### Pinned Version

The Go version is declared in `holon.yaml`:

```yaml
delegates:
  toolchain:
    name: go
    version: "1.24.0"
    source: https://go.dev/dl/
```

This replaces the current `delegates.commands: [go]` field.

### Bootstrap Sequence

When Rob-Go starts and the pinned version is not cached:

1. Check `$OPPATH/toolchains/go/versions/<version>/`
2. If missing: download from `https://go.dev/dl/go<version>.<os>-<arch>.tar.gz`
3. Verify checksum (SHA-256 from `https://go.dev/dl/?mode=json`)
4. Extract to `$OPPATH/toolchains/go/versions/<version>/`
5. Update `current` symlink
6. Ready

If the network is unavailable and the version is not cached, Rob-Go
fails with a clear error — never falls back to system `go`.

---

## Environment Isolation

Every exec-mode subprocess call uses a **hermetic environment**
constructed by Rob-Go. No system Go variables leak in.

### Managed Variables

| Variable | Value | Purpose |
|---|---|---|
| `GOROOT` | `$OPPATH/toolchains/go/versions/<ver>` | Toolchain location |
| `GOPATH` | `$OPPATH/toolchains/go/gopath` | Workspace (rarely used in module mode) |
| `GOMODCACHE` | `$OPPATH/toolchains/go/modcache` | Module download cache |
| `GOCACHE` | `$OPPATH/toolchains/go/cache` | Build cache |
| `GOBIN` | `$OPBIN` | Install target for `go install` |
| `GOTMPDIR` | (system default) | Temp files |
| `PATH` | `<GOROOT>/bin:$OPBIN:<original>` | Prepend Rob's Go |

### Caller Overrides

Per-RPC `env` fields in gRPC requests (e.g., `GoCommandRequest.env`)
are applied **on top of** the managed environment. This lets callers
set build tags, `CGO_ENABLED`, `GOOS`/`GOARCH` etc. without
breaking isolation.

### `workdir` Semantics

The `workdir` field in requests is unchanged — it sets `Cmd.Dir`
for subprocess calls and `packages.Config.Dir` for library calls.
Rob-Go does not impose a workspace; callers point at their own
holon directories.

---

## Exec Mode Changes

### Before (wrapper)

```go
cmd := exec.CommandContext(ctx, "go", argv...)
cmd.Env = append(os.Environ(), env...)
```

### After (toolchain)

```go
cmd := exec.CommandContext(ctx, s.goBinary(), argv...)
cmd.Env = s.hermeticEnv(env)
```

Where:
- `goBinary()` returns the path to the embedded `go` binary.
- `hermeticEnv(overrides)` builds the hermetic environment from
  managed variables + caller overrides. It does **not** inherit
  `os.Environ()` — it constructs from scratch with only the
  necessary system variables (`HOME`, `USER`, `TMPDIR`, `PATH`).

### Library Mode

Library mode uses `go/parser`, `go/format`, etc. compiled into
Rob-Go. These do not use the embedded toolchain at runtime.

However, `packages.Load`, `TypeCheck`, and `Analyze` do shell out
via the driver. They must inherit the same hermetic environment:

```go
cfg := &packages.Config{
    Env: s.hermeticEnv(env),
    // ...
}
```

---

## New `kind: toolchain`

The `kind` field in `holon.yaml` needs a new value:

| Kind | Meaning |
|---|---|
| `native` | Self-contained binary, no external deps |
| `wrapper` | Delegates to an external CLI |
| `composite` | Manifest-only assembly of other holons |
| **`toolchain`** | **Provisions and owns a language runtime** |

A toolchain holon:
- Declares `delegates.toolchain` (name + version + source)
- Provisions the toolchain on first run
- Isolates its environment from the host
- Is the **single authority** for that language in the OP ecosystem

### Updated `holon.yaml`

```yaml
# ── Identity ──────────────────────────────────────────
schema: holon/v0
uuid: "d4e5f6a7-8b9c-0d1e-2f3a-4b5c6d7e8f9a"
given_name: Rob
family_name: Go
motto: Build anything, test everything.
composer: B. ALTER
clade: deterministic/toolchain
status: draft
born: "2026-03-02"

# ── Contract ──────────────────────────────────────────
contract:
  service: go.v1.GoService

# ── Operational ───────────────────────────────────────
kind: toolchain
build:
  runner: go-module
  main: ./cmd/rob-go
delegates:
  toolchain:
    name: go
    version: "1.24.0"
    source: https://go.dev/dl/
artifacts:
  binary: rob-go
```

---

## gRPC Reflection

The bootstrap spec mandates gRPC reflection. The current
implementation is missing `reflection.Register(s)`. This must
be added.

---

## What Does Not Change

- **Proto contract** (`go.v1.GoService`) — 18 RPCs unchanged.
- **Service layer** (`internal/service/`) — delegation pattern
  unchanged, only the environment construction changes.
- **Analyzer library** (`internal/analyzer/`) — in-process functions
  unchanged (except `loadWithConfig` must use hermetic env).
- **Test structure** — same packages, same patterns.

---

## Impact on Other Holons

- **grace-op** (`op build`)  — when it calls `go build` via the
  `go-module` runner, it delegates to Rob-Go. The runner must
  call Rob's `Build` RPC instead of `exec.Command("go", "build")`.
  This is a v0.4+ concern, not v0.1.
- **jess-npm** — same pattern can be applied (embed Node.js,
  own npm cache). `kind: toolchain` generalizes.
