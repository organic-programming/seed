# TODO Status Report

Date: 2026-03-09

Source documents checked:

- `TODO.md`
- `TODO_DISCOVER.md`
- `TODO_CONNECT.md`
- `TODO_MIGRATE_RECIPES.md`

## Overall verdict

`TODO.md` was stale. `discover` is now complete across the SDK fleet,
while `connect` is split between 7 verified SDKs and 7 SDKs that still
need verification or fixes.

The updated `TODO.md` table uses this rule:

- `✅` = module exists and its current tests pass in this checkout
- `❓` = code exists, but verification is incomplete or failing
- `❌` = not implemented

This is narrower than full Go feature parity. Some SDKs still have
reduced `serve` or transport scope even where a module now counts as
implemented and tested.

## `discover`

Complete — all SDKs.

Verified across all 14 SDKs:

- `go-holons`
- `rust-holons`
- `swift-holons`
- `dart-holons`
- `js-holons`
- `js-web-holons` (`discoverFromManifest(...)` browser variant)
- `kotlin-holons`
- `java-holons`
- `csharp-holons`
- `cpp-holons`
- `c-holons`
- `python-holons`
- `ruby-holons`
- `objc-holons`

Conclusion: `TODO_DISCOVER.md` is complete and can be marked complete.

## `connect`

Verified complete (`✅` in `TODO.md`):

- `go-holons`
- `swift-holons`
- `js-holons`
- `js-web-holons` (direct `host:port` browser variant only)
- `c-holons`
- `python-holons`
- `ruby-holons`

Implemented but not yet verified complete (`❓` in `TODO.md`):

- `rust-holons` — `cargo test --lib` fails `test_connect_slug_tcp_override_starts_binary_and_disconnect_stops_it` with `timed out waiting for holon startup`
- `dart-holons` — `connect.dart` exists, but no direct `connect` test coverage was found
- `kotlin-holons` — `Connect.kt` exists, but no direct `connect` test coverage was found
- `java-holons` — `Connect.java` exists, but no direct `connect` test coverage was found
- `csharp-holons` — `Connect.cs` exists, but no direct `connect` test coverage was found
- `cpp-holons` — `connect()` exists, but the connect portion of `make test` was skipped in this checkout because `grpc++` headers were unavailable
- `objc-holons` — the Objective-C test runner fails `connect stdio slug started process`

Conclusion: `TODO_CONNECT.md` remains open.

## Recipe Migration

Recipes migrated to SDK client primitives:

- `go-dart-holons`
- `go-kotlin-holons`
- `go-web-holons`
- `go-qt-holons`
- `go-dotnet-holons`

Recipes partially migrated:

- `go-swift-holons` — embedded macOS launch uses `swift-holons connect(slug)`, but the remote/direct path still builds a raw gRPC channel
- `rust-dart-holons` — Dart frontend uses `dart-holons.connect(...)`, but the Rust daemon still uses raw Rust server wiring
- `rust-swift-holons` — embedded macOS launch uses `swift-holons connect(slug)`, but the Rust daemon is still raw and the remote/direct path is still raw gRPC

Recipes still not migrated:

- `rust-kotlin-holons`
- `rust-web-holons`
- `rust-dotnet-holons`
- `rust-qt-holons`

Conclusion: `TODO_MIGRATE_RECIPES.md` remains open.

## Hello-World Migration

Fully on SDK helpers:

- `go-hello-world`
- `js-hello-world`
- `c-hello-world`
- `python-hello-world`
- `web-hello-world`

Partially migrated:

- `rust-hello-world` — uses `rust-holons` helpers and now has `connect_example.rs`, but the server path is still a manual tonic setup
- `swift-hello-world` — imports `Holons`, but is not yet a full SDK serve/connect example

Still raw baselines:

- `dart-hello-world`
- `kotlin-hello-world`
- `java-hello-world`
- `csharp-hello-world`
- `cpp-hello-world`
- `ruby-hello-world`
- `objc-hello-world`

Connect examples now present in:

- `rust-hello-world`
- `python-hello-world`

Conclusion: hello-world migration is still incomplete.

## Practical Summary

1. `discover` is complete and verified across all 14 SDKs.
2. `connect` is verified in 7 SDKs and still `❓` in 7 SDKs.
3. Recipe migration is at 5 complete, 3 partial, 4 not migrated.
4. Hello-world migration is at 5 full, 2 partial, 7 still raw.
5. `TODO_DISCOVER.md` can be marked complete; `TODO_CONNECT.md` and `TODO_MIGRATE_RECIPES.md` should remain open.
