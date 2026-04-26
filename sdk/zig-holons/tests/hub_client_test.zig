const std = @import("std");
const holons = @import("zig_holons");
const server = @import("support/holonrpc_server.zig");

test "Zig hub client invokes a Holon-RPC hub method" {
    const allocator = std.testing.allocator;
    var process = try server.start(allocator, "hub");
    defer process.stop(allocator);

    var client = try holons.hub.Client.connect(allocator, process.ready.url);
    defer client.deinit();

    var result = try client.invokeAlloc(allocator, "hub.v1.Hub/ListPeers", "{}");
    defer result.deinit(allocator);
    try std.testing.expect(std.mem.indexOf(u8, result.result_json, "go-helper") != null);
}
