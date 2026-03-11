# TASK13 — Combinatorial Testing

## Summary

A Go program (`recipes/testmatrix/gudule-greeting-testmatrix/main.go`)
that discovers and runs all assemblies and composition recipes,
producing a pass/fail matrix. Reusable for CI and manual testing.

## Usage

```bash
go run recipes/testmatrix/gudule-greeting-testmatrix/main.go [--filter flutter-*] [--timeout 60s]
```

## How It Works

1. **Discover** — scan `recipes/assemblies/` and
   `recipes/composition/*/` for `holon.yaml`
2. **Build** — `op build` each (capture exit code + stderr)
3. **Run** — `op run` with timeout (default 30s)
4. **Verify** — automated tier: build success + gRPC health check
   + exit code 0. Full UI + RPC is manual verification.
5. **Report** — matrix table + JSON summary

## Output

```
Assemblies (daemon × hostui):
                flutter  swiftui  compose  web     dotnet  qt
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

TASK12.
