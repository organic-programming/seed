# TASK012 — Implement `connect` in `objc-holons`

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

### ConnectOptions

```objc
@interface HolonsConnectOptions : NSObject
@property (nonatomic) NSTimeInterval timeout;      // default 5.0
@property (nonatomic, copy) NSString *transport;   // @"stdio" (default), @"tcp" for override
@property (nonatomic) BOOL start;                  // YES = start if not running (default YES)
@property (nonatomic, copy) NSString *portFile;    // nil = use default
@end
```

### Resolution logic

Same 3-step algorithm:
1. `target` contains `:` → direct dial.
2. Else → slug → discover → port file → start with
   `serve --listen stdio://` (default) → dial over pipes.
   TCP fallback: `serve --listen tcp://127.0.0.1:0`.

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
