const std = @import("std");
const holons = @import("zig_holons");

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

fn freeLogs(allocator: std.mem.Allocator, logs: []holons.observability.LogEntry) void {
    for (logs) |*entry| entry.deinit(allocator);
    allocator.free(logs);
}

fn freeEvents(allocator: std.mem.Allocator, events: []holons.observability.Event) void {
    for (events) |*event| event.deinit(allocator);
    allocator.free(events);
}
