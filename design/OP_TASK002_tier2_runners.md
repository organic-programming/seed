# OP_TASK002 — Tier 2 Runners (npm, gradle)

## Context

Depends on OP_TASK001 (runner registry must exist).

These runners support the web and JVM-based recipe frontends.

**Repository**: `organic-programming/holons/grace-op`

---

### `npm` runner

For `build.runner: npm`.

| Op | Command |
|---|---|
| Check | verify `node` and `npm` on PATH, `package.json` exists |
| Build | `npm ci && npm run build` |
| Test | `npm test` |
| Clean | `rm -rf node_modules dist build` |

### `gradle` runner

For `build.runner: gradle`.

| Op | Command |
|---|---|
| Check | verify `java` on PATH, `gradlew` or `gradle` exists |
| Build | `./gradlew build` (prefer wrapper) or `gradle build` |
| Test | `./gradlew test` |
| Clean | `./gradlew clean` |

Use `./gradlew` (Gradle wrapper) if present, fall back to system `gradle`.

## Checklist

- [ ] Implement `NPMRunner`
- [ ] Implement `GradleRunner`
- [ ] Unit tests for each runner's Check
- [ ] Integration: `op check` and `op build --dry-run`
- [ ] `go test ./...` — zero failures

## Dependencies

- OP_TASK001 (runner registry)
