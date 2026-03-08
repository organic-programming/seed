# TASK010 — Migrate Hello-World Examples to SDK `connect`

## Context

Depends on: all connect tasks (TASK001–TASK007).

Nine hello-world examples are still on raw gRPC baselines. They must be
migrated to use their SDK's `serve` and `connect` primitives, and a
`connect` example must be added to each.

See `sdk/TODO_MIGRATE_RECIPES.md` § "Hello-world examples" and
`sdk/TODO_STATUS_REPORT.md` § "Hello-world migration".

## Already migrated (reference patterns)

- `go-hello-world` — already uses `go-holons` SDK
- `js-hello-world` — already uses `js-holons` SDK
- `swift-hello-world` — already uses `swift-holons` SDK
- `c-hello-world` — already uses `c-holons` SDK
- `web-hello-world` — already uses `js-web-holons` / `go-holons`

## Hello-worlds to migrate

For each, the migration is:
1. Replace raw gRPC server setup with SDK's `serve` runner.
2. Replace raw gRPC client setup with SDK's `connect`.
3. Add a **connect example** showing slug-based resolution.

| Example | SDK | What to do |
|---|---|---|
| `rust-hello-world` | `rust-holons` | Migrate to `holons::serve`, add `holons::connect` example |
| `dart-hello-world` | `dart-holons` | Verify SDK usage, add connect example |
| `kotlin-hello-world` | `kotlin-holons` | Migrate to SDK serve + connect |
| `java-hello-world` | `java-holons` | Migrate to SDK serve + connect |
| `csharp-hello-world` | `csharp-holons` | Migrate to SDK serve + connect |
| `cpp-hello-world` | `cpp-holons` | Migrate to SDK serve + connect |
| `python-hello-world` | `python-holons` | Migrate to SDK serve + connect |
| `ruby-hello-world` | `ruby-holons` | Migrate to SDK serve + connect |
| `objc-hello-world` | `objc-holons` | Migrate to SDK serve + connect |

## Connect example pattern

Each hello-world should include a small `connect_example` (or equivalent) that:

```
1. Starts the echo-server in the background (or assumes it's built)
2. Calls connect("echo-server") using the SDK
3. Sends a Ping RPC
4. Prints the response
5. Calls disconnect()
```

Reference: see `go-hello-world` for the Go pattern.

## Rules

1. **Non-regression**: each example must still build and run.
2. **One example at a time**: migrate, test, commit.
3. **No proto changes**.
4. Follow each SDK's existing code style.
