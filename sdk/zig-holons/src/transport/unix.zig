const uri = @import("uri.zig");

pub const Connection = struct {
    endpoint: uri.Endpoint,
};

pub fn validate(raw: []const u8) uri.ParseError!uri.Endpoint {
    const endpoint = try uri.parse(raw);
    if (endpoint.scheme != .unix) return error.UnsupportedScheme;
    return endpoint;
}

pub fn connect(raw: []const u8) uri.ParseError!Connection {
    return .{ .endpoint = try validate(raw) };
}
