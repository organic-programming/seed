pub const rest_sse = @import("transport/rest_sse.zig");
pub const stdio = @import("transport/stdio.zig");
pub const tcp = @import("transport/tcp.zig");
pub const unix = @import("transport/unix.zig");
pub const uri = @import("transport/uri.zig");
pub const ws = @import("transport/ws.zig");
pub const wss = @import("transport/wss.zig");

pub const Scheme = uri.Scheme;
pub const Endpoint = uri.Endpoint;

pub fn parse(raw: []const u8) uri.ParseError!Endpoint {
    return uri.parse(raw);
}

pub fn supportsServe(scheme: Scheme) bool {
    return switch (scheme) {
        .stdio, .tcp, .unix => true,
        .ws, .wss, .rest_sse => false,
    };
}

pub fn supportsDial(scheme: Scheme) bool {
    return switch (scheme) {
        .stdio, .tcp, .unix, .ws, .wss, .rest_sse => true,
    };
}

test "transport matrix matches rust parity" {
    const std = @import("std");
    try std.testing.expect(supportsServe(.stdio));
    try std.testing.expect(supportsServe(.tcp));
    try std.testing.expect(supportsServe(.unix));
    try std.testing.expect(!supportsServe(.ws));
    try std.testing.expect(supportsDial(.wss));
    try std.testing.expect(supportsDial(.rest_sse));
}
