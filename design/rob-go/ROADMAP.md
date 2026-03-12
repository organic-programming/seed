# Rob-Go Roadmap

Version plan for Rob Go (Go toolchain holon).

---

## v0.1 — Toolchain Holon

Transform Rob-Go from a wrapper (`kind: wrapper`) into a
self-contained toolchain holon (`kind: toolchain`) that embeds
the Go distribution and manages its own environment.

- Toolchain provisioning (download, verify, cache Go distro)
- Hermetic environment construction (no `os.Environ()` leakage)
- Exec mode wired to embedded go binary
- Library mode wired to hermetic env
- Manifest update to `kind: toolchain`
- gRPC reflection enabled

**Tasks:** [v0.1/_TASKS.md](./v0.1/_TASKS.md)

---

## v0.2 — Version Management

Allow the pinned Go version to be replaced at runtime via a
dedicated RPC. Single active version — the old one is pruned
after a successful switch.

- `UpdateToolchain` RPC (resolve latest, download, verify, swap)
- Prune old version after successful replacement
- Expose current version via `Describe` or dedicated RPC

**Design:** [v0.2/DESIGN_version_management.md](./v0.2/DESIGN_version_management.md)

---

## v0.3 — CGO Toolchain

Enable CGO-dependent builds. Two successive stages:

1. **Passthrough** — inherit host `CC`/`CXX`/`AR` via an allowlist
   in the hermetic environment (minimal, unblocks CGO immediately)
2. **Embedded C toolchain** — bundle a cross-compiler (Zig CC or
   musl-based) so CGO builds are fully hermetic

- Passthrough allowlist (`CC`, `CXX`, `AR`, `PKG_CONFIG_PATH`)
- Manifest `delegates.toolchain.cgo` declaration
- Evaluate Zig CC vs. musl cross-compiler
- Bundle alongside Go distro under `$OPPATH/toolchains/`
- Cross-platform matrix (darwin-arm64, linux-amd64, etc.)

**Design:** [v0.3/DESIGN_cgo_toolchain.md](./v0.3/DESIGN_cgo_toolchain.md)
