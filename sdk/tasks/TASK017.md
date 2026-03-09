# TASK017 — Fix QA Blockers from CODEX Audit Run

## Context

Depends on: `mac_TASK001`, `mac_TASK002`.

**Unblocks:** `QA_TASK001`, `AUDIT_TASK001`.

CODEX ran `QA_TASK001` and got all 12 recipe **builds** passing, but
stopped before committing because end-to-end **runtime** verification
is still failing. This task captures every blocker it identified and
adds the remaining runtime sweep it could not complete.

Until runtime is green and commits are landed, both QA and the
documentation audit remain blocked.

### What CODEX fixed (uncommitted, +1093 −617 across 34 files)

The build regressions were already patched:

| Area | Fix |
|------|-----|
| `greeting-goweb` / `greeting-rustweb` | Dropdown + button event wiring in `ui.ts`, layout in `style.css` |
| `greeting-goqt` / `greeting-rustqt` | Picker + button signal wiring in `MainWindow.cpp/.h` |
| `greeting-godart` / `greeting-rustdart` | Labels and widget keys in `greeting_screen.dart` |
| `greeting-gokotlin` / `greeting-rustkotlin` | Dropdown + button behavior in `ContentView.kt` |
| `greeting-godotnet` / `greeting-rustdotnet` | macOS targeting (`net8.0-maccatalyst`) and daemon shutdown in `.csproj` / `MainPage.xaml.cs` |
| `greeting-rustqt` | RPC client binary path in `GreetingClient.cpp` |

### What is still broken

`go-swift-holons` launch/UI/RPC lifecycle fails when the daemon is
started **by the bundled app** (autostart path). The SwiftUI client
works when pointed at an **already running** TCP daemon, proving the
gRPC layer and UI are correct. CODEX traced a bug in the migrated
`connect(slug)` path, replaced the launcher with a managed TCP
child-process approach with retries/timeouts, but the bundled autostart
still does not reliably pass.

`rust-swift-holons` likely inherits the same `DaemonProcess.swift` bug
but was not verified at runtime.

The remaining 10 recipes were not runtime-verified (build-only).

The `examples/` hello-world matrix was not started.

---

## What to do

### 1. Triage CODEX uncommitted changes

Review the 34 modified files, discard anything speculative, keep only
clean fixes.

- [ ] Checkout a branch from current `master`.
- [ ] Cherry-pick or re-apply the build fixes per recipe (use the diff
      summary above as guide).
- [ ] **Do not** carry forward the experimental `DaemonProcess.swift`
      TCP child-process rewrite — start fresh from master for that file.
- [ ] Commit per-recipe (one commit per recipe repo).

### 2. Fix `go-swift-holons` daemon autostart

The root issue: `DaemonProcess.swift` starts the Go daemon via
`Foundation.Process`, but the gRPC channel connects before the daemon
is listening. The `connect(slug)` migration may have introduced a race
or wrong endpoint.

- [ ] Verify the daemon binary name in `holon.yaml` matches the binary
      produced by `go build` (expected: `gudule-daemon-greeting-goswift`).
- [ ] Verify `DaemonProcess.swift` resolves the binary path correctly
      inside the app bundle (`Bundle.main.url(forAuxiliaryExecutable:)`).
- [ ] Verify the TCP port in `DaemonProcess.swift` matches the daemon's
      listen address (`127.0.0.1:9091`).
- [ ] Add a readiness check: after `Process.run()`, poll the TCP port
      (e.g. `connect()` in a retry loop with 100 ms intervals, 5 s
      timeout) before returning from `startDaemon()`.
- [ ] Verify `GreetingClient.swift` connects to the **same** port.
- [ ] End-to-end: build, launch from Xcode, confirm language picker
      populates and `SayHello` RPC returns a greeting.

### 3. Fix `rust-swift-holons` daemon autostart

Same `DaemonProcess.swift` pattern — apply the same fix with the Rust
binary name (`gudule-daemon-greeting-rustswift`).

- [ ] Apply the port-polling readiness check.
- [ ] Verify Cargo binary path resolution.
- [ ] End-to-end: build, launch, confirm greeting round-trip.

### 4. Runtime-verify remaining 10 recipes

For each recipe, run the **full QA_TASK001 checklist** (Build → Launch
→ UI → RPC → Daemon lifecycle):

#### Go-backend

- [ ] `go-web-holons` — `npm run dev`, open browser, greet.
- [ ] `go-qt-holons` — `cmake --build`, run binary, greet.
- [ ] `go-dart-holons` — `flutter run -d macos`, greet.
- [ ] `go-kotlin-holons` — `./gradlew run`, greet.
- [ ] `go-dotnet-holons` — `dotnet build -f net8.0-maccatalyst`, run, greet.

#### Rust-backend

- [ ] `rust-web-holons` — `npm run dev`, open browser, greet.
- [ ] `rust-qt-holons` — `cmake --build`, run binary, greet.
- [ ] `rust-dart-holons` — `flutter run -d macos`, greet.
- [ ] `rust-kotlin-holons` — `./gradlew run`, greet.
- [ ] `rust-dotnet-holons` — `dotnet build -f net8.0-maccatalyst`, run, greet.

If any recipe fails runtime verification, fix it and commit in the
same pass. Apply the 3-attempt rule: if stuck after 3 attempts, write
`BLOCKED.md` and move on.

### 5. Runtime-verify `examples/` hello-world matrix

For each of the 14 hello-world examples:

- [ ] Build and run tests.
- [ ] Run the connect example if present.
- [ ] Verify `holon.yaml` exists and is valid.

See `QA_TASK001.md` § "Hello-world examples" for the full list.

### 6. Update QA_TASK001 checklist

- [ ] Fill in every cell in the QA_TASK001 tables with ✅ or ❌.
- [ ] Write `BLOCKED.md` for any recipe still stuck after 3 attempts.
- [ ] Update `ROADMAP.md` Phase 6 status.

---

## Rules

- Commit per-recipe, not as one giant commit.
- The task is complete only when every QA_TASK001 row is ✅ or has a
  `BLOCKED.md`.
- Do not modify proto contracts or SDK library code — fixes go in the
  recipe/example source only.
- Follow the 3-attempt rule for any single blocker.
