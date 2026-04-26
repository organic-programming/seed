const uri = @import("uri.zig");
const holonrpc = @import("../holonrpc.zig");

pub const Client = struct {
    http: holonrpc.HTTPClient,

    pub fn deinit(self: *Client) void {
        self.http.deinit();
        self.* = undefined;
    }

    pub fn invokeAlloc(
        self: *Client,
        allocator: @import("std").mem.Allocator,
        method: []const u8,
        params_json: []const u8,
    ) !holonrpc.InvokeResult {
        return self.http.invokeAlloc(allocator, method, params_json);
    }

    pub fn streamAlloc(
        self: *Client,
        allocator: @import("std").mem.Allocator,
        method: []const u8,
        params_json: []const u8,
    ) !holonrpc.SSEEvents {
        return self.http.streamAlloc(allocator, method, params_json);
    }
};

pub fn dial(allocator: @import("std").mem.Allocator, raw: []const u8) !Client {
    const endpoint = try uri.parse(raw);
    if (endpoint.scheme != .rest_sse) return error.UnsupportedScheme;
    return .{ .http = try holonrpc.HTTPClient.init(allocator, raw) };
}
