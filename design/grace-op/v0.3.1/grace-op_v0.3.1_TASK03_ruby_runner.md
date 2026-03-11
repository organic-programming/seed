# TASK03 — Tier 2 Runner (ruby)

## Context

v0.3 introduced a runner registry with native build tools (`cargo`, `npm`, `dotnet`, etc.). However, Ruby holons were left relying on the verbose `recipe` runner (which requires manual `exec` YAML blocks for every step).

`ruby-holons` is a supported SDK in the ecosystem, and currently lacks a declarative runner. To provide a declarative experience, `grace-op` needs a dedicated `ruby` runner.

**Repository**: `organic-programming/holons/grace-op`

---

### `ruby` runner

For `build.runner: ruby`.

| Op | Command |
|---|---|
| Check | Verify `ruby` and `bundle` on PATH. |
| Build | `bundle install` |
| Test | `bundle exec rspec` or `bundle exec rake test`. |
| Clean | Remove `log/`, `tmp/`, `vendor/bundle/` (if locally cached). |

## Checklist

- [ ] Implement `RubyRunner`
- [ ] Unit tests for the runner's Check operation
- [ ] Integration: `op check` and `op build --dry-run`
- [ ] Update `runners` registry in `internal/holons/runner.go`
- [ ] `go test ./...` — zero failures

## Dependencies

- v0.3 TASK03 (runner registry)
