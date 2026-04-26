const std = @import("std");
const holons = @import("zig_holons");
const go_greeting = @import("support/go_greeting.zig");

test "Zig dials gabriel-greeting-go over stdio and calls Describe plus SayHello" {
    const allocator = std.testing.allocator;
    const cwd = try go_greeting.goGreetingDir(allocator);
    defer allocator.free(cwd);
    const argv = [_][:0]const u8{
        "go",
        "run",
        "./cmd",
        "serve",
        "--listen",
        "stdio://",
    };

    var channel = try holons.grpc.client.connectStdioCommand(allocator, .{
        .argv = &argv,
        .cwd = cwd,
    });
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
