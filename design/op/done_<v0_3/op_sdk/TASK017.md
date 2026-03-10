# TASK017 — Fix QA Blockers and Complete Recipe Runtime Verification

## Context

Depends on: `mac_TASK001`, `mac_TASK002`.

**Unblocks:** `QA_TASK001`, `AUDIT_TASK001`.

CODEX ran `QA_TASK001` and got all 12 recipe **builds** passing but
stopped before committing because end-to-end **runtime** verification
is still failing. This task fixes the identified issues and completes
the runtime sweep.

Until every recipe passes full lifecycle verification and commits are
landed, both QA and the documentation audit remain blocked.

---

## 1. Migrate SwiftUI recipes to SDK `connect`

**Root cause of the go-swift / rust-swift blocker.**

The 10 other recipes use their SDK's `connect(slug)` for gRPC channel
creation. The two SwiftUI recipes bypass the SDK entirely, dialing
with raw `grpc-swift`:

```swift
// WRONG — raw gRPC, no SDK
let channel = ClientConnection.insecure(group: group)
    .connect(host: host, port: port)
```

This must be replaced with `swift-holons` connect:

```swift
// CORRECT — SDK-managed connection
import SwiftHolons
let channel = try await SwiftHolons.connect(slug)
```

### `go-swift-holons`

- [ ] Add `swift-holons` as a dependency in `Package.swift`.
- [ ] Rewrite `GreetingClient.swift` to use
      `SwiftHolons.connect("gudule-greeting-goswift")`.
- [ ] Remove hardcoded `host`/`port` constants from `GreetingClient.swift`.
- [ ] Keep `DaemonProcess.swift` daemon launch logic unchanged (subprocess
      management is recipe-specific, not SDK territory).
- [ ] Build and verify: launch from Xcode, confirm language picker
      populates and `SayHello` RPC returns a greeting.

### `rust-swift-holons`

- [ ] Same migration with slug `"gudule-greeting-rustswift"`.
- [ ] Build and verify end-to-end.

---

## 2. Runtime-verify all 12 recipes

For each recipe, run the full `QA_TASK001` checklist
(Build → Launch → UI → RPC → Daemon lifecycle).

Fix any issue found, commit per-recipe. Apply the 3-attempt rule:
if stuck after 3 attempts, write `BLOCKED.md` and move on.

### Go-backend

- [ ] `go-swift-holons` — Xcode, run, greet.
- [ ] `go-web-holons` — `npm run dev`, open browser, greet.
- [ ] `go-qt-holons` — `cmake --build`, run binary, greet.
- [ ] `go-dart-holons` — `flutter run -d macos`, greet.
- [ ] `go-kotlin-holons` — `./gradlew run`, greet.
- [ ] `go-dotnet-holons` — `dotnet build -f net8.0-maccatalyst`, run, greet.

### Rust-backend

- [ ] `rust-swift-holons` — Xcode, run, greet.
- [ ] `rust-web-holons` — `npm run dev`, open browser, greet.
- [ ] `rust-qt-holons` — `cmake --build`, run binary, greet.
- [ ] `rust-dart-holons` — `flutter run -d macos`, greet.
- [ ] `rust-kotlin-holons` — `./gradlew run`, greet.
- [ ] `rust-dotnet-holons` — `dotnet build -f net8.0-maccatalyst`, run, greet.

---

## 3. Runtime-verify `examples/` hello-world matrix

For each of the 14 hello-world examples:

- [ ] Build and run tests.
- [ ] Run the connect example if present.
- [ ] Verify `holon.yaml` exists and is valid.

See `QA_TASK001.md` § "Hello-world examples" for the full list.

---

## 4. Update QA checklist and roadmap

- [ ] Fill in every cell in the `QA_TASK001` tables with ✅ or ❌.
- [ ] Write `BLOCKED.md` for any recipe still stuck after 3 attempts.
- [ ] Update `ROADMAP.md` Phase 6 status.

---

## Rules

- **SDK connect is mandatory.** Every recipe frontend must use its
  SDK's `connect(slug)` — no raw gRPC channel creation.
- Commit per-recipe, not as one giant commit.
- The task is complete only when every `QA_TASK001` row is ✅ or has a
  `BLOCKED.md`.
- Do not modify proto contracts or SDK library code — fixes go in the
  recipe/example source only.
- Follow the 3-attempt rule for any single blocker.
