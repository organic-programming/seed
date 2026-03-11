# TASK01 — Tier 2 Runner (python)

## Context

v0.3 introduced a runner registry with native build tools (`cargo`, `npm`, `dotnet`, etc.). However, Python holons were left relying on the verbose `recipe` runner (which requires manual `exec` YAML blocks for every step).

v0.4 (specifically `TASK07_dry_daemons_python_csharp_node.md`) mandates a standalone Python daemon. To provide a declarative experience on par with `npm` or `dotnet`, `grace-op` needs a dedicated `python` runner.

**Repository**: `organic-programming/holons/grace-op`

---

### `python` runner

For `build.runner: python`.

| Op | Command |
|---|---|
| Check | Verify `python3` (or `python`) on PATH. |
| Build | `python3 -m pip install -r requirements.txt` (if exists) or `pip install -r requirements.txt`. |
| Test | `python3 -m unittest discover` (if `tests/` exists) or `pytest` (if installed/found). |
| Clean | Remove `__pycache__`, `.pytest_cache`, `build/`, `dist/`. |

## Checklist

- [ ] Implement `PythonRunner`
- [ ] Unit tests for the runner's Check operation
- [ ] Integration: `op check` and `op build --dry-run`
- [ ] Update `runners` registry in `internal/holons/runner.go`
- [ ] `go test ./...` — zero failures

## Dependencies

- v0.3 TASK03 (runner registry)
