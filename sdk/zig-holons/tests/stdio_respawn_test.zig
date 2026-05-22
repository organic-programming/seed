const std = @import("std");
const holons = @import("zig_holons");

const c = @cImport({
    @cInclude("stdlib.h");
});

const zig_slug = "observability-cascade-zig-node";
const go_slug = "observability-cascade-go-node";

test "stdio event stream replays spawned go child ready" {
    const allocator = std.heap.c_allocator;
    const go_bin = try binaryPath(allocator, "ZIG_HOLONS_CASCADE_GO_NODE", go_slug);
    defer allocator.free(go_bin);

    const run_root = try std.fmt.allocPrint(allocator, "{s}/zig-holons-stdio-direct-{d}", .{ tmpDir(), nowNs() });
    defer allocator.free(run_root);
    setEnv("OP_OBS", "logs,events,metrics,prom") catch return error.SetEnvFailed;
    setEnv("OP_RUN_DIR", run_root) catch return error.SetEnvFailed;
    setEnv("OP_PROM_ADDR", "127.0.0.1:0") catch return error.SetEnvFailed;

    const obs = try holons.observability.configure(allocator, .{
        .slug = "zig-holons-stdio-direct-root",
        .instance_uid = "zig-holons-stdio-direct-root",
        .logs_ring_size = 8192,
        .events_ring_size = 8192,
        .run_dir = run_root,
    });
    _ = obs;
    defer holons.observability.reset();

    var child = try holons.composite.SpawnMember(allocator, .{
        .slug = go_slug,
        .binary_path = go_bin,
        .transport_name = "stdio",
        .extra_env = &.{
            .{ .key = "OP_OBS", .value = "logs,events,metrics,prom" },
            .{ .key = "OP_PROM_ADDR", .value = "127.0.0.1:0" },
        },
        .dial_options = holons.composite.WithTransitiveObservability(false),
    });
    defer child.Stop();

    const request = try holons.protobuf.runtime.packEventsRequest(allocator, .{
        .follow = true,
    });
    defer allocator.free(request);
    var stream = try child.conn.serverStream(allocator, "/holons.v1.HolonObservability/Events", request, 5_000);
    defer stream.deinit();
    const bytes = (try stream.next(allocator)) orelse return error.ReadyEventMissing;
    defer allocator.free(bytes);
    var event = try holons.observability.eventFromBytes(allocator, bytes);
    defer event.deinit(allocator);
    try std.testing.expectEqual(holons.observability.EventType.instance_ready, event.event_type);
    try std.testing.expectEqualStrings(go_slug, event.slug);
    try std.testing.expectEqualStrings(child.uid, event.instance_uid);
}

test "stdio respawn relays replayed observability through zig zig go cascade" {
    const allocator = std.heap.c_allocator;
    const zig_bin = try binaryPath(allocator, "ZIG_HOLONS_CASCADE_ZIG_NODE", zig_slug);
    defer allocator.free(zig_bin);
    const go_bin = try binaryPath(allocator, "ZIG_HOLONS_CASCADE_GO_NODE", go_slug);
    defer allocator.free(go_bin);

    const run_root = try std.fmt.allocPrint(allocator, "{s}/zig-holons-stdio-respawn-{d}", .{ tmpDir(), nowNs() });
    defer allocator.free(run_root);
    setEnv("OP_OBS", "logs,events,metrics,prom") catch return error.SetEnvFailed;
    setEnv("OP_RUN_DIR", run_root) catch return error.SetEnvFailed;
    setEnv("OP_PROM_ADDR", "127.0.0.1:0") catch return error.SetEnvFailed;

    const obs = try holons.observability.configure(allocator, .{
        .slug = "zig-holons-stdio-respawn-root",
        .instance_uid = "zig-holons-stdio-respawn-root",
        .logs_ring_size = 8192,
        .events_ring_size = 8192,
        .run_dir = run_root,
    });
    _ = obs;
    defer holons.observability.reset();

    try runRespawnPhase(allocator, zig_bin, go_bin, "first");
    try runRespawnPhase(allocator, zig_bin, go_bin, "second");
}

fn runRespawnPhase(
    allocator: std.mem.Allocator,
    zig_bin: []const u8,
    go_bin: []const u8,
    phase: []const u8,
) !void {
    const members = [_]holons.composite.ChildSpec{
        .{ .slug = zig_slug, .binary = zig_bin },
        .{ .slug = zig_slug, .binary = zig_bin },
        .{ .slug = go_slug, .binary = go_bin },
    };
    var cascade = try holons.composite.BuildCascade(allocator, .{
        .transport_name = "stdio",
        .members = members[0..],
        .extra_env = &.{
            .{ .key = "OP_OBS", .value = "logs,events,metrics,prom" },
            .{ .key = "OP_PROM_ADDR", .value = "127.0.0.1:0" },
        },
    });
    defer cascade.Stop();

    const sender = try std.fmt.allocPrint(allocator, "stdio-respawn-{s}", .{phase});
    defer allocator.free(sender);
    const request = try holons.relay.packTickRequest(allocator, sender, "stdio-respawn");
    defer allocator.free(request);
    const response_bytes = try cascade.top.conn.unaryAlloc(
        allocator,
        "/relay.v1.RelayService/Tick",
        request,
        5_000,
    );
    defer allocator.free(response_bytes);
    var response = try holons.relay.unpackTickResponse(allocator, response_bytes);
    defer response.deinit(allocator);

    try expectHopChain(response.hops);
    const expected = try expectedChain(allocator, response.hops);
    defer allocator.free(expected);

    const leaf_uid = response.hops[0].uid;
    const log = holons.composite.CheckRelayedLog(allocator, .{
        .sender = sender,
        .leaf_uid = leaf_uid,
        .expected_chain = expected,
        .timeout_ms = 5_000,
        .poll_ms = 25,
        .live = true,
    });
    if (!log.pass) return error.RelayedLogMissing;

    const event = holons.composite.CheckRelayedEvent(allocator, .{
        .event_type = .instance_ready,
        .leaf_uid = leaf_uid,
        .expected_chain = expected,
        .timeout_ms = 5_000,
        .poll_ms = 25,
        .live = true,
    });
    if (!event.pass) return error.RelayedInstanceReadyMissing;
}

fn expectHopChain(hops: []const holons.relay.HopReceipt) !void {
    try std.testing.expectEqual(@as(usize, 3), hops.len);
    try std.testing.expectEqualStrings(go_slug, hops[0].slug);
    try std.testing.expectEqualStrings(zig_slug, hops[1].slug);
    try std.testing.expectEqualStrings(zig_slug, hops[2].slug);
    for (hops) |hop| {
        try std.testing.expect(hop.uid.len != 0);
        try std.testing.expect(hop.received > 0);
    }
}

fn expectedChain(
    allocator: std.mem.Allocator,
    hops: []const holons.relay.HopReceipt,
) ![]holons.composite.ChainHop {
    var out = try allocator.alloc(holons.composite.ChainHop, hops.len);
    for (hops, 0..) |hop, index| {
        out[index] = .{ .slug = hop.slug, .instance_uid = hop.uid };
    }
    return out;
}

fn binaryPath(allocator: std.mem.Allocator, env_name: [*:0]const u8, exe_name: []const u8) ![]u8 {
    if (c.getenv(env_name)) |value| {
        const path = std.mem.span(value);
        if (path.len != 0) return allocator.dupe(u8, path);
    }
    var roots: [4][]const u8 = undefined;
    var len: usize = 0;
    if (std.mem.eql(u8, exe_name, zig_slug)) {
        roots[len] = "../../examples/observability-cascade/observability-cascade-zig-node/zig-out/bin";
        len += 1;
        roots[len] = "../../examples/observability-cascade/observability-cascade-zig-node/.op/build";
        len += 1;
        roots[len] = "../../examples/observability-cascade/observability-cascade-zig/.op/build";
        len += 1;
    } else if (std.mem.eql(u8, exe_name, go_slug)) {
        roots[len] = "../../examples/observability-cascade/observability-cascade-go-node/.op/build";
        len += 1;
        roots[len] = "../../examples/observability-cascade/observability-cascade-zig/.op/build";
        len += 1;
    }
    roots[len] = "../../.op/build";
    len += 1;
    for (roots[0..len]) |root| {
        if (findBinary(allocator, root, exe_name) catch null) |found| return found;
    }
    return error.MissingCascadeBinary;
}

fn findBinary(allocator: std.mem.Allocator, root: []const u8, exe_name: []const u8) !?[]u8 {
    const io = std.Io.Threaded.global_single_threaded.io();
    var dir = std.Io.Dir.cwd().openDir(io, root, .{ .iterate = true }) catch return null;
    defer dir.close(io);
    var iter = dir.iterate();
    while (try iter.next(io)) |entry| {
        if (entry.name.len == 0) continue;
        const child = try std.fs.path.join(allocator, &.{ root, entry.name });
        defer allocator.free(child);
        switch (entry.kind) {
            .file => {
                if (std.mem.eql(u8, entry.name, exe_name)) return @as(?[]u8, try absolutePath(allocator, child));
            },
            .directory => {
                if (try findBinary(allocator, child, exe_name)) |found| return found;
            },
            else => {},
        }
    }
    return null;
}

fn absolutePath(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    if (std.fs.path.isAbsolute(path)) return allocator.dupe(u8, path);
    var cwd_buffer: [std.fs.max_path_bytes]u8 = undefined;
    const len = try std.process.currentPath(std.Io.Threaded.global_single_threaded.io(), &cwd_buffer);
    return std.fs.path.join(allocator, &.{ cwd_buffer[0..len], path });
}

fn setEnv(key: []const u8, value: []const u8) !void {
    const key_z = try std.heap.c_allocator.dupeZ(u8, key);
    defer std.heap.c_allocator.free(key_z);
    const value_z = try std.heap.c_allocator.dupeZ(u8, value);
    defer std.heap.c_allocator.free(value_z);
    if (c.setenv(key_z.ptr, value_z.ptr, 1) != 0) return error.SetEnvFailed;
}

fn tmpDir() []const u8 {
    if (c.getenv("TMPDIR")) |value| {
        const path = std.mem.span(value);
        if (path.len != 0) return path;
    }
    return "/tmp";
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}
