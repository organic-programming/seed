const std = @import("std");
const holons = @import("zig_holons");
const server = @import("support/holonrpc_server.zig");

test "Zig dials Holon-RPC over wss with a trusted test CA" {
    const allocator = std.testing.allocator;
    var process = try server.start(allocator, "wss");
    defer process.stop(allocator);

    var client = try holons.holonrpc.Client.connectWithOptions(allocator, process.ready.url, .{
        .tls_ca_file = process.ready.ca_file.?,
    });
    defer client.deinit();

    var result = try client.invokeAlloc(allocator, "echo.v1.Echo/Ping", "{\"message\":\"secure\"}");
    defer result.deinit(allocator);
    try std.testing.expectEqualStrings("{\"message\":\"secure\"}", result.result_json);
}
