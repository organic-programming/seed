# TASK008 — Migrate Go-Backend Recipes to SDK `connect`

## Context

Depends on: TASK002 (swift connect), TASK003 (js-web connect), TASK006 (cpp connect).

Three Go-backend recipes still use hardcoded `localhost:PORT` addresses.
They must be migrated to use the `connect("slug")` primitive from their
respective frontend SDKs.

See `sdk/TODO_MIGRATE_RECIPES.md` for the full migration plan.

Already migrated (for reference patterns):
- `go-dart-holons` — uses `dart-holons` `connect`
- `go-kotlin-holons` — uses `kotlin-holons` `connect`
- `go-dotnet-holons` — uses `csharp-holons` `Connect`

## Recipes to migrate

### 1. `go-swift-holons`

- Frontend: SwiftUI using `swift-holons`
- **Before**: `let channel = try GRPCChannelPool.with(target: .host("localhost", port: 9091))`
- **After**: `import Holons; let channel = try Holons.connect("gudule-daemon-greeting-goswift")`
- Reference: see `go-dart-holons` for the equivalent Dart migration pattern.

### 2. `go-web-holons`

- Frontend: Web (browser JS) using `js-web-holons`
- **Before**: hardcoded URL in gRPC-Web or WebSocket client initialization.
- **After**: `import { connect } from 'js-web-holons'; const client = connect("host:port")`
- Note: `js-web-holons` connect is direct-dial only (no slug resolution in browser).
  The daemon address must still be provided — but through `connect()` rather than raw
  channel construction. The backend Go daemon already uses `go-holons` SDK.

### 3. `go-qt-holons`

- Frontend: Qt/C++ using `cpp-holons`
- **Before**: `grpc::CreateChannel("localhost:9091", grpc::InsecureChannelCredentials())`
- **After**: `auto channel = holons::connect("gudule-daemon-greeting-goqt");`

## Rules

1. **Non-regression**: every recipe that builds today must still build after migration.
2. **One recipe at a time**: migrate, test, commit.
3. **Do not modify the Go backend daemon** — it already uses `go-holons` SDK.
4. **Backward compatible**: `connect("localhost:9091")` must still work.
5. **No proto changes**: gRPC contracts stay as-is.
