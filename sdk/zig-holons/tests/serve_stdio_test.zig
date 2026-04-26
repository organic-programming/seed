const std = @import("std");
const holons = @import("zig_holons");

test "Zig serves GreetingService over stdio" {
    const allocator = std.testing.allocator;
    const fixture_path_ptr = std.c.getenv("ZIG_HOLONS_SERVE_FIXTURE") orelse
        return error.MissingServeFixturePath;
    const fixture_path = std.mem.sliceTo(fixture_path_ptr, 0);

    const argv = [_][:0]const u8{
        fixture_path,
        "serve",
        "--listen",
        "stdio://",
    };
    var channel = try holons.grpc.client.connectStdioCommand(allocator, .{ .argv = &argv });
    defer channel.deinit();

    var describe = try channel.describe(allocator);
    defer describe.deinit();
    try std.testing.expectEqualStrings("Greeting-Zig-Test", describe.familyName());

    var hello = try channel.sayHello(allocator, "Bob", "fr");
    defer hello.deinit();
    try std.testing.expectEqualStrings("Bonjour Bob", hello.greeting());
    try std.testing.expectEqualStrings("French", hello.language());
    try std.testing.expectEqualStrings("fr", hello.langCode());
}
