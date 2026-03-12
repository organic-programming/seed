# Release Pipeline & Binary Distribution

## Problem

Users who don't have compilers (or the right compilers) can't
build holons from source. To make holon distribution practical,
we need a pipeline that produces pre-compiled binaries for every
supported platform and a registry where `op install` can fetch
the correct one.

## Architecture

```
Holon Author                    Holon Consumer
     │                               │
     │  op publish                    │  op install greeting
     ▼                               ▼
┌─────────────┐              ┌─────────────────┐
│ CI Matrix   │──────────────▶│ Holon Registry  │
│ (build all  │   platform-  │ (artifact store) │
│  platforms) │   tagged     └────────┬────────┘
└─────────────┘   artifacts          │
                                     │  resolve platform
                                     ▼
                              ┌─────────────────┐
                              │ Download binary  │
                              │ or              │
                              │ Fallback: build │
                              │ from source     │
                              └─────────────────┘
```

## Artifact Naming Convention

```
<holon>_<version>_<os>-<arch>[.<ext>]
```

Examples:
- `greeting-daemon-go_0.3.0_darwin-arm64`
- `greeting-daemon-go_0.3.0_windows-amd64.exe`
- `greeting-daemon-go_0.3.0_ios-arm64.xcframework`
- `greeting-daemon-go_0.3.0_wasm.wasm`

## CI Build Matrix

### GitHub Actions Template

A holon declares its build matrix in `holon.yaml`:

```yaml
build:
  runner: go-module
  targets:
    default:
      mode: binary
    ios:
      mode: framework
    wasm:
      mode: wasm
  publish:
    platforms:
      - darwin-arm64
      - darwin-amd64
      - linux-amd64
      - linux-arm64
      - windows-amd64
      - ios-arm64
      - android-arm64
      - wasm
```

`op publish` reads `build.publish.platforms` and produces one
artifact per entry. On CI, this runs on the appropriate host
(macOS for darwin/ios, Linux for linux/android/wasm, Windows
for windows).

### Multi-Host CI

Some targets must build on specific hosts:

| Target | CI host required |
|---|---|
| darwin-arm64 | macOS (native) |
| darwin-amd64 | macOS (native) |
| linux-amd64 | Linux or cross-compile |
| linux-arm64 | Linux or cross-compile |
| windows-amd64 | Windows or cross-compile |
| ios-arm64 | macOS (Xcode required) |
| android-arm64 | Any (NDK cross-compile) |
| wasm | Any (WASM tools) |

The CI workflow template dispatches to the right runner per
target group.

## `op publish` Command

```bash
# Build + upload all declared platforms
op publish

# Build + upload a single platform
op publish --platform darwin-arm64

# Dry run (build only, don't upload)
op publish --dry-run
```

### Steps

1. Read `build.publish.platforms` from `holon.yaml`
2. For each platform: `op build --target <platform>`
3. Name artifacts with the naming convention
4. Upload to the holon registry with version + platform tags
5. Update the registry index

## `op install` Platform Resolution

```bash
op install greeting-daemon-go
```

Resolution chain:
1. Query registry for `greeting-daemon-go@latest`
2. Match current platform (`darwin-arm64`)
3. If binary exists → download and install to `$OPBIN`
4. If no binary → clone source + `op build` (requires compiler)
5. Verify checksum (integrity)

## Holon Registry

### Options

| Option | Where artifacts live | Who hosts |
|---|---|---|
| GitHub Releases | GitHub release assets | GitHub |
| S3 / GCS bucket | Cloud object storage | Self-hosted |
| OCI Registry | Container registry tags | Any OCI host |
| Custom registry | Dedicated holon registry | Compilons |

The registry stores:
- Versioned artifacts per platform
- Checksums (SHA256) for integrity
- Metadata: holon name, version, build date, platforms available

### Index Format

```json
{
  "name": "greeting-daemon-go",
  "version": "0.3.0",
  "artifacts": [
    {"platform": "darwin-arm64", "url": "...", "sha256": "..."},
    {"platform": "windows-amd64", "url": "...", "sha256": "..."}
  ]
}
```

## Fallback to Source

When no pre-compiled binary is available:

```
op install greeting-daemon-go
→ Registry: no artifact for linux-riscv64
→ Fallback: clone source, op build
→ Requires: compiler toolchain installed
→ Cache: built artifact in ~/.op/cache/
```

The fallback makes the ecosystem work even for niche platforms
that aren't in the CI matrix.

## Security & Signing

Signing operates at **two layers**:

### Layer 1: Artifact Signing (registry integrity)

Every published artifact is signed with the holon author's
Ed25519 key. `op install` verifies the signature before
execution. This prevents tampering in transit/storage.

- Sign: `op publish` attaches `.sig` per artifact
- Verify: `op install` checks against author's public key
- Key: `--key <path>` or `OP_SIGN_KEY` env var
- Reject unsigned artifacts unless `--insecure`

### Layer 2: Platform Code Signing (OS-level trust)

OS-level code signing for distribution through stores,
enterprise deployment, and Gatekeeper/SmartScreen compliance.

#### `build.sign` manifest

```yaml
build:
  sign:
    macos:
      identity: Developer ID Application
      entitlements: entitlements.plist
      options: [runtime]       # hardened runtime for notarization
    windows:
      certificate: $OP_SIGN_CERT    # .pfx path
      password: $OP_SIGN_PASS       # cert password
      timestamp: http://timestamp.digicert.com
    linux:
      gpg_key: $OP_GPG_KEY         # GPG key ID or path
    ios:
      identity: iPhone Distribution
      provisioning: $OP_PROVISION    # .mobileprovision path
    android:
      keystore: $OP_ANDROID_KEYSTORE  # .jks or .keystore path
      alias: $OP_ANDROID_ALIAS
      password: $OP_ANDROID_PASS
```

#### Platform signing tools

| Platform | Tool | Identity type | Use case |
|---|---|---|---|
| **macOS** | `codesign` | Developer ID Application | Gatekeeper, outside App Store |
| **macOS** | `xcrun notarytool` | Apple ID + team | Notarization (required for distribution) |
| **Windows** | `signtool.exe` | EV code signing cert | SmartScreen, enterprise |
| **Windows** | `osslsigncode` | Same cert, cross-platform | Sign from macOS/Linux CI |
| **Linux** | `gpg --detach-sign` | GPG key | Package repos (apt, rpm, pacman) |
| **Linux** | `appimagesign` | GPG key | AppImage distribution |
| **iOS** | `codesign` | iPhone Distribution | App Store / TestFlight |
| **Android** | `apksigner` | Keystore | Play Store / sideload |
| **Android** | `jarsigner` | Keystore | Legacy APK signing |

#### CLI flags

```bash
# Sign with manifest-configured identity
op publish --sign

# Override identity
op publish --sign "Developer ID Application: Compilons"

# Notarize (macOS only, after signing)
op publish --notarize

# Skip platform signing (still does artifact Ed25519)
op publish --no-platform-sign
```

#### Environment variables

| Variable | Purpose |
|---|---|
| `OP_SIGN_KEY` | Ed25519 private key for artifact signing |
| `OP_SIGN_IDENTITY` | Default platform signing identity |
| `OP_SIGN_CERT` | Windows certificate path (.pfx) |
| `OP_SIGN_PASS` | Windows certificate password |
| `OP_GPG_KEY` | Linux GPG key ID or path |
| `OP_PROVISION` | iOS provisioning profile path |
| `OP_ANDROID_KEYSTORE` | Android keystore path (.jks) |
| `OP_ANDROID_ALIAS` | Android key alias |
| `OP_ANDROID_PASS` | Android keystore password |
| `APPLE_ID` / `TEAM_ID` | macOS notarization credentials |

#### Relationship to v0.4.4

v0.4.4 introduced **auto ad-hoc signing** for local development.
v0.8 extends this with real identities, EV certificates, and
store submission. The `build.sign` manifest replaces `--sign -`
with `--sign <identity>`.

## Dependency

- **Requires v0.7** — cross-compilation produces the
  platform-specific artifacts
- **Required by v0.11** — `op setup` installs holons from
  the registry
