# OP Roadmap

Version plan for Grace OP (`op` CLI).

---

## v0.3 — Core Maturity

Complete `op` for single-host development.

- 7 new runners (cargo, swift-package, flutter, npm, gradle, dotnet, qt-cmake)
- Build configs (`--config` + `OP_CONFIG`)
- Composite kind formalization (`kind: composite` + `artifacts.primary`)
- Bundle install (`.app`, `.exe` → `$OPBIN`)
- MVS transitive dependency resolution
- `op install --no-build` flag
- Package manager distribution (Homebrew, WinGet, NPM)
- Recipe restructuring
- Spec documentation for mesh and setup
- Sequences (`op do` + MCP tool exposure)

**Tasks:** [v0_3/_TASKS.md](./v0_3/_TASKS.md)

---

## v0.4 — REST + SSE Transport

Add HTTP-native transport for distributed holon communication.

- Unary RPC → POST mapping
- Server-streaming → SSE (EventSource)
- Proto ↔ JSON transcoding via `protojson`
- SDK transport adapter (serve + connect)
- mTLS over standard HTTPS

**Design:** [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md)

---

## v0.5 — Cross-Compilation & Platform Targets

Build holons for mobile and browser from a desktop host.

- `op build --target <platform>` flag
- Execution mode selection per target (binary, framework, WASM)
- `build.targets` in `holon.yaml` (per-platform build rules)
- Go: `gomobile bind` for iOS/Android, `GOOS=js` for WASM
- Rust: `cdylib` for mobile, `wasm-pack` for browser
- C/C++: Emscripten for WASM, NDK for Android
- Platform-aware connect chain (auto-select transport by mode)

**Design:** [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md)

---

## v0.6 — Mesh

Enable multi-host holon networks with `op mesh`.

- `op mesh init` — generate private CA
- `op mesh add/remove/list/status/describe` — topology management
- SSH-based certificate provisioning
- `mesh.yaml` registry
- SDK integration: mesh-aware discover, connect, serve (mTLS)

**Design:** [DESIGN_mesh.md](./DESIGN_mesh.md)

---

## v0.7 — Public Holons

Expose holons to external consumers with per-listener security.

- Per-listener `security` annotation in `holon.yaml` (`none`, `mesh`, `public`)
- Auth interceptors (API key, JWT, OAuth)
- Multi-listener `serve.Run` with mixed TLS configs
- Consumer identity on gRPC context

**Design:** [DESIGN_public_holons.md](./DESIGN_public_holons.md)

---

## v1.0 — Setup

Declarative host provisioning from zero to functioning OP host.

- `op setup <image.yaml>` — 6-phase execution engine
- Dependency resolution across toolchains, system packages, holons
- Platform-specific package manager bootstrapping
- Source compilation from pinned refs
- Multi-image composition
- Mesh join integration

**Design:** [DESIGN_setup.md](./DESIGN_setup.md)

---

## Dependency Chain

```
v0.3 (single-host)
  └─ v0.4 (REST+SSE transport)
       └─ v0.5 (cross-compilation)
            └─ v0.6 (mesh networking)
                 └─ v0.7 (public security)
                      └─ v1.0 (provisioning)
```

Each version adds one layer to the distributed story.
