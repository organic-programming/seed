# TASK05 — CI Build Matrix Templates

## Objective

Create reusable CI workflow templates (GitHub Actions) that holon
authors can adopt to build and publish for all platforms.

## Repository

- `organic-programming/seed` or new `organic-programming/ci-templates`

## Reference

- [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md) — §CI Build Matrix, §Multi-Host CI

## Scope

### Workflow template

A reusable GitHub Actions workflow that:
1. Reads `build.publish.platforms` from `holon.yaml`
2. Dispatches builds to appropriate runners (macOS/Linux/Windows)
3. Runs `op build --target <platform>` per matrix entry
4. Runs `op publish` to upload artifacts

### Runner allocation

| Target group | CI runner |
|---|---|
| darwin-*, ios-* | `macos-latest` |
| linux-*, android-* | `ubuntu-latest` |
| windows-* | `windows-latest` |
| wasm | `ubuntu-latest` |

### Usage

```yaml
# .github/workflows/release.yml
jobs:
  publish:
    uses: organic-programming/ci-templates/.github/workflows/op-publish.yml@v1
    with:
      holon-path: .
```

## Acceptance Criteria

- [ ] Reusable workflow builds on all 3 runner OSes
- [ ] Platform dispatch correct per target group
- [ ] `op publish` runs after all builds succeed
- [ ] Template works with a reference holon (greeting-daemon-go)
- [ ] Documented in README

## Dependencies

TASK02 (`op publish`), TASK04 (registry).
