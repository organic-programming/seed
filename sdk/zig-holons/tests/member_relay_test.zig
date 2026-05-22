const std = @import("std");
const holons = @import("zig_holons");
const go_greeting = @import("support/go_greeting.zig");

test "member relay forwards logs and events with direct child chain" {
    const allocator = std.testing.allocator;
    var child = try ChildFixture.init("child-zig", "child-uid", null);
    defer holons.observability.reset();
    defer child.deinit();
    try child.start();
    try child.obs.emit(.instance_ready, &.{});

    const parent = try standaloneObs("parent-zig", "parent-uid");
    defer holons.observability.destroy(parent);

    const relays = try holons.member_relay.startAll(std.heap.c_allocator, parent, &.{
        .{ .slug = "child-zig", .address = child.listen },
    });
    defer holons.member_relay.stopAll(std.heap.c_allocator, relays);

    try child.obs.logger("relay-test").info("relayed-log", &.{holons.observability.Field.string("source", "child")});
    try child.obs.emit(.handler_panic, &.{holons.observability.Field.string("source", "child")});

    try waitForLogChain(allocator, parent, "relayed-log", "child-zig", "child-uid");
    try waitForEventChain(allocator, parent, .handler_panic, "child-zig", "child-uid");
}

test "member relay retries streams after child restart" {
    const allocator = std.testing.allocator;
    const parent = try standaloneObs("parent-zig", "parent-uid");
    defer holons.observability.destroy(parent);
    defer holons.observability.reset();

    var child = try ChildFixture.init("child-zig", "child-uid", null);
    try child.start();
    try child.obs.emit(.instance_ready, &.{});

    const relays = try holons.member_relay.startAll(std.heap.c_allocator, parent, &.{
        .{ .slug = "child-zig", .address = child.listen },
    });
    defer holons.member_relay.stopAll(std.heap.c_allocator, relays);

    try child.obs.logger("relay-test").info("before-restart", &.{});
    try waitForLogChain(allocator, parent, "before-restart", "child-zig", "child-uid");

    const port = child.port;
    child.deinit();

    var restarted = try ChildFixture.init("child-zig", "child-uid", port);
    defer restarted.deinit();
    try restarted.start();
    try restarted.obs.emit(.instance_ready, &.{});
    try restarted.obs.logger("relay-test").info("after-restart", &.{});

    try waitForLogChain(allocator, parent, "after-restart", "child-zig", "child-uid");
}

test "member relay resolves child identity from metrics when ready event is unavailable" {
    const allocator = std.testing.allocator;
    var child = try ChildFixture.initWithFamilies("child-zig", "child-uid", null, "logs,metrics");
    defer holons.observability.reset();
    defer child.deinit();
    try child.start();

    const parent = try standaloneObs("parent-zig", "parent-uid");
    defer holons.observability.destroy(parent);

    const relays = try holons.member_relay.startAll(std.heap.c_allocator, parent, &.{
        .{ .slug = "child-zig", .address = child.listen },
    });
    defer holons.member_relay.stopAll(std.heap.c_allocator, relays);

    try child.obs.logger("relay-test").info("metrics-resolved-log", &.{});
    try waitForLogChain(allocator, parent, "metrics-resolved-log", "child-zig", "child-uid");
}

const ChildFixture = struct {
    obs: *holons.observability.Observability,
    server: holons.grpc.server.Server,
    listen: []const u8,
    listen_owned: []u8,
    port: u16,

    fn init(slug: []const u8, uid: []const u8, fixed_port: ?u16) !ChildFixture {
        return initWithFamilies(slug, uid, fixed_port, "logs,events");
    }

    fn initWithFamilies(slug: []const u8, uid: []const u8, fixed_port: ?u16, families: []const u8) !ChildFixture {
        const port = fixed_port orelse try go_greeting.reserveLoopbackPort();
        const listen = try std.fmt.allocPrint(std.heap.c_allocator, "tcp://127.0.0.1:{}", .{port});
        errdefer std.heap.c_allocator.free(listen);
        const env = [_]holons.observability.EnvEntry{
            .{ .key = "OP_OBS", .value = families },
            .{ .key = "OP_INSTANCE_UID", .value = uid },
        };
        const obs = try holons.observability.configureFromEnv(std.heap.c_allocator, .{
            .slug = slug,
            .logs_ring_size = 16,
            .events_ring_size = 16,
        }, &env);
        errdefer holons.observability.reset();
        const server = try holons.grpc.server.bind(std.heap.c_allocator, listen, holons.observability.serviceMethods());
        return .{
            .obs = obs,
            .server = server,
            .listen = server.endpoint.raw,
            .listen_owned = listen,
            .port = port,
        };
    }

    fn start(self: *ChildFixture) !void {
        try self.server.start();
        try go_greeting.waitTcpPort(self.port, 40);
    }

    fn deinit(self: *ChildFixture) void {
        self.server.deinit();
        std.heap.c_allocator.free(self.listen_owned);
    }
};

fn standaloneObs(slug: []const u8, uid: []const u8) !*holons.observability.Observability {
    const env = [_]holons.observability.EnvEntry{
        .{ .key = "OP_OBS", .value = "logs,events" },
        .{ .key = "OP_INSTANCE_UID", .value = uid },
    };
    return holons.observability.createFromEnv(std.heap.c_allocator, .{
        .slug = slug,
        .logs_ring_size = 16,
        .events_ring_size = 16,
    }, &env);
}

fn waitForLogChain(
    allocator: std.mem.Allocator,
    obs: *holons.observability.Observability,
    message: []const u8,
    child_slug: []const u8,
    child_uid: []const u8,
) !void {
    var attempts: u32 = 0;
    while (attempts < 200) : (attempts += 1) {
        const logs = try obs.log_ring.?.drain(allocator);
        defer freeLogs(allocator, logs);
        for (logs) |entry| {
            if (std.mem.eql(u8, entry.message, message) and
                entry.chain.len == 1 and
                std.mem.eql(u8, entry.chain[0].slug, child_slug) and
                std.mem.eql(u8, entry.chain[0].instance_uid, child_uid))
            {
                return;
            }
        }
        sleepMillis(25);
    }
    return error.ExpectedRelayedLogNotFound;
}

fn waitForEventChain(
    allocator: std.mem.Allocator,
    obs: *holons.observability.Observability,
    event_type: holons.observability.EventType,
    child_slug: []const u8,
    child_uid: []const u8,
) !void {
    var attempts: u32 = 0;
    while (attempts < 200) : (attempts += 1) {
        const events = try obs.event_bus.?.drain(allocator);
        defer freeEvents(allocator, events);
        for (events) |event| {
            if (event.event_type == event_type and
                event.chain.len == 1 and
                std.mem.eql(u8, event.chain[0].slug, child_slug) and
                std.mem.eql(u8, event.chain[0].instance_uid, child_uid))
            {
                return;
            }
        }
        sleepMillis(25);
    }
    return error.ExpectedRelayedEventNotFound;
}

fn freeLogs(allocator: std.mem.Allocator, logs: []holons.observability.LogRecord) void {
    for (logs) |*entry| entry.deinit(allocator);
    allocator.free(logs);
}

fn freeEvents(allocator: std.mem.Allocator, events: []holons.observability.Event) void {
    for (events) |*event| event.deinit(allocator);
    allocator.free(events);
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(
        std.testing.io,
        std.Io.Duration.fromMilliseconds(ms),
        .awake,
    ) catch {};
}
