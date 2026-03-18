# TASK02 — Stdio Pipeline Wiring

## Context

The default OP transport is stdio. Port file hijacking does not
work for stdio because there is no addressable endpoint. Grace
must construct a process pipeline for stdio-based interposition.

## Objective

Implement stdio pipeline wiring in Grace for the `--via jack`
flow.

## Changes

### Grace `serve` handler (stdio path)

When target uses stdio transport:

1. Launch target: `rob-go serve --listen stdio://`
2. Launch Jack: `jack-middle serve --listen stdio:// --target stdio://<fd>`
3. Pipe: caller's stdout → Jack's stdin → target's stdin
4. Pipe: target's stdout → Jack's backend stdout → caller's stdin

Essentially: `caller | jack | target` as a Unix pipeline,
but with gRPC frames instead of text.

### Pipe management

- Use `os.Pipe()` to create connected fd pairs
- Set `Cmd.Stdin` / `Cmd.Stdout` on child processes
- Handle EOF and process termination gracefully

## Acceptance Criteria

- [ ] Stdio-based holon proxied through Jack via pipeline
- [ ] gRPC calls work over the piped connection
- [ ] Clean shutdown when any process in the pipeline exits

## Dependencies

TASK01 (`op serve --via`).
