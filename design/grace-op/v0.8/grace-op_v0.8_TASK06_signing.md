# TASK06 — Artifact Signing & Platform Code Signing

## Objective

Two signing layers for published artifacts:

1. **Artifact signing** — Ed25519 signature for registry integrity
2. **Platform code signing** — OS-level trust (Gatekeeper, SmartScreen, Play Store)

## Repository

- `grace-op`: `github.com/organic-programming/grace-op`

## Reference

- [DESIGN_release_pipeline.md](./DESIGN_release_pipeline.md) — §Security & Signing
- [v0.4.4/DESIGN_bundle_codesign.md](../v0.4.4/DESIGN_bundle_codesign.md) — ad-hoc baseline

## Scope

### Layer 1 — Artifact Signing (Ed25519)

- Sign each artifact with author's Ed25519 private key
- Attach `.sig` alongside artifact in registry
- `op install` verifies against author's public key
- Reject unsigned/tampered artifacts unless `--insecure`
- `op publish --key <path>` or `OP_SIGN_KEY` env var

### Layer 2 — Platform Code Signing

Parse `build.sign.<platform>` from `holon.yaml` and run the
appropriate platform-specific signing tool after build:

| Platform | Tool | Credential |
|---|---|---|
| macOS | `codesign` + optional `notarytool` | Developer ID + entitlements |
| Windows | `signtool.exe` or `osslsigncode` | EV certificate (.pfx) |
| Linux | `gpg --detach-sign` | GPG key |
| iOS | `codesign` + provisioning | iPhone Distribution |
| Android | `apksigner` or `jarsigner` | Keystore (.jks) |

### CLI

- `--sign` / `--sign <identity>` — platform code signing
- `--notarize` — Apple notarization (macOS)
- `--no-platform-sign` — skip code signing (Ed25519 still runs)
- `--insecure` — skip verification on install

### Environment Variables

All signing credentials via env vars (never in committed yaml):
`OP_SIGN_KEY`, `OP_SIGN_IDENTITY`, `OP_SIGN_CERT`, `OP_SIGN_PASS`,
`OP_GPG_KEY`, `OP_PROVISION`, `OP_ANDROID_KEYSTORE`,
`OP_ANDROID_ALIAS`, `OP_ANDROID_PASS`, `APPLE_ID`, `TEAM_ID`.

## Acceptance Criteria

- [ ] Artifacts signed with Ed25519 on publish
- [ ] Signatures verified on install
- [ ] Tampered artifact rejected
- [ ] `--insecure` bypasses verification
- [ ] macOS: `codesign` with Developer ID
- [ ] macOS: `--notarize` submits + staples
- [ ] Windows: `signtool` or `osslsigncode` with EV cert
- [ ] Linux: GPG detached signature
- [ ] iOS: `codesign` with provisioning profile
- [ ] Android: `apksigner` with keystore
- [ ] `go test ./...` — zero failures

## Dependencies

TASK02, TASK03, TASK04. Extends v0.4.4 (auto ad-hoc signing).
