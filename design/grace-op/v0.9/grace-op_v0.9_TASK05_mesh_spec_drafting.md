# TASK05 — Mesh Specification Drafting

## Objective

Draft the spec documents for mesh topology, mTLS transport
security, and `mesh.yaml` format. Uses the `.DRAFT_mesh_documentation.md`
as the working plan.

## Repository

- `organic-programming` (seed): specification docs

## Reference

- [DESIGN_mesh.md](./DESIGN_mesh.md)
- [.DRAFT_mesh_documentation.md](./.DRAFT_mesh_documentation.md) (local, gitignored)

## Output

All deliverables go to `v0.9/output/` for review:

| Deliverable | Staging path |
|---|---|
| `OP.md` §11 (mesh) | `output/📝 OP_mesh_section.md` |
| `PROTOCOL.md` §8 (transport security) | `output/📝 PROTOCOL_transport_security.md` |
| `MESH_YAML.md` (new) | `output/📝 MESH_YAML.md` |
| `HOLON_YAML.md` additions | `output/📝 HOLON_YAML_listeners.md` |
| `CONVENTIONS.md` additions | `output/📝 CONVENTIONS_mesh_dirs.md` |

> [!IMPORTANT]
> 📝 All output files require human review before merge.

## Acceptance Criteria

- [ ] Draft all output files per .DRAFT checklist
- [ ] Cross-reference: `OP.md` §11 ↔ `PROTOCOL.md` §8 ↔ `MESH_YAML.md`
- [ ] 📝 Human review of all output files
- [ ] Move reviewed files to final locations

## Dependencies

TASK01–04 (document what was designed and implemented).
