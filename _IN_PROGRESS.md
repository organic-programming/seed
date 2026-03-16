# Re-Engineering In Progress

> **For contributing agents:** This document lists features and concepts
> that are deprecated or under active redesign. Do not implement, extend,
> or rely on them. Check this file before starting work.

---

## Deprecated Concepts

The following have been removed from the specification. They may still
appear in older documents, source code, or proto schemas pending cleanup.

| Concept | Was in | Status | Notes |
|---------|--------|--------|-------|
| `clade` | `Identity.clade` in `manifest.proto`, `holon.yaml` | **removed** | Computational nature taxonomy (deterministic/pure, probabilistic/generative, etc.). No longer part of identity. |
| `reproduction` | `Lineage.reproduction` in `manifest.proto`, `holon.yaml` | **removed** | Birth mode taxonomy (manual, assisted, automatic, autopoietic, bred). No longer tracked. |
| `lineage` | `Lineage` message in `manifest.proto`, `holon.yaml` | **removed** | Origin/parentage tracking. `born` stays in `Identity`. `parents` and `reproduction` were the main fields — both removed. |
| `delegates` | `Delegates` message in `manifest.proto`, `holon.yaml` | **removed** | Wrapper holons no longer declare delegated commands separately. Wrapped commands go in `requires.commands`. |
| `holon.yaml` | all holons | **superseded** | Replaced by proto manifest (`option (holons.v1.manifest)`). Still read by `op` as migration fallback, but not for new holons. |

## Cleanup Tasks

- [ ] Remove `clade` from `Identity` in `manifest.proto`
- [ ] Remove `Lineage` message from `manifest.proto`
- [ ] Remove `lineage` field from `HolonManifest` in `manifest.proto`
- [ ] Remove `Delegates` message from `manifest.proto`
- [ ] Remove `delegates` field from `HolonManifest` in `manifest.proto`
- [ ] Update existing holons that reference deprecated fields in their proto manifests

## Active Specifications

The canonical specs are now:

| Document | Scope |
|----------|-------|
| `HOLON_PROTO.md` | Proto authoring — identity, contract, skills, sequences, guide |
| `HOLON_PACKAGE.md` | Package format — `.holon` directory, distribution, `op mod` |
| `HOLON_BUILD.md` | Build orchestration — runners, recipes, CLI contract |
| `OP.md` | CLI spec — discovery, lifecycle, transport, environment |