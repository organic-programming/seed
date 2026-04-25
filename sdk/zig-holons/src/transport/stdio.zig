const uri = @import("uri.zig");

pub const Connection = struct {
    endpoint: uri.Endpoint,
    owns_child: bool = false,
};

pub fn validate(raw: []const u8) uri.ParseError!uri.Endpoint {
    const endpoint = try uri.parse(raw);
    if (endpoint.scheme != .stdio) return error.UnsupportedScheme;
    return endpoint;
}

pub fn connect(raw: []const u8) uri.ParseError!Connection {
    return .{ .endpoint = try validate(raw), .owns_child = false };
}
