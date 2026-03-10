# OP Documents

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

- [DESIGN_mesh.md](./DESIGN_mesh.md) — `op mesh` commands, mTLS, `mesh.yaml`
- [DESIGN_setup.md](./DESIGN_setup.md) — `op setup` provisioning, `setup.yaml`
- [DESIGN_public_holons.md](./DESIGN_public_holons.md) — per-listener security policy
- [DESIGN_transport_rest_sse.md](./DESIGN_transport_rest_sse.md) — REST + SSE transport
- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md) — `op build --target` cross-compilation
- [DESIGN_sequences.md](./DESIGN_sequences.md) — `sequences:` manifest + `op do` executor
- [DESIGN_holon_templates.md](./DESIGN_holon_templates.md) — `op new --template` scaffolding

## Tasks

- [v0_3/](./v0_3/_TASKS.md) — implementation tasks for v0.3
