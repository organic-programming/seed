const std = @import("std");
const holons = @import("zig_holons");
const fixture = @import("support/zig_greeting_server.zig");
const go_greeting = @import("support/go_greeting.zig");

const Method = holons.grpc.server.Method;
const ServerStream = holons.grpc.server.ServerStream;

var cancel_seen = std.atomic.Value(bool).init(false);

const stream_methods = [_]Method{
    .{ .path = "/test.Streaming/One", .stream_handler = oneMessage },
    .{ .path = "/test.Streaming/Many", .stream_handler = manyMessages },
    .{ .path = "/test.Streaming/Cancel", .stream_handler = waitForCancel },
    .{ .path = "/test.Streaming/Error", .stream_handler = errorAfterMessage },
};

const concurrent_methods = [_]Method{
    .{ .path = "/test.Streaming/Slow", .stream_handler = slowMessages },
    fixture.methods[0],
};

test "server streaming returns one message and OK status" {
    const allocator = std.testing.allocator;
    var server = try bindServer(allocator, &stream_methods);
    defer server.deinit();
    try server.start();

    var channel = try holons.grpc.client.connectTcp(allocator, server.listen);
    defer channel.deinit();

    var stream = try channel.serverStream(allocator, "/test.Streaming/One", "ping", 5_000);
    defer stream.deinit();

    const first = (try stream.next(allocator)).?;
    defer allocator.free(first);
    try std.testing.expectEqualStrings("one:ping", first);
    try std.testing.expectEqual(@as(?[]u8, null), try stream.next(allocator));
}

test "server streaming returns N messages and OK status" {
    const allocator = std.testing.allocator;
    var server = try bindServer(allocator, &stream_methods);
    defer server.deinit();
    try server.start();

    var channel = try holons.grpc.client.connectTcp(allocator, server.listen);
    defer channel.deinit();

    var stream = try channel.serverStream(allocator, "/test.Streaming/Many", "4", 5_000);
    defer stream.deinit();

    for (0..4) |index| {
        const message = (try stream.next(allocator)).?;
        defer allocator.free(message);
        var expected: [16]u8 = undefined;
        const want = try std.fmt.bufPrint(&expected, "msg-{}", .{index});
        try std.testing.expectEqualStrings(want, message);
    }
    try std.testing.expectEqual(@as(?[]u8, null), try stream.next(allocator));
}

test "client cancel is observed by server stream handler" {
    const allocator = std.testing.allocator;
    cancel_seen.store(false, .release);
    var server = try bindServer(allocator, &stream_methods);
    defer server.deinit();
    try server.start();

    var channel = try holons.grpc.client.connectTcp(allocator, server.listen);
    defer channel.deinit();

    var stream = try channel.serverStream(allocator, "/test.Streaming/Cancel", "", 5_000);
    stream.cancel();
    defer stream.deinit();

    var attempt: u32 = 0;
    while (attempt < 100 and !cancel_seen.load(.acquire)) : (attempt += 1) {
        sleepMillis(10);
    }
    try std.testing.expect(cancel_seen.load(.acquire));
}

test "server stream error is surfaced to the client after prior messages" {
    const allocator = std.testing.allocator;
    var server = try bindServer(allocator, &stream_methods);
    defer server.deinit();
    try server.start();

    var channel = try holons.grpc.client.connectTcp(allocator, server.listen);
    defer channel.deinit();

    var stream = try channel.serverStream(allocator, "/test.Streaming/Error", "", 5_000);
    defer stream.deinit();

    const first = (try stream.next(allocator)).?;
    defer allocator.free(first);
    try std.testing.expectEqualStrings("before-error", first);
    try std.testing.expectError(error.GrpcStatusNotOk, stream.next(allocator));
}

test "unary calls complete while a server stream remains active" {
    const allocator = std.testing.allocator;
    fixture.registerDescribe();
    defer holons.describe.clearStaticResponse();

    var server = try bindServer(allocator, &concurrent_methods);
    defer server.deinit();
    try server.start();

    var channel = try holons.grpc.client.connectTcp(allocator, server.listen);
    defer channel.deinit();

    var stream = try channel.serverStream(allocator, "/test.Streaming/Slow", "3", 5_000);
    defer stream.deinit();

    const first = (try stream.next(allocator)).?;
    defer allocator.free(first);
    try std.testing.expectEqualStrings("slow-0", first);

    var hello = try channel.sayHello(allocator, "Ada", "en");
    defer hello.deinit();
    try std.testing.expectEqualStrings("Hello Ada", hello.greeting());

    const second = (try stream.next(allocator)).?;
    defer allocator.free(second);
    try std.testing.expectEqualStrings("slow-1", second);
    const third = (try stream.next(allocator)).?;
    defer allocator.free(third);
    try std.testing.expectEqualStrings("slow-2", third);
    try std.testing.expectEqual(@as(?[]u8, null), try stream.next(allocator));
}

const RunningServer = struct {
    server: holons.grpc.server.Server,
    listen: []const u8,
    listen_owned: []u8,
    port: u16,

    fn start(self: *RunningServer) !void {
        try self.server.start();
        try go_greeting.waitTcpPort(self.port, 40);
    }

    fn deinit(self: *RunningServer) void {
        self.server.deinit();
        std.heap.c_allocator.free(self.listen_owned);
    }
};

fn bindServer(allocator: std.mem.Allocator, methods: []const Method) !RunningServer {
    _ = allocator;
    const port = try go_greeting.reserveLoopbackPort();
    const listen = try std.fmt.allocPrint(std.heap.c_allocator, "tcp://127.0.0.1:{}", .{port});
    errdefer std.heap.c_allocator.free(listen);
    const server = try holons.grpc.server.bind(std.heap.c_allocator, listen, methods);
    return .{
        .server = server,
        .listen = server.endpoint.raw,
        .listen_owned = listen,
        .port = port,
    };
}

fn oneMessage(_: std.mem.Allocator, request: []const u8, stream: *ServerStream) !void {
    var buf: [128]u8 = undefined;
    const message = try std.fmt.bufPrint(&buf, "one:{s}", .{request});
    try stream.send(message);
}

fn manyMessages(_: std.mem.Allocator, request: []const u8, stream: *ServerStream) !void {
    const total = try std.fmt.parseInt(usize, request, 10);
    for (0..total) |index| {
        var buf: [32]u8 = undefined;
        const message = try std.fmt.bufPrint(&buf, "msg-{}", .{index});
        try stream.send(message);
    }
}

fn waitForCancel(_: std.mem.Allocator, _: []const u8, stream: *ServerStream) !void {
    while (!stream.cancelled()) {
        sleepMillis(10);
    }
    cancel_seen.store(true, .release);
}

fn errorAfterMessage(_: std.mem.Allocator, _: []const u8, stream: *ServerStream) !void {
    try stream.send("before-error");
    return error.IntentionalStreamFailure;
}

fn slowMessages(_: std.mem.Allocator, request: []const u8, stream: *ServerStream) !void {
    const total = try std.fmt.parseInt(usize, request, 10);
    for (0..total) |index| {
        var buf: [32]u8 = undefined;
        const message = try std.fmt.bufPrint(&buf, "slow-{}", .{index});
        try stream.send(message);
        sleepMillis(50);
    }
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(
        std.testing.io,
        std.Io.Duration.fromMilliseconds(ms),
        .awake,
    ) catch {};
}
