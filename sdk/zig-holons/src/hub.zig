const holonrpc = @import("holonrpc.zig");

pub const Client = struct {
    rpc: holonrpc.Client,

    pub fn connect(allocator: @import("std").mem.Allocator, raw_uri: []const u8) !Client {
        return .{ .rpc = try holonrpc.Client.connect(allocator, raw_uri) };
    }

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
