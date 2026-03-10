# TASK11 — Sequences (`op do`)

## Summary

Add `sequences:` section to `holon.yaml` and `op do` CLI command.
Sequences are deterministic, linear step sequences declared in the
manifest and executable via CLI or MCP tools.

## Changes

### 1. `OP.md` — Add `sequences:` fields

Add `### Sequences fields` section after `### Skills fields`.
Spec: [DESIGN_sequences.md](../DESIGN_sequences.md)

### 2. `op do` CLI command

New command in `grace-op`:

```bash
op do <holon> <sequence> [--param=value ...]
```

- Parse `sequences:` from `holon.yaml`
- Validate required params
- Substitute params via Go `text/template`
- Execute each `run:` step sequentially
- Stop on first non-zero exit code
- Report per-step progress

### 3. `op mcp` — Expose sequences as MCP tools

Extend `op mcp` to expose sequences as callable MCP tools
alongside individual RPCs. Tool name: `sequence_<name>`.

### 4. `op inspect` — Display sequences

Show declared sequences in `op inspect` output, similar to
how skills are displayed today.

## Acceptance Criteria

- [ ] `sequences:` parsed from `holon.yaml`
- [ ] `op do` executes steps with param substitution
- [ ] `op do --dry-run` prints steps without executing
- [ ] `op mcp` exposes sequences as MCP tools
- [ ] `op inspect` displays sequences
- [ ] OP.md updated with sequences spec

## Dependencies

None.
