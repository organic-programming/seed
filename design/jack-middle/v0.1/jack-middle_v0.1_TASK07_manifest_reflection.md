# TASK07 — Manifest and gRPC Reflection

## Context

Jack needs a `holon.yaml` manifest and gRPC reflection to
be a proper OP citizen.

## Objective

Create the manifest and enable reflection.

## Changes

### `holon.yaml` [NEW]

```yaml
schema: holon/v0
uuid: "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
given_name: Jack
family_name: Middle
motto: I see everything.
composer: B. ALTER
clade: deterministic/side-effects
status: draft
born: "2026-03-12"

description: |
  Transparent gRPC man-in-the-middle proxy for the OP ecosystem.
  Impersonates any holon, relays traffic to its real backend, and
  applies a configurable middleware chain for logging, tracing,
  metrics, recording, and fault injection.

contract:
  service: middle.v1.MiddleService

kind: native
build:
  runner: go-module
  main: ./cmd/jack-middle
artifacts:
  binary: jack-middle
```

### `cmd/jack-middle/main.go`

Add `reflection.Register(s)` call.

## Acceptance Criteria

- [ ] `holon.yaml` passes `op check`
- [ ] `op describe jack-middle` lists `middle.v1.MiddleService`
- [ ] gRPC reflection enabled

## Dependencies

TASK06 (control service registered).
