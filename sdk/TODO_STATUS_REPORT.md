# TODO Status Report

Date: 2026-03-07

Source documents checked:

- `TODO.md`
- `TODO_DISCOVER.md`
- `TODO_CONNECT.md`
- `TODO_MIGRATE_RECIPES.md`

## Overall verdict

`TODO.md` is now stale.

It still reports `discover` and `connect` as missing in most SDKs, but
the workspace already contains much more than that. At the same time,
the high-level 5-module goal is still not fully achieved across the
fleet, and the recipe/example migration work is only partially done.

## What is effectively complete

### `discover`

`discover` is implemented across the SDK fleet, with the expected
browser-specific adaptation in `js-web-holons`:

| SDK | Status | Evidence |
|---|---|---|
| `go-holons` | complete | `pkg/discover/discover.go` |
| `rust-holons` | complete | `src/discover.rs` |
| `swift-holons` | complete | `Sources/Holons/Discover.swift` |
| `dart-holons` | complete | `lib/src/discover.dart` |
| `js-holons` | complete | `src/discover.js` |
| `js-web-holons` | adapted | `src/discover.mjs` (`discoverFromManifest`) |
| `kotlin-holons` | complete | `Discover.kt` |
| `java-holons` | complete | `Discover.java` |
| `csharp-holons` | complete | `Holons/Discover.cs` |
| `cpp-holons` | complete | `include/holons/holons.hpp` |
| `c-holons` | complete | `include/holons/holons.h` + `src/holons.c` |
| `python-holons` | complete | `holons/discover.py` |
| `ruby-holons` | complete | `lib/holons/discover.rb` |
| `objc-holons` | complete | `include/Holons/Holons.h` + `src/Holons.m` |

Conclusion: `TODO_DISCOVER.md` is functionally done.

### Documentation audit

The documentation audit is also done enough to stop treating it as an
open blocker:

- `sdk/SDK_GUIDE.md` now reflects actual SDK and recipe state.
- `sdk/README.md` now reflects actual discover/connect coverage.
- Per-SDK READMEs now distinguish between implemented modules and gaps.

## What is still missing

### 1. `connect` is only partially complete

The roadmap-level `connect` module is implemented in these SDKs:

- `go-holons`
- `dart-holons`
- `js-holons`
- `python-holons`
- `kotlin-holons`
- `java-holons`
- `csharp-holons`

The roadmap-level `connect` module is still missing in these SDKs:

- `rust-holons`
- `swift-holons`
- `js-web-holons`
- `ruby-holons`
- `c-holons`
- `cpp-holons`
- `objc-holons`

Notes:

- `c-holons` has low-level dial helpers, but not the TODO-defined
  slug-aware `connect` module.
- `js-web-holons` has no `connect(hostPort)` helper yet, even though
  `TODO_CONNECT.md` explicitly called for a browser-limited variant.

Conclusion: `TODO_CONNECT.md` is still open.

### 2. The 5-module architecture is still not true in the strict sense

`TODO.md` defines `serve` as:

`parse flags, listen, run gRPC server, shutdown`

That is still not true across the full fleet.

SDKs that currently have a real `serve` runner:

- `go-holons`
- `js-holons`
- `python-holons`
- `c-holons`

SDKs that currently expose only serve-flag parsing or partial serve
helpers:

- `rust-holons`
- `swift-holons`
- `dart-holons`
- `kotlin-holons`
- `java-holons`
- `csharp-holons`
- `cpp-holons`
- `ruby-holons`
- `objc-holons`

Conclusion: even after discover/connect progress, the master TODO goal
"every SDK must expose these 5 modules" remains incomplete if `serve`
is interpreted strictly.

### 3. Recipe migration is only partially complete

Desktop recipe migrations completed:

- `go-dart-holons`
- `go-kotlin-holons`
- `go-dotnet-holons`

Recipe migrations still missing:

- `go-swift-holons`
- `go-web-holons`
- `go-qt-holons`
- `rust-dart-holons` (frontend uses `dart-holons`, but the recipe still
  runs against fixed localhost TCP and the daemon is still raw Rust)
- `rust-swift-holons`
- `rust-kotlin-holons`
- `rust-web-holons`
- `rust-dotnet-holons`
- `rust-qt-holons`

Conclusion: `TODO_MIGRATE_RECIPES.md` is still open.

### 4. Hello-world migration is still missing

The roadmap also called for migrating hello-world examples and adding
connect examples. That is not done.

Hello-worlds currently importing their matching SDK:

- `go-hello-world`
- `js-hello-world`
- `swift-hello-world`
- `c-hello-world`
- `web-hello-world` (browser half via synced `js-web-holons`, backend via `go-holons`)

Hello-worlds still on raw gRPC baselines:

- `rust-hello-world`
- `dart-hello-world`
- `kotlin-hello-world`
- `java-hello-world`
- `csharp-hello-world`
- `cpp-hello-world`
- `python-hello-world`
- `ruby-hello-world`
- `objc-hello-world`

No fleet-wide hello-world `connect` examples were added.

## Practical summary

If `TODO.md` is used as the source of truth today, these are the real
remaining items:

1. Finish `connect` in `rust`, `swift`, `js-web`, `ruby`, `c`, `cpp`,
   and `objc`.
2. Decide whether the master roadmap still requires a full `serve`
   runner in every SDK. If yes, that remains a major open tranche.
3. Finish recipe migration for `go-swift`, `go-web`, `go-qt`, and all
   Rust-backed recipes.
4. Migrate the remaining raw hello-world examples and add the missing
   `connect` examples.
5. Update `TODO.md` itself so its "Current state per SDK" table matches
   the current repository state.
