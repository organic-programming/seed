const std = @import("std");
const composite = @import("composite.zig");
const describe = @import("describe.zig");
const grpc_server = @import("grpc/server.zig");
const member_relay = @import("member_relay.zig");
const observability = @import("observability.zig");
const transport = @import("transport.zig");

const c = @cImport({
    @cInclude("signal.h");
});

var shutdown_signal = std.atomic.Value(bool).init(false);
var current_transport_mutex: std.Io.Mutex = .init;
var current_transport_buf: [16]u8 = [_]u8{0} ** 16;
var current_transport_len: usize = 0;

pub fn CurrentTransport() []const u8 {
    current_transport_mutex.lockUncancelable(std.Options.debug_io);
    defer current_transport_mutex.unlock(std.Options.debug_io);
    return current_transport_buf[0..current_transport_len];
}

fn setCurrentTransport(value: []const u8) void {
    current_transport_mutex.lockUncancelable(std.Options.debug_io);
    defer current_transport_mutex.unlock(std.Options.debug_io);
    const len = @min(value.len, current_transport_buf.len);
    @memcpy(current_transport_buf[0..len], value[0..len]);
    current_transport_len = len;
}

pub const Options = struct {
    listen_uri: []const u8 = "stdio://",
    methods: []const grpc_server.Method = &.{},
    member_endpoints: []const member_relay.MemberRef = &.{},
};

pub const ParsedOptions = struct {
    options: Options,
    member_endpoints: []member_relay.MemberRef = &.{},

    pub fn deinit(self: *ParsedOptions, allocator: std.mem.Allocator) void {
        for (self.member_endpoints) |*member| member.deinit(allocator);
        allocator.free(self.member_endpoints);
        self.* = .{ .options = .{} };
    }
};

pub const Error = error{
    MissingListenValue,
    MissingMemberValue,
    UnsupportedListenTransport,
} || transport.uri.ParseError || describe.Error || member_relay.ParseError || std.mem.Allocator.Error;

pub const ChildSpec = composite.ChildSpec;

pub const ChildFlagResult = struct {
    children: []ChildSpec,
    remaining: []const []const u8,

    pub fn deinit(self: *ChildFlagResult, allocator: std.mem.Allocator) void {
        for (self.children) |child| {
            allocator.free(child.slug);
            allocator.free(child.binary);
        }
        allocator.free(self.children);
        allocator.free(self.remaining);
        self.* = .{ .children = &.{}, .remaining = &.{} };
    }
};

pub fn ParseChildFlags(allocator: std.mem.Allocator, args: []const []const u8) !ChildFlagResult {
    var children: std.ArrayList(ChildSpec) = .empty;
    var remaining: std.ArrayList([]const u8) = .empty;
    errdefer {
        for (children.items) |child| {
            allocator.free(child.slug);
            allocator.free(child.binary);
        }
        children.deinit(allocator);
        remaining.deinit(allocator);
    }

    var i: usize = 0;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--child")) {
            if (i + 1 < args.len) {
                if (try parseChildSpec(allocator, args[i + 1])) |child| {
                    try children.append(allocator, child);
                }
                i += 1;
                continue;
            }
        }
        if (std.mem.startsWith(u8, arg, "--child=")) {
            if (try parseChildSpec(allocator, arg["--child=".len..])) |child| {
                try children.append(allocator, child);
            }
            continue;
        }
        try remaining.append(allocator, arg);
    }
    return .{
        .children = try children.toOwnedSlice(allocator),
        .remaining = try remaining.toOwnedSlice(allocator),
    };
}

fn parseChildSpec(allocator: std.mem.Allocator, raw: []const u8) !?ChildSpec {
    const eq = std.mem.indexOfScalar(u8, raw, '=') orelse return null;
    const slug = std.mem.trim(u8, raw[0..eq], " \t\r\n");
    const binary = std.mem.trim(u8, raw[eq + 1 ..], " \t\r\n");
    if (slug.len == 0 or binary.len == 0) return null;
    return .{
        .slug = try allocator.dupe(u8, slug),
        .binary = try allocator.dupe(u8, binary),
    };
}

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

pub fn parseOptionsAlloc(allocator: std.mem.Allocator, args: []const []const u8) Error!ParsedOptions {
    var options = try parseOptions(args);
    var members: std.ArrayList(member_relay.MemberRef) = .empty;
    errdefer {
        for (members.items) |*member| member.deinit(allocator);
        members.deinit(allocator);
    }

    var i: usize = 0;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--member")) {
            i += 1;
            if (i >= args.len) return error.MissingMemberValue;
            try appendParsedMember(allocator, &members, args[i]);
            continue;
        }
        if (std.mem.startsWith(u8, arg, "--member=")) {
            try appendParsedMember(allocator, &members, arg["--member=".len..]);
        }
    }
    const owned = try members.toOwnedSlice(allocator);
    options.member_endpoints = owned;
    return .{ .options = options, .member_endpoints = owned };
}

pub fn runSingle(options: Options) !void {
    _ = try describe.current();
    const owned_methods = try methodsWithObservability(std.heap.c_allocator, options.methods);
    defer if (owned_methods) |methods| std.heap.c_allocator.free(methods);
    var server = try grpc_server.bind(std.heap.c_allocator, options.listen_uri, owned_methods orelse options.methods);
    if (!transport.supportsServe(server.endpoint.scheme)) return error.UnsupportedListenTransport;
    defer server.deinit();
    setCurrentTransport(server.endpoint.scheme.text());
    defer setCurrentTransport("");
    const relay_obs = observability.current();
    const relays = if (relay_obs) |obs|
        try member_relay.startAll(std.heap.c_allocator, obs, options.member_endpoints)
    else
        try std.heap.c_allocator.alloc(member_relay.MemberRelay, 0);
    defer member_relay.stopAll(std.heap.c_allocator, relays);
    try server.start();
    if (server.endpoint.scheme != .stdio) {
        std.debug.print("{s}\n", .{server.endpoint.raw});
    }
    const watcher = try startSignalWatcher(&server);
    defer stopSignalWatcher(watcher);
    server.wait();
}

pub fn runBound(server: *grpc_server.Server) !void {
    try server.start();
    try waitStarted(server);
}

pub fn waitStarted(server: *grpc_server.Server) !void {
    setCurrentTransport(server.endpoint.scheme.text());
    defer setCurrentTransport("");
    if (server.endpoint.scheme != .stdio) {
        std.debug.print("{s}\n", .{server.endpoint.raw});
    }
    const watcher = try startSignalWatcher(server);
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

fn appendParsedMember(
    allocator: std.mem.Allocator,
    members: *std.ArrayList(member_relay.MemberRef),
    raw: []const u8,
) !void {
    const parsed = try member_relay.parseMember(raw);
    try members.append(allocator, .{
        .slug = try allocator.dupe(u8, parsed.slug),
        .uid = try allocator.dupe(u8, parsed.uid),
        .address = try allocator.dupe(u8, parsed.address),
    });
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

test "parse repeated member options" {
    var parsed = try parseOptionsAlloc(std.testing.allocator, &.{
        "serve",
        "--member",
        "child-a@uid-a=tcp://127.0.0.1:9001",
        "--member=child-b=tcp://127.0.0.1:9002",
    });
    defer parsed.deinit(std.testing.allocator);
    try std.testing.expectEqual(@as(usize, 2), parsed.options.member_endpoints.len);
    try std.testing.expectEqualStrings("child-a", parsed.options.member_endpoints[0].slug);
    try std.testing.expectEqualStrings("uid-a", parsed.options.member_endpoints[0].uid);
    try std.testing.expectEqualStrings("tcp://127.0.0.1:9002", parsed.options.member_endpoints[1].address);
}

test "current transport tracks serve lifecycle state" {
    try std.testing.expectEqualStrings("", CurrentTransport());
    setCurrentTransport("stdio");
    try std.testing.expectEqualStrings("stdio", CurrentTransport());
    setCurrentTransport("");
    try std.testing.expectEqualStrings("", CurrentTransport());
}
