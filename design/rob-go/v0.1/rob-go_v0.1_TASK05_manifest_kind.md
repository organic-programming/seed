# TASK05 — Update Manifest to `kind: toolchain`

## Context

Rob-Go's `holon.yaml` currently declares `kind: wrapper` and
`delegates.commands: [go]`. The new design introduces
`kind: toolchain` with a `delegates.toolchain` declaration.

## Objective

Update `holon.yaml` to reflect the toolchain identity.

## Changes

### `holon.yaml`

```yaml
kind: toolchain

# Remove:
# requires:
#   commands: [go]
# delegates:
#   commands: [go]

# Add:
delegates:
  toolchain:
    name: go
    version: "1.24.0"
    source: https://go.dev/dl/
```

Remove `requires.commands: [go]` — Rob no longer requires
system Go. He provides it.

Remove `requires.files: [go.mod]` — Rob does not require a
go.mod in his own directory to function (he has one, but it
is not a precondition for other holons calling him).

## Acceptance Criteria

- [ ] `kind: toolchain` in `holon.yaml`
- [ ] `delegates.toolchain` with name, version, source
- [ ] `requires.commands: [go]` removed
- [ ] `op check` accepts the new kind (or gracefully ignores unknown kinds)

## Dependencies

None (manifest change only).
