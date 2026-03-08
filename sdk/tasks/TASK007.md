# TASK007 — Implement `connect` in `objc-holons`

## Context

The Organic Programming SDK fleet requires a `connect` module in every SDK.
`objc-holons` uses a single-file architecture (`src/Holons.m` + `include/Holons/Holons.h`).
The `connect` functions must be added to these existing files.

The **reference implementation** is `go-holons/pkg/connect/connect.go` — study
it before starting.

## Workspace

- SDK root: `sdk/objc-holons/`
- Existing files: `include/Holons/Holons.h` (header), `src/Holons.m` (implementation)
- Reference: `sdk/go-holons/pkg/connect/connect.go`
- Spec: `sdk/TODO_CONNECT.md` § `objc-holons`

## What to implement

Add methods to the existing `Holons` class in `Holons.h` and `Holons.m`.

### Public API

```objc
+ (GRPCChannel *)connect:(NSString *)target;
+ (GRPCChannel *)connect:(NSString *)target options:(HolonsConnectOptions *)options;
+ (void)disconnect:(GRPCChannel *)channel;
```

### Resolution logic

Same 3-step algorithm:
1. `target` contains `:` → direct dial.
2. Else → slug → discover → port file → start → dial.

### Process management

- Use `NSTask` to launch the binary.
- Track started tasks in a static `NSMutableDictionary`.
- `disconnect:`: close channel, if ephemeral → `[task terminate]`, wait 2s,
  then `[task interrupt]`.
- Parse port from `NSPipe` attached to stdout/stderr.

### Port file convention

Path: `$CWD/.op/run/<slug>.port`
Content: `tcp://127.0.0.1:<port>\n`

## Testing

Add tests following existing patterns.

## Rules

- Follow existing code style in `Holons.m` — match the discover section.
- Use Foundation framework APIs (`NSTask`, `NSPipe`, `NSFileManager`).
- All existing tests must still pass.
