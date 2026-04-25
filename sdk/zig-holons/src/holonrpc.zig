const std = @import("std");
const transport = @import("transport.zig");

pub const Request = struct {
    id: []const u8,
    method: []const u8,
    payload_json: []const u8,
};

pub const Response = struct {
    id: []const u8,
    payload_json: []const u8,
};

pub const Client = struct {
    endpoint: transport.Endpoint,

    pub fn connect(raw_uri: []const u8) transport.uri.ParseError!Client {
        const endpoint = try transport.parse(raw_uri);
        if (!transport.supportsDial(endpoint.scheme)) return error.UnsupportedScheme;
        return .{ .endpoint = endpoint };
    }
};

pub fn encodeRequest(allocator: std.mem.Allocator, request: Request) ![]u8 {
    return std.fmt.allocPrint(
        allocator,
        "{{\"id\":\"{s}\",\"method\":\"{s}\",\"payload\":{s}}}",
        .{ request.id, request.method, request.payload_json },
    );
}

test "encode JSON request frame" {
    const encoded = try encodeRequest(std.testing.allocator, .{
        .id = "1",
        .method = "HolonMeta.Describe",
        .payload_json = "{}",
    });
    defer std.testing.allocator.free(encoded);
    try std.testing.expectEqualStrings("{\"id\":\"1\",\"method\":\"HolonMeta.Describe\",\"payload\":{}}", encoded);
}
