const std = @import("std");
const holons = @import("zig_holons");
const go_greeting = @import("support/go_greeting.zig");
const fixture = @import("support/zig_greeting_server.zig");

test "Zig serves GreetingService over tcp" {
    const allocator = std.testing.allocator;
    fixture.registerDescribe();
    defer holons.describe.clearStaticResponse();

    const port = try go_greeting.reserveLoopbackPort();
    var listen_buf: [64]u8 = undefined;
    const listen = try std.fmt.bufPrint(&listen_buf, "tcp://127.0.0.1:{}", .{port});

    var server = try holons.serve.bind(allocator, .{
        .listen_uri = listen,
        .methods = &fixture.methods,
    });
    defer server.deinit();
    try server.start();
    defer server.shutdown();
    try go_greeting.waitTcpPort(port, 40);

    var channel = try holons.grpc.client.connectTcp(allocator, listen);
    defer channel.deinit();

    var described = try channel.describe(allocator);
    defer described.deinit();
    try std.testing.expectEqualStrings("Greeting-Zig-Test", described.familyName());

    var hello = try channel.sayHello(allocator, "Bob", "fr");
    defer hello.deinit();
    try std.testing.expectEqualStrings("Bonjour Bob", hello.greeting());
    try std.testing.expectEqualStrings("French", hello.language());
}
