# TASK009 — Migrate Rust-Backend Recipes to SDK `connect`

## Context

Depends on: TASK001 (rust connect) + frontend SDK connect tasks.

All six Rust-backend recipes need a **double migration**:
1. The **Rust daemon** must use `rust-holons` `serve` (currently raw Rust gRPC).
2. The **frontend** must use its SDK's `connect` (currently hardcoded addresses).

See `sdk/TODO_MIGRATE_RECIPES.md` for the full migration plan.

## Recipes to migrate

| Recipe | Daemon SDK | Frontend SDK | Frontend connect task dependency |
|---|---|---|---|
| `rust-dart-holons` | `rust-holons` | `dart-holons` | done (dart connect exists) |
| `rust-swift-holons` | `rust-holons` | `swift-holons` | TASK002 |
| `rust-kotlin-holons` | `rust-holons` | `kotlin-holons` | done (kotlin connect exists) |
| `rust-web-holons` | `rust-holons` | `js-web-holons` | TASK003 |
| `rust-dotnet-holons` | `rust-holons` | `csharp-holons` | done (csharp connect exists) |
| `rust-qt-holons` | `rust-holons` | `cpp-holons` | TASK006 |

## Daemon migration (all 6)

In each recipe's Rust daemon:
1. Replace hand-rolled gRPC server setup with `holons::serve::run()`.
2. Replace hand-rolled flag parsing with SDK's `--listen` convention.
3. Ensure `holon.yaml` has correct `artifacts.binary` so `connect` can find it.

Reference: see how Go daemons in `go-dart-holons` use `serve.Run()`.

## Frontend migration (all 6)

Replace hardcoded `localhost:PORT` with `connect("slug")` using each
frontend's SDK. Follow the same pattern as the Go-backend recipe migrations.

## Rules

1. **Non-regression**: every recipe that builds today must still build.
2. **One recipe at a time**: daemon first, then frontend, then commit.
3. **Backward compatible**: direct `host:port` must still work everywhere.
4. **No proto changes**.
