# Implementation: Holon Discovery

Align `grace-op` code with the [Discovery Spec](../DISCOVERY.md) and [OP Discovery](../holons/grace-op/OP_DISCOVERY.md).

---

## Tasks

### Discover API

- [ ] Refactor `DiscoverHolons` into `Discover(holon, root, specifiers...)` — single public function
- [ ] Implement specifier-based layer filtering (siblings, cwd, source, built, installed, cached)
- [ ] Implement resolution order: siblings → cwd → source → built → installed → cached
- [ ] Multiple specifiers = union, resolved in default order (flag position is meaningless)

### `--root` flag

- [ ] `--root` is a global flag usable with any command
- [ ] When set, constrain all discovery to that root only
- [ ] When unset, root defaults to cwd
- [ ] Reject `--root` with empty or invalid path

### Unify `<holon>` parameter across all commands

Every command that resolves a holon must accept the same input: slug, alias, uuid, path, or binary path. Today, different commands use different resolution paths.

**Current state (code audit):**

| Code path | Used by | What it accepts |
|---|---|---|
| `ResolveTarget(ref)` in `discovery.go` | `op <holon>`, `op run` | slug, alias, `existingTargetDir` (cwd-relative) |
| `resolveHolonRef(ref)` in `cli.go` | `op build`, `op check`, `op test`, `op clean`, `op install` | slug, path (`.`, `./foo`), but separate resolution logic |
| `who.Show(ref)` in `who.go` | `op show` | UUID prefix |
| Custom parsing in `cli.go` | `op inspect` | slug, host:port |

**Target state:** one `Discover(holon, root, specifiers...)` call for all.

**Steps:**

- [ ] Audit all commands: identify where each parses `<holon>` → list each code path
- [ ] Route `op build`, `op check`, `op test`, `op clean`, `op install` through `Discover(holon, root, --source)`
- [ ] Route `op run` through `Discover(holon, root, --installed, --built, --siblings)`
- [ ] Route `op show` through `Discover(holon, root)` with UUID matching
- [ ] Route `op inspect`, `op tools`, `op mcp`, `op uninstall`, `op do` through `Discover(holon, root)`
- [ ] `op list [root]` stays as-is — positional arg is a directory, not a holon
- [ ] `op inspect` keeps `host:port` as an exception (skip Discover, connect directly)
- [ ] Remove `resolveHolonRef` — replaced by `Discover`
- [ ] Remove `ResolveTarget` — replaced by `Discover`

### Remove `op discover`

- [ ] Remove `op discover` command from CLI (`cli.go`, `cli_misc.go`)
- [ ] Remove `Discover` from code API (`api/public_identity.go`)
- [ ] Update all documentation references
- [ ] `op list` absorbs all discovery functionality

### Command special cases

- [ ] `op build` forces `--source` specifier, ignores others
- [ ] `op install` uses `--built` specifier
- [ ] `op run` uses `--installed --built --siblings`, with auto-build fallback from source
- [ ] `op run` default listen URI: `tcp://127.0.0.1:0` (not `stdio://`)
- [ ] `op run --listen stdio://` rejected

### `.holon` package execution

- [ ] `op run` handles `.holon` packages (`.holon.json` + binary, no `holon.proto` required)
- [ ] `runWithIO` resolves binary via `PackageBinaryPath` when `holon.proto` is absent

### `--bin` → `--origin`

- [ ] Rename `--bin` flag to `--origin`
- [ ] `--origin` outputs resolved path + layer to stderr
- [ ] Fix: `op run <holon> --origin` must not cause `op: holon "run" not found`

### SDK validation — `gabriel-greeting-app-swiftui`

The SwiftUI app is the proof that `Discover()` works end-to-end from the SDK side.

**Current state:** the app hardcodes holon descriptors (names, UUIDs, binary paths, sort ranks) in ~200-400 lines of plumbing code.

**Target state:** the app calls `swift-holons` SDK's `Discover()` to find its member holons dynamically.

- [ ] Implement `Discover(holon, root, specifiers...)` in `swift-holons` SDK (`pkg/discover`)
- [ ] Published app mode: scan `Contents/Resources/Holons/*.holon/`, read `.holon.json`
- [ ] Dev mode: delegate to `op list --format json` or reimplement the same walk
- [ ] Replace hardcoded descriptors in `gabriel-greeting-app-swiftui` with `HolonCatalog.discover()`
- [ ] Verify: app discovers all 12 greeting holons from its bundle at runtime
- [ ] Verify: adding/removing a `.holon` package from the bundle changes what the app discovers (no code change needed)



Two tiers: Go unit tests first (fast, prevent code-level regressions), then integration tests (full CLI pipeline).

#### Tier 1 — Go unit tests (`internal/holons/discovery_test.go`)

Extend existing test infrastructure (`writeDiscoveryHolon`, `chdirForHolonTest`, `t.Setenv`).

**`--root` / OPROOT behavior:**

- [ ] `TestDiscoverWithOPROOT` — `t.Setenv("OPROOT", root)`, call `DiscoverHolons(".")`, verify only holons under root are found
- [ ] `TestDiscoverOPROOTIgnoresCWD` — cwd has holons, OPROOT points elsewhere, verify cwd holons are NOT found
- [ ] `TestResolveTargetRespectsOPROOT` — slug exists under OPROOT but not cwd, verify it resolves
- [ ] `TestResolveTargetBareSlugIgnoresCWDWithOPROOT` — slug matches a cwd dir name, but OPROOT is set, verify OPROOT wins
- [ ] `TestDiscoverRejectsEmptyRoot` — empty string root returns error

**`.holon` package discovery:**

- [ ] `TestDiscoverFindsHolonPackage` — create a `foo.holon/` dir with `.holon.json`, verify discovered
- [ ] `TestDiscoverHolonPackageNeedsNoProto` — `.holon` package without `holon.proto`, verify discovered
- [ ] `TestResolveTargetFindsHolonPackageBySlug` — slug `foo` resolves to `foo.holon/`

**Alias resolution:**

- [ ] `TestResolveTargetByAlias` — holon with `aliases: ["op"]`, verify `ResolveTarget("op")` finds it
- [ ] `TestResolveTargetAliasWorksWithOPROOT` — same but with OPROOT set

**Specifier filtering (once Discover API is refactored):**

- [ ] `TestDiscoverWithSourceSpecifier` — only source holons returned
- [ ] `TestDiscoverWithInstalledSpecifier` — only `$OPBIN` packages returned
- [ ] `TestDiscoverIntersectionOrder` — `--installed --source` returns results in default order (source before installed)

#### Tier 2 — Integration tests (`scripts/test_tools/acceptance_test.sh`)

Runs the compiled `op` binary against real filesystem fixtures. Each test:
1. Creates a temp dir with controlled structure
2. Runs an `op` command
3. Asserts on exit code + output

```bash
# Fixture setup:
#   $FIXTURES/
#     workspace/
#       holons/gabriel-greeting-go/   (source holon with holon.proto)
#     isolated/
#       gabriel-greeting-go.holon/    (.holon package with .holon.json + binary)
#     empty/

# --- Root scoping ---
test_list_root_constrains_discovery()
  op list --root "$FIXTURES/isolated"     # finds gabriel-greeting-go
  op list --root "$FIXTURES/empty"        # finds nothing
  op list --root "$FIXTURES/workspace"    # finds source holon only

# --- Run behavior ---
test_run_defaults_to_tcp()
  op run gabriel-greeting-go 2>&1 | grep "tcp://"

test_run_rejects_stdio()
  op run gabriel-greeting-go --listen stdio://  # exit 1

test_run_holon_package()
  op run gabriel-greeting-go --root "$FIXTURES/isolated"  # succeeds

test_run_op()
  op run op  # alias resolution, must not fail

# --- Origin flag ---
test_origin_shows_path()
  op gabriel-greeting-go SayHello '{}' --origin 2>&1 | grep "origin:"
```

#### Execution order

1. Write Tier 1 Go tests **before** touching `discovery.go`
2. Run `go test ./internal/holons/...` — some will fail, documenting bugs
3. Fix `discovery.go` to make failing tests pass
4. **SDK validation: implement `Discover()` in `swift-holons`, integrate into `gabriel-greeting-app-swiftui`**
5. Write Tier 2 integration script after Go + Swift are validated
6. Run Tier 2 before every release

### Performance

- [ ] `op list --root ~/` remains under 30s on macOS (current: ~28s)

