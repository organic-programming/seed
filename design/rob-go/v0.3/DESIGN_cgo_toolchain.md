# Rob-Go v0.3 — CGO Toolchain

Rob-Go v0.1 builds a hermetic Go environment that strips all
host variables. This breaks `CGO_ENABLED=1` builds because Go
shells out to a C compiler (`CC`, `CXX`) that must exist on
the host. v0.3 addresses CGO in two stages.

---

## Problem

1. **Hermetic env blocks CGO**: `HermeticEnv` does not carry `CC`,
   `CXX`, or `AR`. Any holon using CGO (e.g. SQLite bindings,
   wisupaa-whisper) fails to build.
2. **Host dependency**: Even with passthrough, CGO still depends
   on whatever compiler the developer has installed. No
   reproducibility guarantee for C code.

---

## Stage 1 — Passthrough Allowlist

Minimal approach: inherit host C toolchain variables when CGO
is expected.

### Allowlist

| Variable | Purpose |
|---|---|
| `CC` | C compiler path |
| `CXX` | C++ compiler path |
| `AR` | Archiver |
| `PKG_CONFIG_PATH` | pkg-config search paths |

### Activation

The allowlist applies when either:
1. The manifest declares `delegates.toolchain.cgo.enabled: true`
2. The caller's per-RPC `env` contains `CGO_ENABLED=1`

### Implementation

```go
func (t *Toolchain) HermeticEnv(overrides []string, cgoPassthrough bool) []string
```

### Manifest Extension

```yaml
delegates:
  toolchain:
    name: go
    version: "1.24.0"
    source: https://go.dev/dl/
    cgo:
      enabled: true
      passthrough: [CC, CXX, AR, PKG_CONFIG_PATH]
```

---

## Stage 2 — Embedded C Toolchain

Full hermiticity: bundle a C cross-compiler alongside Go so
that CGO builds never depend on host-installed compilers.

### Candidates

| Option | Pros | Cons |
|---|---|---|
| **Zig CC** | Drop-in `CC` replacement, excellent cross-compilation, single binary | Zig release cadence, non-trivial download (~40 MB) |
| **musl-cross** | Battle-tested for static Linux builds | Linux-only, no macOS/Windows cross-compile |
| **System Xcode/GCC** | Zero download cost | Not hermetic, breaks the model |

### Storage Layout

```
$OPPATH/toolchains/
├── go/           ← managed by Rob (v0.1)
└── cc/           ← managed by Rob (v0.3)
    ├── zig-0.13/
    └── current -> zig-0.13
```

### Hermetic `CC`/`CXX`

When the embedded C toolchain is present, `HermeticEnv` sets
`CC` and `CXX` to the bundled compiler paths — no host
passthrough needed.

---

## Cross-Platform Matrix

| Target | Stage 1 (passthrough) | Stage 2 (embedded) |
|---|---|---|
| darwin-arm64 | Host Xcode clang | Zig CC |
| darwin-amd64 | Host Xcode clang | Zig CC |
| linux-amd64 | Host gcc/clang | Zig CC or musl |
| linux-arm64 | Host gcc/clang | Zig CC or musl |
| windows-amd64 | Host MSVC/MinGW | Zig CC |
