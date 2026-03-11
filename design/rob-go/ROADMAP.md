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
