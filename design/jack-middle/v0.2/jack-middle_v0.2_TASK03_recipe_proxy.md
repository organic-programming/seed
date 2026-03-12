# TASK03 — Recipe-Level Proxy Block

## Context

Recipes and assemblies can declare proxy interposition
per-member, so that `op build` automatically injects Jack
during test and integration runs.

## Objective

Parse and honor a `proxy` block in recipe member declarations.

## Changes

### Recipe schema extension

```yaml
members:
  - id: daemon
    path: rob-go
    proxy:
      middleware: [logger, metrics, recorder]
      plugins: [snoopy-inspect]
      record_dir: .op/traces/
```

### Grace `op build` / recipe runner

When a member has a `proxy` block:
1. Build and launch the member holon normally
2. Launch Jack with the declared middleware/plugin chain
3. Wire callers to Jack instead of the member directly

### Manifest parsing

- Parse `proxy.middleware` as `[]string`
- Parse `proxy.plugins` as `[]string`
- Parse `proxy.record_dir` as `string`
- Missing `proxy` block means no interposition (default)

## Acceptance Criteria

- [ ] `proxy` block parsed from recipe members
- [ ] Jack injected automatically during recipe assembly
- [ ] Middleware and plugin flags forwarded correctly
- [ ] Missing `proxy` block = no interposition

## Dependencies

TASK01 (`op serve --via`).
