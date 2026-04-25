const std = @import("std");
const holons = @import("zig_holons");
const fixture = @import("support/zig_greeting_server.zig");

const c = @cImport({
    @cInclude("unistd.h");
});

test "Zig serves GreetingService over unix sockets" {
    if (@import("builtin").os.tag == .windows) return error.SkipZigTest;

    const allocator = std.testing.allocator;
    fixture.registerDescribe();
    defer holons.describe.clearStaticResponse();

    var path_buf: [160]u8 = undefined;
    const path = try std.fmt.bufPrint(&path_buf, "/tmp/zig-holons-serve-unix-test.sock", .{});
    const path_z = try allocator.dupeZ(u8, path);
    defer allocator.free(path_z);
    _ = c.unlink(path_z.ptr);
    defer _ = c.unlink(path_z.ptr);

    var listen_buf: [192]u8 = undefined;
    const listen = try std.fmt.bufPrint(&listen_buf, "unix://{s}", .{path});

    var server = try holons.serve.bind(allocator, .{
        .listen_uri = listen,
        .methods = &fixture.methods,
    });
    defer server.deinit();
    try server.start();
    defer server.shutdown();

    var attempt: u32 = 0;
    while (attempt < 20) : (attempt += 1) {
        std.Io.Dir.cwd().access(std.testing.io, path, .{}) catch {
            try std.Io.sleep(std.testing.io, std.Io.Duration.fromMilliseconds(50), .awake);
            continue;
        };
        break;
    }

    var channel = try holons.grpc.client.connect(allocator, listen);
    defer channel.deinit();

    var described = try channel.describe(allocator);
    defer described.deinit();
    try std.testing.expectEqualStrings("Greeting-Zig-Test", described.familyName());

    var hello = try channel.sayHello(allocator, "Ana", "en");
    defer hello.deinit();
    try std.testing.expectEqualStrings("Hello Ana", hello.greeting());
    try std.testing.expectEqualStrings("English", hello.language());
}
