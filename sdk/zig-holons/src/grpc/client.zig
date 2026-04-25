const core = @import("core.zig");
const transport = @import("../transport.zig");

pub const Channel = struct {
    endpoint: transport.Endpoint,
};

pub fn init() void {
    core.init();
}

pub fn shutdown() void {
    core.shutdown();
}

pub fn connect(raw_uri: []const u8) transport.uri.ParseError!Channel {
    const endpoint = try transport.parse(raw_uri);
    if (!transport.supportsDial(endpoint.scheme)) return error.UnsupportedScheme;
    return .{ .endpoint = endpoint };
}
