const std = @import("std");
const grpc_client = @import("grpc/client.zig");
const transport = @import("transport.zig");

pub const Result = struct {
    channel: ?grpc_client.Channel,
    uri: []const u8,
};

pub const Error = transport.uri.ParseError || error{
    EmptyTarget,
};

pub fn connect(allocator: std.mem.Allocator, raw_uri: []const u8) !Result {
    if (std.mem.trim(u8, raw_uri, " \t\r\n").len == 0) return error.EmptyTarget;
    return .{
        .channel = try grpc_client.connect(allocator, raw_uri),
        .uri = raw_uri,
    };
}

pub fn disconnect(result: *Result) void {
    if (result.channel) |*channel| channel.deinit();
    result.channel = null;
}

test "connect parses URI" {
    var result = try connect(std.testing.allocator, "tcp://127.0.0.1:9090");
    defer disconnect(&result);
    try std.testing.expect(result.channel != null);
}
