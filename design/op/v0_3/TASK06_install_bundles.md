# TASK06 — Install App Bundles to OPBIN

## Problem

`op install` only handles single-binary artifacts. Composites with
`.app` (macOS), `.exe` directories (Windows), or other bundle formats
are rejected:

```
op install: holon "gudule-greeting-rustkotlin" has non-binary primary
artifact ...; non-binary install is out of scope
```

## Objective

Make `op install <slug>` work for any artifact format — binaries,
`.app` bundles, `.exe` installers, anything a holon produces.

## Proposed Behavior

```
op install <slug>

  1. Resolve holon → read manifest
  2. Build if needed (same as op run)
  3. Determine artifact:
     - artifacts.binary         → single file
     - artifacts.primary_by_target → platform-specific bundle
  4. Install to $OPBIN:
     - Binary              → copy to $OPBIN/<slug>
     - .app bundle (macOS) → copy to $OPBIN/<slug>.app
     - .exe (Windows)      → copy to $OPBIN/<slug>.exe
  5. Print confirmation
```

### macOS .app specifics

`.app` bundles are directories. `op install` should:
- Copy the entire `.app` directory to `$OPBIN/<slug>.app`
- Optionally symlink to `/Applications/` with `--link-applications`
- `op run <slug>` then launches via `open $OPBIN/<slug>.app`

### Resolution in `op run`

`op run` must check for `$OPBIN/<slug>.app` in addition to
`$OPBIN/<slug>`. Launch via `open` on macOS for `.app` bundles.

## Changes

### `internal/cli/commands.go` — update `cmdInstall`

- Remove the "non-binary" guard
- Detect artifact type (file vs directory, `.app` suffix)
- Use `cp -R` for directory bundles
- Add `--link-applications` flag for macOS

### `internal/holons/discovery.go` — update `lookupBinaryOnSystem`

- Check for `$OPBIN/<name>.app` directories
- Return the `.app` path when found

### `internal/cli/commands.go` — update `cmdRun`

- When the resolved path is a `.app`, launch with `open` instead
  of direct exec

## Checklist

- [ ] Remove non-binary artifact guard in `cmdInstall`
- [ ] Handle directory-based artifacts (cp -R)
- [ ] Detect `.app` bundles and install to `$OPBIN/<slug>.app`
- [ ] Update `lookupBinaryOnSystem` to find `.app` in OPBIN
- [ ] Update `cmdRun` to launch `.app` via `open` on macOS
- [ ] Add `--link-applications` flag (optional)
- [ ] Test: `op install gudule-greeting-godart` → `.app` in OPBIN
- [ ] Test: `op run gudule-greeting-godart` → launches the `.app`
- [ ] Test: `op uninstall gudule-greeting-godart` → removes `.app`
- [ ] `go test ./...` — zero failures
