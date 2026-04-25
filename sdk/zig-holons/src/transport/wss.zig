const uri = @import("uri.zig");

pub const Client = struct {
    endpoint: uri.Endpoint,
};

pub fn dial(raw: []const u8) uri.ParseError!Client {
    const endpoint = try uri.parse(raw);
    if (endpoint.scheme != .wss) return error.UnsupportedScheme;
    return .{ .endpoint = endpoint };
}
