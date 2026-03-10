# TASK06 — Cross-Compilation Documentation

## Objective

Document `op build --target`, `build.targets` schema, execution
modes, and transport chain selection in the OP specification.

## Repository

- `organic-programming` (seed): specification docs

## Reference

- [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md)
- [PLATFORM_MATRIX.md](../../PLATFORM_MATRIX.md)

## Scope

### `OP.md` updates

- Add `--target` flag to `op build` reference
- Document `build.targets` manifest field

### `OP_BUILD_SPEC.md` updates

- Add execution modes (binary, framework, WASM)
- Document per-runner cross-compilation behavior
- Document composite target pass-through

### `HOLON_YAML.md` updates

- Add `build.targets` schema with field table

### `PROTOCOL.md` updates

- Update transport selection to reflect mode-dependent chains

## Output

All deliverables go to `v0.7/output/` for review:

| Deliverable | Staging path |
|---|---|
| `OP.md` additions | `output/📝 OP_build_target.md` |
| `OP_BUILD_SPEC.md` additions | `output/📝 OP_BUILD_SPEC_modes.md` |
| `HOLON_YAML.md` additions | `output/📝 HOLON_YAML_targets.md` |
| `PROTOCOL.md` additions | `output/📝 PROTOCOL_transport_modes.md` |

> [!IMPORTANT]
> 📝 All output files require human review before merge.

## Acceptance Criteria

- [ ] Draft all output files
- [ ] Cross-reference with PLATFORM_MATRIX.md
- [ ] 📝 Human review of all output files
- [ ] Move reviewed files to final locations

## Dependencies

TASK01–TASK05 (document what was implemented).
