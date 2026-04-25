const holonrpc = @import("holonrpc.zig");

pub const Client = struct {
    rpc: holonrpc.Client,

    pub fn connect(raw_uri: []const u8) !Client {
        return .{ .rpc = try holonrpc.Client.connect(raw_uri) };
    }
};
