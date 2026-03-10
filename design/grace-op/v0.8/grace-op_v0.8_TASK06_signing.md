# TASK06 — Artifact Signing & Verification

## Objective

Sign published artifacts with the holon author's key and verify
signatures on `op install`.

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md) — §Security

## Scope

### Signing (`op publish`)

- Sign each artifact with author's private key
- Attach signature alongside artifact in registry
- Support: Ed25519 keys (simple, no PKI overhead)

### Verification (`op install`)

- Download signature alongside artifact
- Verify against author's public key (from registry index)
- Reject unsigned or tampered artifacts
- `--insecure` flag to skip verification (dev/testing only)

### Key management

- `op publish --key <path>` or `OP_SIGN_KEY` env var
- Public key stored in registry index per holon
- Future: integrate with mesh CA for key distribution

## Acceptance Criteria

- [ ] Artifacts signed on publish
- [ ] Signatures verified on install
- [ ] Tampered artifact rejected
- [ ] `--insecure` bypasses verification
- [ ] `go test ./...` — zero failures

## Dependencies

TASK02, TASK03, TASK04.
