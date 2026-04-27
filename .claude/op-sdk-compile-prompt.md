# Claude prompt — extend `op sdk` with compile, source listing, and shell completion

> Hand this prompt to a fresh Claude Code session. Self-contained: the
> session has filesystem access to the `seed` repo. Do not assume any
> prior conversation context.

---

## Mission (one paragraph)

Round out the `op sdk` surface so developers can drive SDK natives end
to end from their checkout, not just from published releases. Three
linked features, all on one PR:

1. `op sdk compile <lang>` — build a SDK prebuilt from local sources
   (gRPC submodules + per-SDK build script) and install the resulting
   tarball into `$OPPATH/sdk`. Same on-disk layout as `op sdk install`,
   but sourced from `dist/sdk-prebuilts/...` instead of a release.
2. `op sdk list --compilable` (or whatever flag fits the existing
   `--installed` / `--available` shape) — list the SDKs for which a
   local source-build is *actually possible* on this checkout (script
   present, submodules initialised, prereq binaries on PATH). Today
   `op sdk list` only sees what is installed or what is on the release
   server.
3. **Cobra shell completion** for the whole `op sdk` family — positional
   `<lang>` completes against the right scope per verb (release for
   `install`, source for `compile`, installed for `uninstall`/`verify`/`path`),
   and `--target` / `--version` / `--lang` complete against real
   inventories. Reuse the in-repo pattern (`completeHolonSlugs` in
   `holons/grace-op/internal/cli/cmd_lifecycle.go`).

`compile` is the source-build counterpart of `install` (which downloads
a published release tarball). They are explicit alternatives — no silent
fallback from one to the other.

---

## Why this matters

Today `op sdk install <lang>` is the only way to land a prebuilt. When
the resolver can't find a published release, the user gets a hard
error:

```
op sdk install: no available sdk prebuilt release for zig aarch64-apple-darwin
```

That blocks every dependent flow (`op build gabriel-greeting-zig`,
`op run gabriel-greeting-app-swiftui --clean`, etc.). The compile path
already exists end-to-end — `.github/workflows/sdk-prebuilts.yml`
drives it via `.github/scripts/build-prebuilt-{zig,cpp,c,ruby}.sh` —
but it is only reachable through CI. A composer with `zig`, `cmake`,
`ninja`, `xcrun`, etc. on their machine *can* build the prebuilt; `op`
should let them.

The composer's preferred verb is **`compile`**, distinct from `install`:

- `op sdk install <lang>` — download a published release tarball (~30 s).
- `op sdk compile <lang>` — build from local sources (~30–60 min the
  first time, fast on cache hit). No fallback magic — the user opts in.

This keeps the surface honest: install is cheap, compile is expensive,
and the verb tells you which.

---

## State at session start

- `op sdk` already exposes: `install`, `list`, `uninstall`, `verify`,
  `path` (see [`holons/grace-op/internal/cli/cmd_sdk.go`](../holons/grace-op/internal/cli/cmd_sdk.go)).
- The proto service `Holon` exposes the matching RPCs at
  [`holons/grace-op/api/v1/holon.proto:215-227`](../holons/grace-op/api/v1/holon.proto). No `CompileSdkPrebuilt` rpc exists.
- `ListSdkPrebuiltsRequest` today has `installed`, `available`, `lang`
  (proto file, ~line 698). It does not surface "what could I compile
  from this checkout". The compile-side inventory has to be added.
- The compile machinery exists as four shell scripts and is wired into
  CI:
  - [`.github/scripts/build-prebuilt-zig.sh`](../.github/scripts/build-prebuilt-zig.sh)
  - [`.github/scripts/build-prebuilt-cpp.sh`](../.github/scripts/build-prebuilt-cpp.sh)
  - [`.github/scripts/build-prebuilt-c.sh`](../.github/scripts/build-prebuilt-c.sh) — delegates to cpp
  - [`.github/scripts/build-prebuilt-ruby.sh`](../.github/scripts/build-prebuilt-ruby.sh)
- The resolver/installer that handles release downloads:
  [`holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go`](../holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go) —
  `Install`, `Locate`, `Verify`, …
- `.gitmodules` lists `sdk/zig-holons/third_party/grpc` and
  `sdk/zig-holons/third_party/protobuf-c`. Compile requires those (and
  their nested submodules) checked out — the four CI scripts assume
  that.
- Cobra completion is already wired in this repo: `completeHolonSlugs`
  in [`holons/grace-op/internal/cli/cmd_lifecycle.go`](../holons/grace-op/internal/cli/cmd_lifecycle.go) is attached as
  `ValidArgsFunction` on `build`/`test`/`run`/etc. Same `cobra` import,
  same shell scripts (`completion_scripts.go`). Reuse the pattern; no
  new dependency. Viper handles config in this repo (env vars / flags),
  which is unrelated to completion — don't conflate the two.

---

## Required reading

1. [`holons/grace-op/api/v1/holon.proto`](../holons/grace-op/api/v1/holon.proto) §RPC list (lines 215-227) and §SDK message bodies (lines 678-732).
2. [`holons/grace-op/internal/cli/cmd_sdk.go`](../holons/grace-op/internal/cli/cmd_sdk.go) — the existing CLI sub-command shape.
3. [`holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go`](../holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go) — `Install`, `Locate`, `Verify`. The compile path should live alongside.
4. [`holons/grace-op/api/`](../holons/grace-op/api/) — the Go-side API wrappers (`InstallSdkPrebuilt`, etc.) — `compile` mirrors them.
5. [`.github/scripts/build-prebuilt-zig.sh`](../.github/scripts/build-prebuilt-zig.sh) and [`.github/scripts/build-prebuilt-cpp.sh`](../.github/scripts/build-prebuilt-cpp.sh) — the canonical compile flow per SDK; what `op sdk compile` will end up driving.
6. [`COAX.md`](../COAX.md) and [`CONVENTIONS.md`](../CONVENTIONS.md) — the surface-symmetry rule (`Code API = CLI = RPC = Tests`) the implementation must respect.
7. [`docs/specs/sdk-prebuilts.md`](../docs/specs/sdk-prebuilts.md) — release format, target list, version pinning. Compile must produce the same on-disk layout as install.
8. [`OP_SDK.md`](../OP_SDK.md) — user-facing doc for `op sdk`. Update once the verb lands.
9. [`CLAUDE.md`](../CLAUDE.md) — repo invariants, "doubt is the method".

---

## Implementation plan

### Step 0 — Verify scope with the composer if anything below smells off

Before writing code: re-read this prompt against the current state.
The architecture rules ("doubt is the method" in CLAUDE.md) hold —
flag conflicts rather than papering over them.

### Step 1 — Extend the proto

Two additions in [`holons/grace-op/api/v1/holon.proto`](../holons/grace-op/api/v1/holon.proto).

**1a — `CompileSdkPrebuilt` rpc.** In the `Holon` service block, next to the other SDK rpcs:

```proto
// CompileSdkPrebuilt builds a SDK prebuilt from local sources and
// installs the resulting tarball into $OPPATH/sdk. Long-running
// (~30–60 min for a cold cache).
rpc CompileSdkPrebuilt (CompileSdkPrebuiltRequest) returns (SdkPrebuiltResponse);
```

And the request message (mirror the shape of `InstallSdkPrebuiltRequest`):

```proto
message CompileSdkPrebuiltRequest {
  string lang = 1;
  string target = 2;
  string version = 3;
  // jobs caps the parallelism passed to cmake/ninja and zig build.
  // 0 = let the runtime pick (defaults match the env vars in the
  // current scripts: ZIG_HOLONS_JOBS, CPP_HOLONS_JOBS, etc.).
  int32 jobs = 4;
  // force re-runs the compile even if the target tarball is already
  // present in the work dir cache.
  bool force = 5;
  // install_after_build = true (default) installs into $OPPATH/sdk.
  // false leaves the tarball in dist/ for inspection.
  bool install_after_build = 6;
}
```

**1b — extend `ListSdkPrebuiltsRequest` with a `compilable` filter.**
Add field 4 to the existing message (additive, schema-compatible):

```proto
message ListSdkPrebuiltsRequest {
  bool installed  = 1;
  bool available  = 2;
  string lang     = 3;
  bool compilable = 4;  // SDKs that op sdk compile <lang> can build now
}
```

Either extend `SdkPrebuilt` or use the existing `notes` channel on
`ListSdkPrebuiltsResponse` to surface *why* a lang is or is not
compilable (script present? submodules initialised? `zig` /
`cmake` / `ninja` / `xcrun` on PATH?). Per-entry diagnostics on
`SdkPrebuilt` (e.g. a `string status` or `repeated string blockers`)
read better at the CLI; choose one and document the choice in the
PR body.

Reuse `SdkPrebuiltResponse` for the compile rpc. Regenerate the Go
bindings — this repo commits generated code (see invariant 7 in
CLAUDE.md). Run whatever `protoc` / `buf` invocation the repo uses;
do not hand-edit `holons/grace-op/gen/`.

### Step 2 — Implement compile + source listing in `internal/sdkprebuilts`

Two pieces in [`holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go`](../holons/grace-op/internal/sdkprebuilts/sdkprebuilts.go):

**2a — `ListCompilable(repoRoot string) ([]Prebuilt, []string, error)`.**
For each `lang` in `defaultVersions` (zig, c, cpp, ruby), check:

- Build script present at `.github/scripts/build-prebuilt-<lang>.sh`.
- Required submodules initialised (e.g. `sdk/zig-holons/third_party/grpc/CMakeLists.txt`
  for zig/cpp/c; `sdk/zig-holons/third_party/protobuf-c/build-cmake/CMakeLists.txt`
  for zig/c). Use the same paths the scripts already error out on.
- Required binaries on PATH per SDK: `zig` for zig/cpp/c, `cmake` for
  zig/cpp/c, `ninja` for zig/cpp/c, `xcrun` on darwin targets, `ruby` +
  `bundler` for ruby. Check; do not exec.

Emit one `Prebuilt` per *(lang, target=host)* with `Source` pointing at
the script path and either `Installed=false` or a new field describing
the compile-ready status. Push human-readable blockers (e.g. `zig SDK:
missing submodule sdk/zig-holons/third_party/grpc — run git submodule
update --init`) onto the `notes` slice or per-entry blocker list,
aligned with the proto choice in Step 1b.

**2b — `Compile(ctx context.Context, opts CompileOptions) (Prebuilt, []string, error)`.**
It should:

1. Normalise `lang`, `target`, `version` (reuse the existing
   `NormalizeLang`, `NormalizeTarget`, `NormalizeVersion` helpers).
2. Resolve the per-SDK build script. There are two viable approaches —
   pick one and justify in the PR:
   - **A — exec the on-disk script.** `Compile` finds the repo root
     (via `op env`) and `exec`s `.github/scripts/build-prebuilt-<lang>.sh`
     with the env vars already used in CI (`SDK_TARGET`, `SDK_VERSION`,
     `<LANG>_HOLONS_JOBS`, `MACOSX_DEPLOYMENT_TARGET`, `ZIG`, `RUBY` if
     ruby, etc.). Pros: zero duplication, matches CI exactly. Cons:
     `op` only works inside a clone with the scripts present (fine for
     the dev workflow, surprising for users who installed `op` from a
     release into a non-source dir).
   - **B — embed the scripts in the `op` binary** via `//go:embed`,
     materialise into a tempdir at runtime. Pros: `op sdk compile` works
     anywhere there is a checked-out source tree (with submodules).
     Cons: every script edit needs an `op` rebuild; drift risk if CI
     edits the scripts and `op` ships an older copy.

   Default recommendation: **start with A**. Embedding is a follow-up
   if/when `op` ships independently of the source tree.
3. Stream stdout/stderr line-by-line back to the CLI (the existing
   `op build` invocation pattern in `holons/grace-op/internal/cli/cmd_lifecycle.go`
   has a working pattern to follow). The compile is long-running — the
   user must see progress, not a 60-min freeze.
4. After the script exits 0, locate the produced tarball at
   `dist/sdk-prebuilts/<lang>/<target>/<lang>-holons-v<version>-<target>.tar.gz`
   and call the existing `Install(...)` path with `Source` pointing at
   the local tarball — that gives idempotent on-disk install with the
   same SHA verification, manifest writing, and tree-hash recording as
   the release path.
5. On failure, surface the script's exit code and a short tail of its
   stderr (don't dump the whole 50k-line ninja log).

### Step 3 — Wire the wrapper API

Mirror the existing `InstallSdkPrebuilt` etc. in `holons/grace-op/api/`
— add `CompileSdkPrebuilt(req *opv1.CompileSdkPrebuiltRequest) (*opv1.SdkPrebuiltResponse, error)`.

### Step 4 — Wire the RPC handler

Mirror the existing handler for `InstallSdkPrebuilt` (location
discoverable via `grep -rn "InstallSdkPrebuilt" holons/grace-op/internal/`).
Translate request → `CompileOptions` → `Compile(...)` → response.

### Step 5 — Wire the CLI sub-command

In [`holons/grace-op/internal/cli/cmd_sdk.go`](../holons/grace-op/internal/cli/cmd_sdk.go), add `newSdkCompileCmd()` next to `newSdkInstallCmd()` and register it in `newSdkCmd()`. Suggested shape:

```go
cmd := &cobra.Command{
    Use:   "compile <lang>",
    Short: "Compile a SDK prebuilt from local sources and install it",
    Long:  "compile builds a SDK prebuilt from the gRPC + per-SDK\n" +
           "sources in this checkout. Long-running (~30–60 min cold).\n" +
           "Use install instead when a published release is available.",
    Args:  cobra.ExactArgs(1),
    ValidArgsFunction: completeCompilableSdks,  // see Step 5b
    ...
}
cmd.Flags().String("target", "", "target triplet (defaults to host)")
cmd.Flags().String("version", "", "SDK prebuilt version")
cmd.Flags().Int("jobs", 0, "compile parallelism (0 = sensible default)")
cmd.Flags().Bool("force", false, "rebuild even if a cached tarball exists")
cmd.Flags().Bool("no-install", false, "leave tarball in dist/ instead of installing")
```

Map `--no-install` to `install_after_build = false` in the request.

Also extend `newSdkListCmd()` with a third boolean flag matching the
proto change:

```go
cmd.Flags().Bool("compilable", false, "list SDKs that op sdk compile can build now")
```

Mutually-exclusive enforcement (only one of installed/available/compilable
at a time) goes in the `RunE` — match the existing default-to-installed
behaviour at lines 164-166 of `cmd_sdk.go`.

### Step 5b — Cobra completion for the whole `op sdk` family

Mirror the in-repo pattern (`completeHolonSlugs` in [`cmd_lifecycle.go`](../holons/grace-op/internal/cli/cmd_lifecycle.go)) but
filtered per verb. Add a small helpers file (e.g.
`holons/grace-op/internal/cli/cmd_sdk_completion.go`) exposing:

```go
// All four return ([]string, cobra.ShellCompDirective).
func completeInstallableSdks(cmd, args, toComplete) // langs in "available" releases
func completeCompilableSdks(cmd, args, toComplete)  // langs ListCompilable says are buildable
func completeInstalledSdks(cmd, args, toComplete)   // langs already in $OPPATH/sdk
func completeAllSdks(cmd, args, toComplete)         // union (for filters / --lang)
```

All four should return `cobra.ShellCompDirectiveNoFileComp` so the
shell does not fall back to file completion. Wire them as
`ValidArgsFunction` per command:

| Command | `<lang>` completer |
|---|---|
| `op sdk install` | `completeInstallableSdks` |
| `op sdk compile` | `completeCompilableSdks` |
| `op sdk uninstall` | `completeInstalledSdks` |
| `op sdk verify` | `completeInstalledSdks` |
| `op sdk path` | `completeInstalledSdks` |

Then register flag completions on every relevant command:

```go
_ = cmd.RegisterFlagCompletionFunc("target", completeAllowedTargets)
_ = cmd.RegisterFlagCompletionFunc("version", completeVersionsForLang)
_ = cmd.RegisterFlagCompletionFunc("lang", completeAllSdks) // on `list`
```

`completeAllowedTargets` returns the keys of `allowedTargets` from
`sdkprebuilts.go`. `completeVersionsForLang` reads `args[0]` (the
already-typed lang) and returns the `defaultVersions[lang]` plus any
extra versions discovered in installed/available inventories (so a
user mid-flag can complete to a release tag they actually have).

**Performance constraint.** Cobra runs completion synchronously every
TAB. The "available releases" GitHub API call must be cached per shell
session (5-min TTL is fine — implement the same way the resolver
already memoises in `sdkprebuilts.go`). The "compilable" check must
not exec subprocesses except cheap `LookPath` calls. The "installed"
check is a directory walk under `$OPPATH/sdk` — already cheap.

If the network is offline, the completer should silently return
nothing for "available" rather than blocking the shell.

Sanity-test the result:

```bash
op completion bash > /tmp/op.bash && source /tmp/op.bash
op sdk install <TAB>     # → zig c cpp ruby
op sdk compile <TAB>     # → only langs whose script + submodules are present
op sdk path <TAB>        # → only langs already installed
op sdk install zig --target <TAB>   # → 8 target triplets
```

### Step 6 — Tests (COAX surface symmetry)

Per [`CONVENTIONS.md`](../CONVENTIONS.md) and [`COAX.md`](../COAX.md):
the new operation has to be exercised through *all three* facets
(Code API, CLI, RPC) and prove they produce identical observable state.

Concretely:

1. **Unit tests** under `holons/grace-op/internal/sdkprebuilts/` — small
   tests that verify `Compile` resolves the right script, fails clearly
   when prereqs are missing (no `zig`, no `cmake`, …), respects `--force`,
   and refuses unknown langs.
2. **`ader` integration triplet** under
   `ader/catalogues/grace-op/integration/sdk-compile/` — three checks
   (CLI / API / RPC) following the pattern in
   `ader/catalogues/grace-op/integration/<existing-cmd>/`. The shared
   helpers `SetupIsolatedOP`, `assertLifecycleEqual` already exist.
   Mark the triplet as `paused` in the suite YAML if compile is too
   slow/fragile to run on every CI pass — promote later.

The unit tests can mock the script (or shim it to `true`); the ader
triplet should run a real compile only if the runner can host it
(macOS dev machine). Document any opt-in env var (e.g.
`OP_TEST_SDK_COMPILE=1`).

### Step 7 — Docs

Update:

- [`OP_SDK.md`](../OP_SDK.md) — add the `compile` verb to the surface, document when to prefer it over `install`, and the prereq list per SDK (zig: `zig`, `cmake`, `ninja`, `xcrun` on darwin; cpp/c: same; ruby: `ruby`, `bundler`).
- [`docs/specs/sdk-prebuilts.md`](../docs/specs/sdk-prebuilts.md) — note that `op sdk compile` is the local counterpart of the release pipeline and shares the same build scripts.
- [`CONVENTIONS.md`](../CONVENTIONS.md) — only if anything you added here changed an invariant. Most likely no.

### Step 8 — Smoke-test on the composer's blocked path

The motivating use case is unblocking
`op run gabriel-greeting-app-swiftui --clean` when no `zig` release
exists. End-to-end check:

```bash
op sdk compile zig                                # ~30–60 min cold
op sdk path zig                                   # confirm install
op build gabriel-greeting-zig                     # builds without --source
op run gabriel-greeting-app-swiftui --clean       # composite up, 13 members
```

Document the wall-clock numbers in the PR body (cold + warm cache).

---

## PR conventions

- Branch name: `bpds/op-sdk-compile` or similar (composer is the actor).
- Base: `master`.
- Title: `feat(op): add op sdk compile`.
- One commit per logical step is fine; squash on merge if the composer
  prefers.
- PR body must include:
  - The verb chosen and why (link this prompt).
  - Approach A vs B for script discovery (and why).
  - Generated-code diff size (so reviewers know to skim, not read).
  - Output of `op sdk compile zig` cold + warm timings.
  - Evidence the COAX triplet passes:
    `ader test ader/catalogues/grace-op --suite sdk-compile --profile smoke --source workspace`.
  - Confirmation `op run gabriel-greeting-app-swiftui --clean` succeeds
    end-to-end on a fresh checkout.

---

## Operating mode

- **Halt at any real doubt.** Examples: the proto change breaks
  unrelated holons (regen failure); the script discovery resolves to
  the wrong path on a release-installed `op`; the ader helpers don't
  fit a long-running operation (compile can outlive a normal test
  step).
- **No silent fallback.** Do not make `op sdk install` quietly call
  compile — that was an explicit design choice (see "Why this matters").
  If install fails, the error already names compile as the alternative;
  refine the error string if needed, but don't auto-route.
- **Don't refactor the four build scripts in this PR.** They are
  CI-tuned. If something needs changing (e.g., to add a non-CI flag),
  add it conservatively and call it out.
- **Cap edits to non-generated files.** `holons/grace-op/gen/**` is
  produced by the proto pipeline — only the regen step writes there.
- **Do not publish anything externally** (no GitHub releases, no Slack
  messages, no force-pushes to `master`).

---

## Definition of done

- [ ] `op sdk compile <lang>` exists, with the same flag surface
      sketch above, and `op sdk --help` lists it next to `install`.
- [ ] `op sdk list --compilable` returns the source-buildable langs
      with per-entry blocker reasons when something is missing
      (submodule not initialised, `cmake` not on PATH, etc.).
- [ ] Cobra completion for `op sdk install/compile/uninstall/verify/path`
      and `--target` / `--version` / `--lang` flags. Manual smoke:
      `source <(op completion bash); op sdk compile <TAB>` lists only
      compilable langs.
- [ ] Proto regen committed; `go build ./...` and `go test ./...`
      green at the repo root.
- [ ] Unit tests cover: missing prereq error, unknown lang error,
      cached-tarball reuse, `--force` rebuild, and the four
      completion functions return the expected sets.
- [ ] `ader` triplet for `sdk-compile` exists and either passes or is
      explicitly paused in the suite YAML with a note.
- [ ] `OP_SDK.md` and `docs/specs/sdk-prebuilts.md` updated.
- [ ] On the composer's machine: `op sdk compile zig &&
      op build gabriel-greeting-zig &&
      op run gabriel-greeting-app-swiftui --clean` completes with all
      13 members up.
- [ ] PR opened against `master`, composer admin-merges, chantier
      closes.

Go.
