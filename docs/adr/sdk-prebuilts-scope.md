# ADR: SDK Prebuilts Scope and M0 Gate

Status: Proposed, M0 gate passed

Date: 2026-04-26

## Context

`docs/specs/sdk-prebuilts.md` defines an `op sdk` subsystem for the SDKs whose
native runtime dependencies cause substantive cold-build pain. The grounding
audit is `docs/audit/sdk-toolchain-audit.md`.

The v1 chantier scope is deliberately narrow. In scope:

- `ruby`
- `c`
- `cpp`
- `zig`

Out of scope:

- `go`
- `rust`
- `dart`
- `js`
- `js-web`
- `swift`
- `java`
- `kotlin`
- `csharp`
- `python`

The spike touched no SDK source. It ran outside the repository in
`/tmp/seed-prebuilts-m0-spike/`; this phase commits ADRs only.

## Composer Decisions

The 8 questions in spec §11 have been arbitrated. These decisions are binding
constraints for this chantier:

1. Python OUT of v1. Add as v1.1 only if Alpine demand emerges. Do not include sdk/python-holons in the prebuilts pipeline this chantier.

2. popok-first runner strategy, transitional GitHub-hosted at kickoff. Target state per spec §6.2: popok (self-hosted Apple Silicon Mac) hosts most targets via Docker / containers, and winwok (a separate Windows mini-PC) hosts the Windows target. Both runners live at Saint-Émilion on residential fibre — see docs/st_emilions_runners.md for the deployment plan.

Important transitional reality: at the time this chantier kicks off (right after the in-flight Zig P12 merges), the Saint-Émilion physical deployment has NOT yet happened (scheduled Thursday 2026-04-30). Therefore:

All workflows ship initially with GitHub-hosted runners (runs-on: macos-14, ubuntu-latest, ubuntu-24.04-arm, windows-latest). This is the explicit transitional default per docs/st_emilions_runners.md §1.
The first complete release cycle on GitHub-hosted validates the workflow logic itself.
A separate follow-up PR (small, workflow-YAML-only) swaps runs-on: to the self-hosted labels ([self-hosted, popok, macos], [self-hosted, popok, linux-via-docker], [self-hosted, winwok, windows]) once both runners are operational at Saint-Émilion and the composer signals readiness.
Do not delay any of the 10 chantier PRs waiting for Saint-Émilion; the transition is orthogonal.

3. cosign keyless signing at v1.0. v0.x ships SHA-256 only.

4. Manifest field name sdk_prebuilts as Requires.sdk_prebuilts: repeated string. Add to holons/grace-op/_protos/holons/v1/manifest.proto.

5. T0 + T1 only in v1 (7 targets). T2 mobile (iOS, Android) deferred to v1.5. Do not add iOS/Android workflow stubs in this chantier.

6. Stripped main archive + sidecar debug archive per platform: .dSYM/ (macOS), .pdb (Windows), -debug.tar.gz (Linux).

7. Cache + prebuilts are complementary. Keep the existing actions/cache@v4 block in ader.yml; add the prebuilts-output cache key separately.

8. Hardened mobile composites are an implementation detail of the composite recipe runner, lands with T2 in v1.5. Out of scope for this chantier.

## M0 Spike Evidence

The spike built gRPC `v1.80.0` and protobuf-c `v1.5.2` from pinned source
checkouts:

```text
grpc v1.80.0       f5e2d6e856176c2f6b7691032adfefe21e5f64c1
protobuf-c v1.5.2  4719fdd7760624388c2c5b9d6759eb6a47490626
```

| Target | Mechanism | Wall time | Archive | SHA-256 | Result |
|---|---:|---:|---:|---|---|
| `aarch64-apple-darwin` | native macOS on Apple Silicon | 434s | 37M | `dd584ce3dc74ac9ad65a1c8d0e9082f01125f7fdd2e5db02bae0ed6c2252bcad` | green |
| `x86_64-unknown-linux-gnu` | Docker `linux/amd64` with QEMU emulation | 2029s | 45M | `28bf84bab49a0941f415763f10eeae8b2ef8d6931f6b792b31004da024ef0384` | green |
| `aarch64-unknown-linux-gnu` | Docker `linux/arm64` native on Apple Silicon | 787s | 45M | `4e68e83c19a4458febfe9536eb437978682bee22e93af6cbfa12b1d713ec54c5` | green |

Each output was packaged as:

```text
cpp-m0-1.80.0-<target>.tar.gz
cpp-m0-1.80.0-<target>.tar.gz.sha256
cpp-m0-1.80.0-<target>.spdx.json
```

The SBOMs were intentionally stub SPDX JSON files. Full SBOM generation lands
with the production workflow phase.

## Windows Decision

Decision: **Path B** for M0 and the initial v1 workflow.

The local Windows VM probe on the Apple Silicon host reported:

```text
timestamp=2026-04-26T12:12:49Z
host=macbpds.home
tart=missing
utmctl=missing
```

No UTM or Tart Windows VM was available, so no comparable local Windows timing
could be captured. No GitHub-hosted Windows job was run in M0.

The initial workflow therefore uses GitHub-hosted `windows-latest` for
`x86_64-pc-windows-msvc`, matching the transitional runner policy. When winwok
is operational at Saint-Émilion and the composer signals readiness, the
workflow-only transition PR moves the Windows target to
`[self-hosted, winwok, windows]`.
