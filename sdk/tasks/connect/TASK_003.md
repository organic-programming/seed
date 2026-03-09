# TASK_003 — Fix Tier 2 + Tier 3 SDK `connect` Defaults

## Context

Depends on: `TASK_001` (Go reference).

Tier 2 SDKs have hello-world examples. Tier 3 are native SDKs.
All must default to `stdio`.

## Current state

| SDK | Default | stdio impl | Action |
|-----|:-------:|:----------:|--------|
| `js-holons` | ❌ `tcp` | ✅ | Fix default |
| `python-holons` | ❌ `tcp` | ✅ | Fix default |
| `ruby-holons` | ✅ `stdio` | ✅ | Verify |
| `kotlin-holons` | ? | ? | Audit + fix |
| `java-holons` | ? | ? | Audit + fix |
| `csharp-holons` | ? | ? | Audit + fix |
| `c-holons` | ? | ? | Audit + fix |
| `cpp-holons` | ? | ? | Audit + fix |
| `objc-holons` | ? | ? | Audit + fix |
| `js-web-holons` | N/A | N/A | Browser — no change |

## What to do

For each SDK marked with ❌ or ?:

- [ ] Audit the connect source for default transport value.
- [ ] If default is not `stdio`, change it.
- [ ] If stdio implementation is missing, implement it.
- [ ] Run tests. If no connect tests exist, add at minimum the
      "slug via stdio" test (test 2 from CONNECT.md).
- [ ] Commit per-SDK.

## Verification

Run each SDK's test suite. The specific command varies:

| SDK | Test command |
|-----|-------------|
| `js-holons` | `npm test` |
| `python-holons` | `python -m pytest` |
| `ruby-holons` | `bundle exec rake test` |
| `kotlin-holons` | `./gradlew test` |
| `java-holons` | `./gradlew test` |
| `csharp-holons` | `dotnet test` |
| `c-holons` | `cmake --build build && ctest --test-dir build` |
| `cpp-holons` | `cmake --build build && ctest --test-dir build` |
| `objc-holons` | `xcodebuild test` |

## Rules

- `js-web-holons` is exempt — browsers cannot spawn processes.
- Each SDK is a separate commit.
- If an SDK has no test infrastructure at all, add the stdio test
  as the first test.
