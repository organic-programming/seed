# TASK01 — Create Assembly Manifests

## Summary

Write thin composite `holon.yaml` manifests in `recipes/assemblies/`.
Each assembly references a daemon and a HostUI via relative paths.
No source code in assemblies — manifests only.

> [!NOTE]
> **Web assemblies** (`*-web`) have a single member: the daemon with
> embedded web dist. No separate HostUI holon — the daemon serves
> the web client directly via Connect protocol.

## Scale

8 daemons × 6 HostUIs = **48 assemblies** total.
The 9 validated assemblies from the v0.4.2/TASK06 3×3 matrix already exist — this task creates the **remaining 39**.

```
recipes/assemblies/
├── gudule-greeting-flutter-go/holon.yaml
├── gudule-greeting-flutter-rust/holon.yaml
├── gudule-greeting-swiftui-go/holon.yaml
├── gudule-greeting-go-web/holon.yaml           ← reversed: daemon serves web
├── gudule-greeting-compose-go/holon.yaml
├── gudule-greeting-dotnet-go/holon.yaml
├── gudule-greeting-qt-go/holon.yaml
├── ...
└── gudule-greeting-qt-node/holon.yaml
```

## Manifest Template

```yaml
schema: holon/v0
kind: composite
given_name: gudule
family_name: Greeting-Flutter-Go
transport: tcp            # explicit — avoids stdio/tcp mismatch
build:
  runner: recipe
  members:
    - path: ../../daemons/gudule-daemon-greeting-go
    - path: ../../hostui/gudule-greeting-hostui-flutter
```

> [!WARNING]
> **Transport must be explicit.** Some SDKs default to `stdio`
> (Swift), others to `tcp` (Go, Kotlin). The assembly manifest must
> specify `transport:` to avoid `connect(slug)` timeout failures
> in cross-language combinations.

## Acceptance Criteria

- [ ] 39 new `holon.yaml` files created (48 total including the 9 from v0.4.2/TASK06; names per [DESIGN_recipe_monorepo.md](../v0.4/DESIGN_recipe_monorepo.md))
- [ ] Each `family_name` matches the canonical names in DESIGN_recipe_monorepo.md §4
- [ ] Each builds with `op build`
- [ ] Each manifest specifies `transport:` explicitly (tcp or stdio)

## Dependencies

v0.4.2/TASK06 (3×3 validation must pass first; its 9 assemblies already exist).

## Reference

Full 48-entry assembly matrix with highlighted full-stack combos:
[DESIGN_recipe_monorepo.md](../v0.4/DESIGN_recipe_monorepo.md)
