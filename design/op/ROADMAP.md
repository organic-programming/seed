# OP Roadmap

Version plan for Grace OP (`op` CLI).

---

## v0.3 — Core Maturity

Complete `op` for single-host development.

- `op install --no-build` flag
- Composite kind formalization (`kind: composite` + `artifacts.primary`)
- 7 new runners (cargo, swift-package, flutter, npm, gradle, dotnet, qt-cmake)
- Bundle install (`.app`, `.exe` → `$OPBIN`)
- Package manager distribution (Homebrew, WinGet, NPM)
- Spec documentation for mesh and setup
- Holon templates (`op new --template`)

**Tasks:** [v0.3/_TASKS.md](./v0.3/_TASKS.md)

---

## v0.4 — Recipe Ecosystem

Restructure recipes into a DRY monorepo with shared components
and composition patterns.

- Extract 8 DRY daemons + 6 DRY HostUIs
- 48 assembly manifests (daemon × HostUI)
- Composition recipes (direct-call, pipeline, fan-out)
- Remove 12 legacy submodules
- Combinatorial testmatrix program

**Tasks:** [v0.4/_TASKS.md](./v0.4/_TASKS.md)

---

## v0.5 — Extensibility

Build configs, dependency resolution, and executable sequences.

- Build configs (`--config` + `OP_CONFIG`)
- MVS transitive dependency resolution
- Sequences (`op do` + MCP tool exposure)

**Tasks:** [v0.5/_TASKS.md](./v0.5/_TASKS.md)

---

## v0.6 — REST + SSE Transport

Add HTTP-native transport for distributed holon communication.

- Unary RPC → POST mapping
- Server-streaming → SSE (EventSource)
- Proto ↔ JSON transcoding via `protojson`
- SDK transport adapter (serve + connect)
- mTLS over standard HTTPS

**Design:** [DESIGN_transport_rest_sse.md](./v0.6/DESIGN_transport_rest_sse.md)

---

## v0.7 — Cross-Compilation & Platform Targets

Build holons for mobile and browser from a desktop host.

- `op build --target <platform>` flag
- Execution mode selection per target (binary, framework, WASM)
- `build.targets` in `holon.yaml` (per-platform build rules)
- Go: `gomobile bind` for iOS/Android, `GOOS=js` for WASM
- Rust: `cdylib` for mobile, `wasm-pack` for browser
- C/C++: Emscripten for WASM, NDK for Android
- Platform-aware connect chain (auto-select transport by mode)

**Design:** [DESIGN_cross_compilation.md](./v0.7/DESIGN_cross_compilation.md)

---

## v0.8 — Mesh

Enable multi-host holon networks with `op mesh`.

- `op mesh init` — generate private CA
- `op mesh add/remove/list/status/describe` — topology management
- SSH-based certificate provisioning
- `mesh.yaml` registry
- SDK integration: mesh-aware discover, connect, serve (mTLS)

**Design:** [DESIGN_mesh.md](./v0.8/DESIGN_mesh.md)

---

## v0.9 — Public Holons

Expose holons to external consumers with per-listener security.

- Per-listener `security` annotation in `holon.yaml` (`none`, `mesh`, `public`)
- Auth interceptors (API key, JWT, OAuth)
- Multi-listener `serve.Run` with mixed TLS configs
- Consumer identity on gRPC context

**Design:** [DESIGN_public_holons.md](./v0.9/DESIGN_public_holons.md)

---

## v0.10 — Setup

Declarative host provisioning from zero to functioning OP host.

- `op setup <image.yaml>` — 6-phase execution engine
- Dependency resolution across toolchains, system packages, holons
- Platform-specific package manager bootstrapping
- Source compilation from pinned refs
- Multi-image composition
- Mesh join integration

**Design:** [DESIGN_setup.md](./v0.10/DESIGN_setup.md)

---

## Dependency Chain

```
v0.3 (core maturity)
  ├─ v0.4 (recipe ecosystem)
  └─ v0.5 (extensibility)
       └─ v0.6 (REST+SSE transport)
            └─ v0.7 (cross-compilation)
                 └─ v0.8 (mesh networking)
                      └─ v0.9 (public security)
                           └─ v0.10 (provisioning)
```

v0.4 and v0.5 can proceed in parallel after v0.3. The distributed
story begins at v0.6.
