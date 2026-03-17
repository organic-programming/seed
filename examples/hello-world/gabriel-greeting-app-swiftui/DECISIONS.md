## 2026-03-17 — Swift Host Terminology Rename
**Options considered:** Keep existing `daemon` naming and only adjust user-visible strings; rename only public symbols; rename all files, symbols, logs, comments, and strings to `holon`.
**Chosen approach:** Rename all Swift host file names, symbols, identifiers, logs, comments, and user-visible strings from `daemon` to `holon`.
**Rationale:** The task requires the word `daemon` to be entirely absent from the Swift host, and partial renames would leave an inconsistent API surface.

## 2026-03-17 — Staged Proto Layout
**Options considered:** Keep only `_protos`; switch entirely to `protos`; stage `protos` as canonical and mirror `_protos` for compatibility.
**Chosen approach:** Stage `protos` as the canonical layout and mirror `_protos` during this cleanup.
**Rationale:** Most SDKs default Describe metadata to `protos`, while Go-style logic still recognizes `_protos`; mirroring avoids regressions while normalizing the host.

## 2026-03-17 — C Holon Serving Structure
**Options considered:** Preserve the current example-owned serve lifecycle; rewrite the example to use SDK serve primitives and keep the HTTP backend only as a private bridge backend; replace the bridge pattern with a native gRPC implementation.
**Chosen approach:** Rewrite the public C `serve` path to use SDK-owned flag parsing and bridge delegation, while keeping the HTTP backend private.
**Rationale:** This matches the SDK-first rule, removes duplicated transport lifecycle logic from the example, and avoids a larger gRPC-native redesign.

## 2026-03-17 — C Bridge Packaging
**Options considered:** Keep copying the SDK shell bridge into the holon package; resolve a prebuilt bridge from the developer machine; build a self-contained bridge binary during the C holon build.
**Chosen approach:** Build a real `grpc-bridge` binary from the SDK's Go source during the C holon build and package it beside the public and backend binaries.
**Rationale:** The SDK shell bridge depends on its original repository layout, so copying it into the holon package is brittle. Building the binary produces a portable runtime layout that works from both source-tree and packaged installs.

## 2026-03-17 — Gradle Runtime Layout
**Options considered:** Rewrite the Java and Kotlin launchers; move the launcher back to `bin/`; keep the launcher in `bin/<arch>/` and copy `installDist/lib` into the package.
**Chosen approach:** Keep the architecture-specific launcher layout and copy Gradle's runtime `lib/` directory into `bin/lib/` inside the packaged holon.
**Rationale:** The generated Gradle launcher already resolves `APP_HOME/lib/*` from the parent of `bin/<arch>/`, so copying `lib/` is the smallest change that preserves the stock launcher behavior.

## 2026-03-17 — Ruby Bundle Isolation
**Options considered:** Continue using the developer's global gem set; vendor gems into `vendor/bundle`; install a project-local bundle under `.op/bundle` with an arm64 platform lock.
**Chosen approach:** Install Ruby gems into `.op/bundle`, force source builds for the current platform, and add the `arm64-darwin` platform to the lockfile during `op build`.
**Rationale:** The existing failures came from loading stale x86_64 native gems on arm64 macOS. A local, architecture-aware bundle isolates the holon from machine-global gem state and produces repeatable builds.

## 2026-03-17 — Script Wrapper Working Directory
**Options considered:** Keep Python and Ruby wrappers `cd`-ing into the source tree; rewrite the holons to pass explicit Describe metadata paths; preserve the caller's working directory and make the wrappers self-sufficient with absolute paths and bundle environment variables.
**Chosen approach:** Preserve the caller's working directory in Python and Ruby wrappers, using absolute entrypoint paths and Ruby bundle environment variables instead of `cd`.
**Rationale:** The Swift host stages `holon.yaml` and `protos` in a temporary working directory for Describe readiness. Wrappers that change directories hide that staged metadata and break host discovery even though the underlying scripts can run without changing cwd.

## 2026-03-17 — Python Interpreter Selection
**Options considered:** Keep using the first `python3` on `PATH`; rewrite the Python holon to vendor dependencies itself; prefer a project-local virtualenv interpreter when present and fall back to the host Python otherwise.
**Chosen approach:** Prefer the Python holon's local virtualenv interpreter for build, test, and launcher generation, and fall back to the host Python only when no project-local interpreter exists.
**Rationale:** The example already carries a `.venv` with the required gRPC and protobuf packages. Using that interpreter keeps dependency handling in the runner layer and avoids fragile source-level workarounds or host-global Python assumptions.

## 2026-03-17 — C++ HolonMeta Linking
**Options considered:** Rely on the C++ SDK headers alone; disable C++ Describe readiness in the host; compile and link the SDK's generated `holonmeta` sources into the example.
**Chosen approach:** Compile and link the SDK's generated `holonmeta` protobuf/grpc sources into the C++ example and expose their include path to the example target.
**Rationale:** The example had enabled `auto_register_holon_meta`, but without the generated `holonmeta` headers and objects the SDK compiled out the Describe service entirely. Linking the generated sources restores the intended SDK-first behavior without expanding the example's own server logic.
