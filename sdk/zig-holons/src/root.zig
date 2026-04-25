//! Organic Programming SDK for Zig.
//!
//! The public Zig API keeps protobuf-c and gRPC Core behind idiomatic Zig
//! wrappers. The C ABI exported from `cabi.zig` uses opaque handles only.

pub const cabi = @import("cabi.zig");
pub const connect = @import("connect.zig");
pub const describe = @import("describe.zig");
pub const discover = @import("discover.zig");
pub const grpc = struct {
    pub const client = @import("grpc/client.zig");
    pub const core = @import("grpc/core.zig");
    pub const posix = @import("grpc/posix.zig");
    pub const server = @import("grpc/server.zig");
};
pub const holonrpc = @import("holonrpc.zig");
pub const hub = @import("hub.zig");
pub const identity = @import("identity.zig");
pub const observability = @import("observability.zig");
pub const protobuf = struct {
    pub const runtime = @import("protobuf/runtime.zig");
};
pub const serve = @import("serve.zig");
pub const transport = @import("transport.zig");

test {
    const std = @import("std");
    std.testing.refAllDecls(@This());
}
