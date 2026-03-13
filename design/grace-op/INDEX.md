# Grace-OP Documents

## Current Specifications

- [OP.md](../../OP.md) — `op` CLI command reference
- [HOLON_YAML.md](../../HOLON_YAML.md) — holon manifest specification
- [OP_BUILD_SPEC.md](../../holons/grace-op/OP_BUILD_SPEC.md) — `op build` recipe runner spec

## Implementation

- [grace-op/](../../holons/grace-op/) — holon source code
- [grace-op/README.md](../../holons/grace-op/README.md) — install, setup, usage

## Proposed Additions

These design documents describe features **not yet in the specs above**.
Each proposes new commands, schemas, or SDK capabilities to be added.

- [DESIGN_holon_templates.md](./v0.3/DESIGN_holon_templates.md) — `op new --template` scaffolding
- [DESIGN_recipe_ecosystem.md](./v0.4/DESIGN_recipe_ecosystem.md) — DRY recipe monorepo
- [DESIGN_recipe_monorepo.md](./v0.4/DESIGN_recipe_monorepo.md) — composition matrix & proto contracts
- ⭐ [VERIFICATION_composition.md](./v0.4/VERIFICATION_composition.md) — **exhaustive manual verification guide** (66 assemblies + 36 compositions)
- [DESIGN_bundle_codesign.md](./v0.4.4/DESIGN_bundle_codesign.md) — auto ad-hoc bundle signing
- [DESIGN_native_daemon_expansion.md](./v0.4.5/DESIGN_native_daemon_expansion.md) — C++, C, Java Daemons
- [DESIGN_sequences.md](./v0.5/DESIGN_sequences.md) — `sequences:` manifest + `op do` executor
- [DESIGN_transport_rest_sse.md](./v0.6/DESIGN_transport_rest_sse.md) — REST + SSE transport
- [DESIGN_sdk_transport_parity.md](./v0.6.1/DESIGN_sdk_transport_parity.md) — SDK transport full parity (source-verified matrix, runs after v0.6)
- [DESIGN_cross_compilation.md](./v0.7/DESIGN_cross_compilation.md) — `op build --target` cross-compilation
- [DESIGN_release_pipeline.md](./v0.8/DESIGN_release_pipeline.md) — `op publish`, holon registry, CI matrix
- [DESIGN_mesh.md](./v0.9/DESIGN_mesh.md) — `op mesh` commands, mTLS, `mesh.yaml`
- [DESIGN_public_holons.md](./v0.10/DESIGN_public_holons.md) — per-listener security policy
- [DESIGN_setup.md](./v0.11/DESIGN_setup.md) — `op setup` provisioning, `setup.yaml`

## Tasks

- [v0.3/](./v0.3/_TASKS.md) — Core Maturity (7 tasks)
- [v0.4/](./v0.4/_TASKS.md) — Recipe Ecosystem (6 tasks)
  - [v0.4.1/](./v0.4.1/_TASKS.md) — Go+Dart PoC validation
  - [v0.4.2/](./v0.4.2/_TASKS.md) — Repo-Truth Matrix Extraction (daemons, hostuis, assemblies)
  - [v0.4.3/](./v0.4.3/_TASKS.md) — Assembly Manifests & Composition (48 assemblies, testmatrix)
  - [v0.4.4/](./v0.4.4/_TASKS.md) — Bundle Auto-Signing (2 tasks)
  - [v0.4.5/](./v0.4.5/_TASKS.md) — Native Daemon Expansion (5 tasks: 18 assemblies + 3 C compositions + doc updates)
- [v0.5/](./v0.5/_TASKS.md) — Extensibility (3 tasks)
- [v0.6/](./v0.6/_TASKS.md) — REST + SSE Transport (10 tasks)
  - [v0.6.1/](./v0.6.1/_TASKS.md) — SDK Transport Full Parity (6 tasks, after v0.6 delivery)
- [v0.7/](./v0.7/_TASKS.md) — Cross-Compilation (15 tasks)
- [v0.8/](./v0.8/_TASKS.md) — Release Pipeline (7 tasks)
- [v0.9/](./v0.9/_TASKS.md) — Mesh (4 tasks)
- [v0.10/](./v0.10/_TASKS.md) — Public Holons (6 tasks)
- [v0.11/](./v0.11/_TASKS.md) — Setup (4 tasks)
