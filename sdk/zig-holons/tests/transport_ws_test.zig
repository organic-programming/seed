const std = @import("std");
const holons = @import("zig_holons");
const server = @import("support/holonrpc_server.zig");

test "Zig dials Holon-RPC over ws" {
    const allocator = std.testing.allocator;
    var process = try server.start(allocator, "ws");
    defer process.stop(allocator);

    var client = try holons.holonrpc.Client.connect(allocator, process.ready.url);
    defer client.deinit();

    var result = try client.invokeAlloc(allocator, "echo.v1.Echo/Ping", "{\"message\":\"hola\"}");
    defer result.deinit(allocator);
    try std.testing.expectEqualStrings("{\"message\":\"hola\"}", result.result_json);
}
