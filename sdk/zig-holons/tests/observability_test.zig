const std = @import("std");
const holons = @import("zig_holons");
const go_greeting = @import("support/go_greeting.zig");

const runtime = holons.protobuf.runtime;

test "observability exposes tier two logs metrics events and disk metadata" {
    const allocator = std.testing.allocator;
    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();
    defer holons.observability.reset();

    const run_root = try std.fs.path.join(allocator, &.{ ".zig-cache", "tmp", tmp.sub_path[0..], "obs" });
    defer allocator.free(run_root);

    const env = [_]holons.observability.EnvEntry{
        .{ .key = "OP_OBS", .value = "logs,metrics,events,prom" },
        .{ .key = "OP_INSTANCE_UID", .value = "instance-1" },
    };
    var obs = try holons.observability.configureFromEnv(allocator, .{
        .slug = "zig-observable",
        .run_dir = run_root,
        .logs_ring_size = 2,
        .events_ring_size = 2,
    }, &env);
    try std.testing.expect(obs.enabled(.logs));
    try std.testing.expect(obs.enabled(.metrics));
    try std.testing.expect(obs.enabled(.events));
    try std.testing.expect(obs.enabled(.prom));

    try holons.observability.enableDiskWriters(obs, obs.cfg.run_dir);
    try holons.observability.writeMetaJson(allocator, obs.cfg.run_dir, .{
        .slug = "zig-observable",
        .uid = "instance-1",
        .pid = 123,
        .started_at_ns = 0,
        .mode = "serve",
        .transport = "stdio",
        .address = "stdio://",
        .is_default = true,
    });

    var logger = obs.logger("test");
    try logger.info("ready", &.{.{ .key = "token", .value = "secret" }});
    try obs.emit(.instance_ready, &.{.{ .key = "state", .value = "ready" }});

    const counter = (try obs.counter("requests_total", "Total requests.", &.{})).?;
    counter.add(2);
    try std.testing.expectEqual(@as(i64, 2), counter.read());

    const gauge = (try obs.gauge("active_sessions", "Active sessions.", &.{})).?;
    gauge.set(3);
    try std.testing.expectEqual(@as(f64, 3), gauge.read());

    const histogram = (try obs.histogram("rpc_seconds", "RPC latency.", &.{}, &.{ 0.01, 0.1, 1.0 })).?;
    histogram.observe(0.05);
    var snapshot = try histogram.snapshot(allocator);
    defer snapshot.deinit(allocator);
    try std.testing.expectEqual(@as(i64, 1), snapshot.total);
    try std.testing.expect(snapshot.quantile(0.5) <= 0.1);

    const logs = try obs.log_ring.?.drain(allocator);
    defer freeLogs(allocator, logs);
    try std.testing.expectEqual(@as(usize, 1), logs.len);
    try std.testing.expectEqualStrings("ready", logs[0].message);

    const events = try obs.event_bus.?.drain(allocator);
    defer freeEvents(allocator, events);
    try std.testing.expectEqual(@as(usize, 1), events.len);
    try std.testing.expectEqual(.instance_ready, events[0].event_type);

    const meta_path = try std.fs.path.join(allocator, &.{ obs.cfg.run_dir, "meta.json" });
    defer allocator.free(meta_path);
    const meta = try std.Io.Dir.cwd().readFileAlloc(std.testing.io, meta_path, allocator, .limited(1024 * 1024));
    defer allocator.free(meta);
    try std.testing.expect(std.mem.indexOf(u8, meta, "\"slug\":\"zig-observable\"") != null);
}

test "observability rejects reserved otel token at rust-parity tier" {
    try std.testing.expectError(error.ReservedObservabilityToken, holons.observability.parseOpObs("otel"));
}

test "HolonObservability Logs drains current ring" {
    const allocator = std.testing.allocator;
    var fixture = try ObservabilityFixture.init("logs");
    defer fixture.deinit();

    var logger = fixture.obs.logger("test");
    try logger.info("drain-log", &.{.{ .key = "kind", .value = "initial" }});
    try fixture.start();

    var channel = try holons.grpc.client.connectTcp(allocator, fixture.listen);
    defer channel.deinit();

    const request = try runtime.packLogsRequest(allocator, .{ .follow = false });
    defer allocator.free(request);
    var stream = try channel.serverStream(allocator, "/holons.v1.HolonObservability/Logs", request, 5_000);
    defer stream.deinit();

    const first_bytes = (try stream.next(allocator)).?;
    defer allocator.free(first_bytes);
    var first = try runtime.unpackLogEntry(first_bytes);
    defer first.deinit(allocator);
    try std.testing.expectEqualStrings("drain-log", first.message());
    try std.testing.expectEqual(@as(?[]u8, null), try stream.next(allocator));
}

test "HolonObservability Logs follow streams initial drain and live tail" {
    const allocator = std.testing.allocator;
    var fixture = try ObservabilityFixture.init("logs");
    defer fixture.deinit();

    var logger = fixture.obs.logger("test");
    try logger.info("initial-log", &.{});
    try fixture.start();

    var channel = try holons.grpc.client.connectTcp(allocator, fixture.listen);
    defer channel.deinit();

    const request = try runtime.packLogsRequest(allocator, .{ .follow = true });
    defer allocator.free(request);
    var stream = try channel.serverStream(allocator, "/holons.v1.HolonObservability/Logs", request, 5_000);
    defer stream.deinit();

    const initial_bytes = (try stream.next(allocator)).?;
    defer allocator.free(initial_bytes);
    var initial = try runtime.unpackLogEntry(initial_bytes);
    defer initial.deinit(allocator);
    try std.testing.expectEqualStrings("initial-log", initial.message());

    try logger.info("live-log", &.{});
    const live_bytes = (try stream.next(allocator)).?;
    defer allocator.free(live_bytes);
    var live = try runtime.unpackLogEntry(live_bytes);
    defer live.deinit(allocator);
    try std.testing.expectEqualStrings("live-log", live.message());
    stream.cancel();
}

test "HolonObservability Metrics returns registry snapshot" {
    const allocator = std.testing.allocator;
    var fixture = try ObservabilityFixture.init("metrics");
    defer fixture.deinit();

    const counter = (try fixture.obs.counter("zig_requests_total", "Total requests.", &.{.{ .key = "route", .value = "tick" }})).?;
    counter.add(7);
    try fixture.start();

    var channel = try holons.grpc.client.connectTcp(allocator, fixture.listen);
    defer channel.deinit();

    const request = try runtime.packMetricsRequest(allocator);
    defer allocator.free(request);
    const response = try channel.unaryAlloc(allocator, "/holons.v1.HolonObservability/Metrics", request, 5_000);
    defer allocator.free(response);
    var snapshot = try runtime.unpackMetricsSnapshot(response);
    defer snapshot.deinit();

    try std.testing.expectEqualStrings("zig-observable", snapshot.slug());
    try std.testing.expectEqual(@as(i64, 7), snapshot.counterValue("zig_requests_total").?);
}

test "HolonObservability Events follow streams initial drain and live tail" {
    const allocator = std.testing.allocator;
    var fixture = try ObservabilityFixture.init("events");
    defer fixture.deinit();

    try fixture.obs.emit(.instance_ready, &.{.{ .key = "state", .value = "initial" }});
    try fixture.start();

    var channel = try holons.grpc.client.connectTcp(allocator, fixture.listen);
    defer channel.deinit();

    const request = try runtime.packEventsRequest(allocator, .{ .follow = true });
    defer allocator.free(request);
    var stream = try channel.serverStream(allocator, "/holons.v1.HolonObservability/Events", request, 5_000);
    defer stream.deinit();

    const initial_bytes = (try stream.next(allocator)).?;
    defer allocator.free(initial_bytes);
    var initial = try runtime.unpackEventInfo(initial_bytes);
    defer initial.deinit(allocator);
    try std.testing.expectEqual(@intFromEnum(holons.observability.EventType.instance_ready), initial.eventType());

    try fixture.obs.emit(.handler_panic, &.{.{ .key = "state", .value = "live" }});
    const live_bytes = (try stream.next(allocator)).?;
    defer allocator.free(live_bytes);
    var live = try runtime.unpackEventInfo(live_bytes);
    defer live.deinit(allocator);
    try std.testing.expectEqual(@intFromEnum(holons.observability.EventType.handler_panic), live.eventType());
    stream.cancel();
}

test "Prometheus endpoint exposes registry text" {
    var fixture = try ObservabilityFixture.init("metrics,prom");
    defer fixture.deinit();

    const gauge = (try fixture.obs.gauge("zig_active_sessions", "Active sessions.", &.{})).?;
    gauge.set(3);

    const metrics_addr = (try holons.observability.startPrometheusEndpoint(fixture.obs)).?;
    defer std.heap.c_allocator.free(metrics_addr);

    const body = try httpGet(std.testing.allocator, metrics_addr);
    defer std.testing.allocator.free(body);
    try std.testing.expect(std.mem.indexOf(u8, body, "zig_active_sessions 3") != null);
}

fn freeLogs(allocator: std.mem.Allocator, logs: []holons.observability.LogEntry) void {
    for (logs) |*entry| entry.deinit(allocator);
    allocator.free(logs);
}

fn freeEvents(allocator: std.mem.Allocator, events: []holons.observability.Event) void {
    for (events) |*event| event.deinit(allocator);
    allocator.free(events);
}

const ObservabilityFixture = struct {
    obs: *holons.observability.Observability,
    server: holons.grpc.server.Server,
    listen: []const u8,
    listen_owned: []u8,
    port: u16,

    fn init(families: []const u8) !ObservabilityFixture {
        holons.observability.reset();
        const env = [_]holons.observability.EnvEntry{
            .{ .key = "OP_OBS", .value = families },
            .{ .key = "OP_INSTANCE_UID", .value = "zig-instance-1" },
        };
        const obs = try holons.observability.configureFromEnv(std.heap.c_allocator, .{
            .slug = "zig-observable",
            .logs_ring_size = 8,
            .events_ring_size = 8,
        }, &env);
        const port = try go_greeting.reserveLoopbackPort();
        const listen = try std.fmt.allocPrint(std.heap.c_allocator, "tcp://127.0.0.1:{}", .{port});
        errdefer std.heap.c_allocator.free(listen);
        const server = try holons.grpc.server.bind(std.heap.c_allocator, listen, holons.observability.serviceMethods());
        return .{
            .obs = obs,
            .server = server,
            .listen = server.endpoint.raw,
            .listen_owned = listen,
            .port = port,
        };
    }

    fn start(self: *ObservabilityFixture) !void {
        try self.server.start();
        try go_greeting.waitTcpPort(self.port, 40);
    }

    fn deinit(self: *ObservabilityFixture) void {
        self.server.deinit();
        std.heap.c_allocator.free(self.listen_owned);
        holons.observability.reset();
    }
};

fn httpGet(allocator: std.mem.Allocator, url: []const u8) ![]u8 {
    const prefix = "http://127.0.0.1:";
    if (!std.mem.startsWith(u8, url, prefix)) return error.UnsupportedTestUrl;
    const rest = url[prefix.len..];
    const slash = std.mem.indexOfScalar(u8, rest, '/') orelse return error.UnsupportedTestUrl;
    const port = try std.fmt.parseInt(u16, rest[0..slash], 10);
    const path = rest[slash..];

    const c = struct {
        const raw = @cImport({
            @cInclude("arpa/inet.h");
            @cInclude("netinet/in.h");
            @cInclude("sys/socket.h");
            @cInclude("unistd.h");
        });
    }.raw;

    const fd = c.socket(c.AF_INET, c.SOCK_STREAM, 0);
    if (fd < 0) return error.SocketFailed;
    defer _ = c.close(fd);

    var addr: c.struct_sockaddr_in = std.mem.zeroes(c.struct_sockaddr_in);
    if (@hasField(c.struct_sockaddr_in, "sin_len")) addr.sin_len = @sizeOf(c.struct_sockaddr_in);
    addr.sin_family = c.AF_INET;
    addr.sin_port = c.htons(port);
    addr.sin_addr.s_addr = c.htonl(0x7f000001);
    if (c.connect(fd, @ptrCast(&addr), @sizeOf(c.struct_sockaddr_in)) != 0) return error.ConnectFailed;

    const request = try std.fmt.allocPrint(allocator, "GET {s} HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n", .{path});
    defer allocator.free(request);
    if (c.write(fd, request.ptr, request.len) < 0) return error.WriteFailed;

    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);
    var buf: [1024]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n < 0) return error.ReadFailed;
        if (n == 0) break;
        try out.appendSlice(allocator, buf[0..@intCast(n)]);
    }
    return out.toOwnedSlice(allocator);
}
