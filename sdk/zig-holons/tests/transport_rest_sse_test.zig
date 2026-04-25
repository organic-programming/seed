const std = @import("std");
const holons = @import("zig_holons");
const server = @import("support/holonrpc_server.zig");

test "Zig invokes Holon-RPC over rest+sse HTTP unary and stream" {
    const allocator = std.testing.allocator;
    var process = try server.start(allocator, "rest");
    defer process.stop(allocator);

    var client = try holons.holonrpc.HTTPClient.init(allocator, process.ready.url);
    defer client.deinit();

    var result = try client.invokeAlloc(allocator, "echo.v1.Echo/Ping", "{\"message\":\"rest\"}");
    defer result.deinit(allocator);
    try std.testing.expectEqualStrings("{\"message\":\"rest\"}", result.result_json);

    var events = try client.streamAlloc(allocator, "echo.v1.Echo/Stream", "{\"message\":\"rest\"}");
    defer events.deinit(allocator);
    try std.testing.expectEqual(@as(usize, 3), events.items.len);
    try std.testing.expectEqualStrings("message", events.items[0].event);
    try std.testing.expectEqualStrings("{\"message\":\"rest:1\"}", events.items[0].result_json);
    try std.testing.expectEqualStrings("message", events.items[1].event);
    try std.testing.expectEqualStrings("{\"message\":\"rest:2\"}", events.items[1].result_json);
    try std.testing.expectEqualStrings("done", events.items[2].event);
}
