# TASK10 — Create Assembly Manifests

## Summary

Write thin composite `holon.yaml` manifests in `recipes/assemblies/`.
Each assembly references a daemon and a HostUI via relative paths.
No source code in assemblies — manifests only.

## Scale

8 daemons × 6 HostUIs = **48 assemblies**.

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
build:
  runner: recipe
  members:
    - path: ../../daemons/gudule-daemon-greeting-go
    - path: ../../hostui/gudule-greeting-hostui-flutter
```

## Acceptance Criteria

- [ ] 48 `holon.yaml` files created (names per [DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md))
- [ ] Each `family_name` matches the canonical names in DESIGN_recipe_monorepo.md §4
- [ ] Each builds with `op build`
- [ ] Each runs with `op run` (daemon + UI start together)

## Dependencies

TASK09.

## Reference

Full 48-entry assembly matrix with highlighted full-stack combos:
[DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md)
