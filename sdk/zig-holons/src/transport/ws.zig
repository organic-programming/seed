const uri = @import("uri.zig");
const holonrpc = @import("../holonrpc.zig");

pub const Client = struct {
    rpc: holonrpc.Client,

    pub fn deinit(self: *Client) void {
        self.rpc.deinit();
        self.* = undefined;
    }

    pub fn invokeAlloc(
        self: *Client,
        allocator: @import("std").mem.Allocator,
        method: []const u8,
        params_json: []const u8,
    ) !holonrpc.InvokeResult {
        return self.rpc.invokeAlloc(allocator, method, params_json);
    }
};

pub fn dial(allocator: @import("std").mem.Allocator, raw: []const u8) !Client {
    const endpoint = try uri.parse(raw);
    if (endpoint.scheme != .ws) return error.UnsupportedScheme;
    return .{ .rpc = try holonrpc.Client.connect(allocator, raw) };
}
