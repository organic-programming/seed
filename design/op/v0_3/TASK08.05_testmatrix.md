# TASK08.05 — Combinatorial Testing

## Summary

A Go program (`recipes/testmatrix/main.go`) that discovers and
runs all assemblies and composition recipes, producing a pass/fail
matrix. Reusable for CI and manual testing.

## Usage

```bash
go run recipes/testmatrix/main.go [--filter go-*] [--timeout 60s]
```

## How It Works

1. **Discover** — scan `recipes/assemblies/` and
   `recipes/composition/*/` for `holon.yaml`
2. **Build** — `op build` each (capture exit code + stderr)
3. **Run** — `op run` with timeout (default 30s)
4. **Verify** — assemblies: gRPC health check;
   composition: exit code 0 + expected output
5. **Report** — matrix table + JSON summary

## Output

```
Assemblies (daemon × hostui):
                swiftui  flutter  kotlin  web     dotnet  qt
  go            ✅       ✅       ✅      ✅      ❌      ⏭️
  rust          ✅       ✅       ❌      ✅      ✅      ⏭️
  ...

Composition (pattern × orchestrator):
                go       rust     swift   python  ...
  direct-call   ✅       ✅       ✅      ✅
  pipeline      ✅       ✅       ❌      ✅
  fan-out       ✅       ❌       ❌      ❌

Legend: ✅ pass  ❌ fail  ⏭️ skipped
Summary: 34/48 passed, 6 failed, 8 skipped
```

## Features

- `--format json` for CI integration
- `--filter "go-*"` / `--skip "qt,dotnet"` for selective testing
- Prerequisite detection: missing toolchain → ⏭️ not ❌
- `--dry-run` to list recipes without executing

## Documentation

The `recipes/README.md` must feature the matrix prominently:
1. "Try It Yourself" as the first section (one command)
2. "Current Matrix" as the second (real results, committed)
3. Architecture explanation comes after

## Acceptance Criteria

- [ ] Discovers assemblies and composition holons automatically
- [ ] Builds and runs each with timeout
- [ ] Detects missing toolchains (skip, not fail)
- [ ] Outputs table + JSON
- [ ] `recipes/README.md` showcases the matrix

## Dependencies

TASK08.03 (assemblies), TASK08.06 (composition recipes).
