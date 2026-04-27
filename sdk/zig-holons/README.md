# zig-holons

Zig SDK for Organic Programming holons. The SDK targets parity with
`rust-holons`: `tcp://`, `unix://`, and `stdio://` can dial and serve;
`ws://`, `wss://`, and `rest+sse://` are dial-only; the hub API is client-only.

The public Zig API owns the orchestration surface. Vendored gRPC Core and
`libprotobuf-c` are private implementation dependencies. Normal holon builds
consume the native prebuilt installed by `op sdk install zig`; SDK contributors
can still build the local fallback with `zig build vendor`.

## serve

```zig
const holons = @import("zig_holons");

pub fn main() !void {
    holons.describe.useStaticResponse(.{ .json = "{\"holon\":\"demo\"}" });
    const opts = try holons.serve.parseOptions(&.{ "serve", "--listen", "stdio://" });
    try holons.serve.runSingle(opts);
}
```

## transport

```zig
const endpoint = try holons.transport.parse("tcp://127.0.0.1:9090");
_ = endpoint;
```

Serve transports are `stdio://`, `tcp://`, and `unix://`. Dial transports also
include `ws://`, `wss://`, and `rest+sse://`.

## identity / describe

```zig
holons.describe.useStaticResponse(gen.describe_generated.staticDescribeResponse());
```

`op build` writes the generated Incode Description source that supplies the
static response.

## discover

```zig
const found = try holons.discover.findBySlug(allocator, repo_root, "gabriel-greeting-zig");
defer allocator.free(found.path);
```

## connect

```zig
var conn = try holons.connect.connect(allocator, "tcp://127.0.0.1:9090");
defer holons.connect.disconnect(&conn);
```

## Holon-RPC dial transports

```zig
var ws = try holons.transport.ws.dial(allocator, "ws://127.0.0.1:8080/rpc");
defer ws.deinit();

var reply = try ws.invokeAlloc(allocator, "example.v1.Echo/Ping", "{\"message\":\"hello\"}");
defer reply.deinit(allocator);
```

```zig
var rest = try holons.transport.rest_sse.dial(allocator, "rest+sse://127.0.0.1:8080/api/v1/rpc");
defer rest.deinit();

var events = try rest.streamAlloc(allocator, "example.v1.Echo/Stream", "{\"message\":\"hello\"}");
defer events.deinit(allocator);
```

## hub client

```zig
var hub = try holons.hub.Client.connect(allocator, "wss://hub.example/holons");
defer hub.deinit();

var peers = try hub.invokeAlloc(allocator, "hub.v1.Hub/ListPeers", "{}");
defer peers.deinit(allocator);
```

## Build and test

```sh
op sdk install zig
op sdk verify zig
export OP_SDK_ZIG_PATH="$(op sdk path zig)"
zig build
zig build test
```

`op build` sets `OP_SDK_ZIG_PATH` automatically for holons that declare
`requires.sdk_prebuilts: ["zig"]`. Direct SDK builds set it explicitly with
`op sdk path zig`.

`build.zig` resolves native dependencies in this order: `OP_SDK_ZIG_PATH`, then
`.zig-vendor/native` for SDK contributors, then an actionable error pointing at
`op sdk install zig`.
