const transport = @import("../transport.zig");

pub const Server = struct {
    endpoint: transport.Endpoint,
    started: bool = false,
};

pub fn bind(raw_uri: []const u8) transport.uri.ParseError!Server {
    const endpoint = try transport.parse(raw_uri);
    if (!transport.supportsServe(endpoint.scheme)) return error.UnsupportedScheme;
    return .{ .endpoint = endpoint, .started = false };
}

pub fn start(server: *Server) void {
    server.started = true;
}

pub fn shutdown(server: *Server) void {
    server.started = false;
}
