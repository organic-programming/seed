# `op build` — Auto Ad-Hoc Signing for Bundles

## Problem

Code signing is **copy-pasted** across 8+ SwiftUI assembly
manifests. Every one contains an identical bash exec step:

```yaml
- exec:
    argv: [bash, -lc, "... codesign --force --deep --sign - \"$app_dir\""]
```

This is a DRY violation. Adding a new assembly requires
copying the same codesign boilerplate.

## Solution

The recipe runner automatically ad-hoc signs bundle artifacts
after assembly. No configuration needed.

---

## Behavior

When `op build` produces a bundle artifact (`.app`, `.framework`),
the recipe runner detects it and applies ad-hoc signing:

```bash
codesign --force --deep --sign - "$artifact"
```

### Detection logic

1. Read `artifacts.primary` from `holon.yaml`
2. If it ends with `.app` or `.framework` → auto-sign
3. If `--no-sign` flag is set → skip

### CLI

```bash
# Default: auto ad-hoc signing for bundles
op build

# Skip signing
op build --no-sign
```

One flag. No identity, no certificates, no env vars.

---

## Changes

### Recipe runner (`internal/holons/runner_recipe.go`)

After the final build step, before `assert_file`:

```go
if isBundleArtifact(manifest.Artifacts.Primary) {
    if !flags.NoSign {
        exec("codesign", "--force", "--deep", "--sign", "-", artifactPath)
    }
}
```

### Assembly manifests (8+ files)

**Remove** the hand-rolled exec step containing `codesign`:

```diff
       - copy:
           from: ...daemon...
           to: build/GreetingSwiftUI.app/Contents/Resources/daemon/...
-      - exec:
-          argv: [bash, -lc, "... codesign --force --deep --sign - ..."]
       - assert_file:
           path: build/GreetingSwiftUI.app/Contents/_CodeSignature/CodeResources
```

The assert on `_CodeSignature/CodeResources` stays — it validates
the runner did its job.

---

## Scope — What This Is NOT

This milestone is **only** auto ad-hoc signing. The following
are explicitly deferred to **v0.8 (Release Pipeline)**:

- Developer ID signing (Apple)
- EV code signing (Windows)
- App Store signing + provisioning (iOS)
- Notarization (`xcrun notarytool`)
- `build.sign` manifest section
- `--sign <identity>` flag
- `OP_SIGN_*` environment variables
- Cross-platform signing (osslsigncode)

See [v0.8/DESIGN_release_pipeline.md](../v0.8/DESIGN_release_pipeline.md)
for the full signing infrastructure.

---

## Acceptance Criteria

- [ ] `op build` auto-signs `.app` bundles (ad-hoc)
- [ ] `_CodeSignature/CodeResources` present after build
- [ ] `--no-sign` skips signing
- [ ] 8 SwiftUI assembly manifests updated (codesign exec removed)
- [ ] All assemblies still pass `assert_file` on CodeResources
