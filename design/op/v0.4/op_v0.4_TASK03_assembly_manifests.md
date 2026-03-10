# TASK03 — Create Assembly Manifests

## Summary

Write thin composite `holon.yaml` manifests in `recipes/assemblies/`.
Each assembly references a daemon and a HostUI via relative paths.
No source code in assemblies — manifests only.

## Scale

8 daemons × 6 HostUIs = **48 assemblies**.

```
recipes/assemblies/
├── go-swiftui/holon.yaml
├── go-flutter/holon.yaml
├── go-kotlin/holon.yaml
├── go-web/holon.yaml
├── go-dotnet/holon.yaml
├── go-qt/holon.yaml
├── rust-swiftui/holon.yaml
├── ...
└── node-qt/holon.yaml
```

## Manifest Template

```yaml
schema: holon/v0
kind: composite
given_name: gudule
family_name: greeting-go-swiftui
build:
  runner: recipe
  members:
    - path: ../../daemons/greeting-daemon-go
    - path: ../../hostui/greeting-hostui-swiftui
```

## Acceptance Criteria

- [ ] 48 `holon.yaml` files created
- [ ] Each builds with `op build`
- [ ] Each runs with `op run` (daemon + UI start together)

## Dependencies

TASK01, TASK02 (daemons and HostUIs must exist).

## Reference

Full 48-entry assembly matrix with highlighted full-stack combos:
[DESIGN_recipe_monorepo.md](./DESIGN_recipe_monorepo.md)
