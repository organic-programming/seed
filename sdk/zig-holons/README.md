# zig-holons

Zig SDK for Organic Programming holons. The SDK targets parity with
`rust-holons`: `tcp://`, `unix://`, and `stdio://` can dial and serve;
`ws://`, `wss://`, and `rest+sse://` are dial-only; the hub API is client-only.

The public Zig API owns the orchestration surface. Vendored gRPC Core and
`libprotobuf-c` are private implementation dependencies, built from source by
`build.zig`.

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
var conn = try holons.connect.connect("tcp://127.0.0.1:9090");
defer holons.connect.disconnect(&conn);
```

## hub client

```zig
const hub = try holons.hub.Client.connect("wss://hub.example/holons");
_ = hub;
```

## Build and test

```sh
zig build
zig build test
```
