const std = @import("std");
const describe = @import("describe.zig");
const grpc_server = @import("grpc/server.zig");
const observability = @import("observability.zig");
const transport = @import("transport.zig");

const c = @cImport({
    @cInclude("signal.h");
});

var shutdown_signal = std.atomic.Value(bool).init(false);

pub const Options = struct {
    listen_uri: []const u8 = "stdio://",
    methods: []const grpc_server.Method = &.{},
};

pub const Error = error{
    MissingListenValue,
    UnsupportedListenTransport,
} || transport.uri.ParseError || describe.Error;

pub fn parseOptions(args: []const []const u8) Error!Options {
    var options = Options{};
    var i: usize = 0;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "serve")) continue;
        if (std.mem.eql(u8, arg, "--listen")) {
            i += 1;
            if (i >= args.len) return error.MissingListenValue;
            options.listen_uri = args[i];
            continue;
        }
        if (std.mem.startsWith(u8, arg, "--listen=")) {
            options.listen_uri = arg["--listen=".len..];
        }
    }
    return options;
}

pub fn runSingle(options: Options) !void {
    _ = try describe.current();
    const owned_methods = try methodsWithObservability(std.heap.c_allocator, options.methods);
    defer if (owned_methods) |methods| std.heap.c_allocator.free(methods);
    var server = try grpc_server.bind(std.heap.c_allocator, options.listen_uri, owned_methods orelse options.methods);
    if (!transport.supportsServe(server.endpoint.scheme)) return error.UnsupportedListenTransport;
    defer server.deinit();
    try server.start();
    if (server.endpoint.scheme != .stdio) {
        std.debug.print("{s}\n", .{server.endpoint.raw});
    }
    const watcher = try startSignalWatcher(&server);
    defer stopSignalWatcher(watcher);
    server.wait();
}

pub fn bind(allocator: std.mem.Allocator, options: Options) !grpc_server.Server {
    _ = try describe.current();
    const owned_methods = try methodsWithObservability(allocator, options.methods);
    defer if (owned_methods) |methods| allocator.free(methods);
    const server = try grpc_server.bind(allocator, options.listen_uri, owned_methods orelse options.methods);
    if (!transport.supportsServe(server.endpoint.scheme)) return error.UnsupportedListenTransport;
    return server;
}

fn methodsWithObservability(allocator: std.mem.Allocator, methods: []const grpc_server.Method) !?[]grpc_server.Method {
    const obs = observability.current() orelse return null;
    if (!obs.enabled(.logs) and !obs.enabled(.metrics) and !obs.enabled(.events)) return null;
    const obs_methods = observability.serviceMethods();
    var out = try allocator.alloc(grpc_server.Method, methods.len + obs_methods.len);
    @memcpy(out[0..obs_methods.len], obs_methods);
    @memcpy(out[obs_methods.len..], methods);
    return out;
}

fn startSignalWatcher(server: *grpc_server.Server) !std.Thread {
    shutdown_signal.store(false, .release);
    _ = c.signal(c.SIGTERM, handleSignal);
    _ = c.signal(c.SIGINT, handleSignal);
    return std.Thread.spawn(.{}, signalWatcher, .{server});
}

fn stopSignalWatcher(watcher: std.Thread) void {
    shutdown_signal.store(true, .release);
    watcher.join();
}

fn handleSignal(_: c_int) callconv(.c) void {
    shutdown_signal.store(true, .release);
}

fn signalWatcher(server: *grpc_server.Server) void {
    while (!shutdown_signal.load(.acquire)) {
        sleepMillis(50);
    }
    server.shutdown();
}

fn sleepMillis(ms: c_long) void {
    std.Io.sleep(
        std.Io.Threaded.global_single_threaded.io(),
        std.Io.Duration.fromMilliseconds(@intCast(ms)),
        .awake,
    ) catch {};
}

test "parse listen option" {
    const options = try parseOptions(&.{ "serve", "--listen", "tcp://127.0.0.1:9090" });
    try std.testing.expectEqualStrings("tcp://127.0.0.1:9090", options.listen_uri);
}
