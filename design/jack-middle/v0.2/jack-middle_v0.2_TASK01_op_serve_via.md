# TASK01 — `op serve --via jack` in Grace

## Context

Grace (the OP orchestrator) must learn to inject Jack Middle
transparently when the operator requests proxy interposition.

## Objective

Add a `--via` flag to `op serve` that interposes Jack between
the caller and the target holon.

## Changes

### Grace `serve` handler

- Parse `--via jack` flag (with optional `--middleware` and `--plugin`)
- Launch target holon normally
- Launch Jack with `--target <target_uri>` + middleware/plugin flags
- Advertise Jack's frontend address as the visible endpoint
- Bind lifecycle: stopping Jack stops the target

### Grace CLI

```bash
op serve rob-go --via jack --middleware logger,metrics
```

## Acceptance Criteria

- [ ] `op serve rob-go --via jack` starts both Rob and Jack
- [ ] Callers see Jack's address when they `connect("rob-go")`
- [ ] `--middleware` and `--plugin` flags forwarded to Jack
- [ ] `op stop rob-go` stops both Jack and Rob

## Dependencies

Jack Middle v0.1 (fully functional proxy).
