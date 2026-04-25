const std = @import("std");
const holons = @import("zig_holons");
const go_greeting = @import("support/go_greeting.zig");

test "Zig dials gabriel-greeting-go over tcp and calls Describe plus SayHello" {
    const allocator = std.testing.allocator;
    const port = try go_greeting.reserveLoopbackPort();
    var process = try go_greeting.startTcp(allocator, port);
    defer process.stop();
    try go_greeting.waitTcpPort(port, 80);

    const uri = try std.fmt.allocPrint(allocator, "tcp://127.0.0.1:{}", .{port});
    defer allocator.free(uri);

    var channel = try holons.grpc.client.connectTcp(allocator, uri);
    defer channel.deinit();

    var describe = try channel.describe(allocator);
    defer describe.deinit();
    try std.testing.expectEqualStrings("Greeting-Go", describe.familyName());
    try std.testing.expect(describe.serviceCount() > 0);

    var hello = try channel.sayHello(allocator, "Bob", "fr");
    defer hello.deinit();
    try std.testing.expectEqualStrings("Bonjour Bob", hello.greeting());
    try std.testing.expectEqualStrings("French", hello.language());
    try std.testing.expectEqualStrings("fr", hello.langCode());
}
