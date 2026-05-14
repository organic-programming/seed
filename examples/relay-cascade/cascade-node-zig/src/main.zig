const std = @import("std");
const holons = @import("zig_holons");
const describe_generated = @import("describe_generated");

const c = @cImport({
    @cInclude("unistd.h");
    @cInclude("protobuf-c/protobuf-c.h");
    @cInclude("relay/v1/relay.pb-c.h");
});

const slug = "cascade-node-zig";
const version = "0.1.0";

const methods = [_]holons.grpc.server.Method{
    .{ .path = "/relay.v1.RelayService/Tick", .handler = tick },
};

pub fn main(init: std.process.Init.Minimal) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.c_allocator);
    defer arena.deinit();
    const allocator = arena.allocator();
    const args = try init.args.toSlice(allocator);

    holons.describe.useStaticResponse(describe_generated.staticDescribeResponse());

    if (args.len > 1 and std.mem.eql(u8, args[1], "version")) {
        try writeStdout(try std.fmt.allocPrint(allocator, "{s} {s}\n", .{ slug, version }));
        return;
    }
    if (args.len < 2 or !std.mem.eql(u8, args[1], "serve")) {
        try writeStderr("usage: cascade-node-zig serve [--listen <uri>] [--member <slug>=<address>]\n");
        return error.InvalidCommand;
    }

    var parsed = try holons.serve.parseOptionsAlloc(std.heap.c_allocator, args[1..]);
    defer parsed.deinit(std.heap.c_allocator);
    parsed.options.methods = methods[0..];

    const obs = try holons.observability.configure(std.heap.c_allocator, .{
        .slug = slug,
        .default_log_level = .info,
        .logs_ring_size = 1024,
        .events_ring_size = 1024,
    });
    defer holons.observability.reset();
    if (obs.cfg.run_dir.len != 0) try holons.observability.enableDiskWriters(obs, obs.cfg.run_dir);
    const metrics_addr = (try holons.observability.startPrometheusEndpoint(obs)) orelse "";
    defer if (metrics_addr.len != 0) std.heap.c_allocator.free(metrics_addr);
    if (obs.cfg.run_dir.len != 0) {
        try writeMeta(allocator, obs, parsed.options.listen_uri, metrics_addr);
    }
    try obs.emit(.instance_ready, &.{});

    try holons.serve.runSingle(parsed.options);
}

fn tick(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    const request = c.relay__v1__tick_request__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeTickRequestFailed;
    defer c.relay__v1__tick_request__free_unpacked(request, null);

    const obs = holons.observability.current() orelse return error.ObservabilityNotConfigured;
    const uid = obs.cfg.instance_uid;
    var logger = obs.logger("tick");
    try logger.info("tick received", &.{
        .{ .key = "sender", .value = cstr(request.*.sender) },
        .{ .key = "note", .value = cstr(request.*.note) },
        .{ .key = "responder_slug", .value = slug },
        .{ .key = "responder_uid", .value = uid },
    });
    if (try obs.counter("cascade_ticks_total", "Ticks received by this cascade node.", &.{
        .{ .key = "responder_uid", .value = uid },
    })) |counter| {
        counter.inc();
    }

    var response: c.Relay__V1__TickResponse = undefined;
    c.relay__v1__tick_response__init(&response);
    const response_slug = try allocator.dupeZ(u8, slug);
    const response_uid = try allocator.dupeZ(u8, uid);
    response.responder_slug = response_slug.ptr;
    response.responder_instance_uid = response_uid.ptr;
    const len = c.relay__v1__tick_response__get_packed_size(&response);
    const out = try allocator.alloc(u8, len);
    const encoded = c.relay__v1__tick_response__pack(&response, out.ptr);
    if (encoded != len) return error.EncodeTickResponseFailed;
    return out;
}

fn writeMeta(
    allocator: std.mem.Allocator,
    obs: *holons.observability.Observability,
    listen_uri: []const u8,
    metrics_addr: []const u8,
) !void {
    const log_path = try std.fs.path.join(allocator, &.{ obs.cfg.run_dir, "stdout.log" });
    defer allocator.free(log_path);
    try holons.observability.writeMetaJson(allocator, obs.cfg.run_dir, .{
        .slug = slug,
        .uid = obs.cfg.instance_uid,
        .pid = c.getpid(),
        .started_at_ns = nowNs(),
        .mode = "serve",
        .transport = transportName(listen_uri),
        .address = listen_uri,
        .metrics_addr = metrics_addr,
        .log_path = log_path,
        .organism_uid = obs.cfg.organism_uid,
        .organism_slug = obs.cfg.organism_slug,
        .is_default = true,
    });
}

fn transportName(uri: []const u8) []const u8 {
    if (std.mem.startsWith(u8, uri, "tcp://")) return "tcp";
    if (std.mem.startsWith(u8, uri, "unix://")) return "unix";
    if (std.mem.startsWith(u8, uri, "stdio://")) return "stdio";
    return "";
}

fn cstr(value: [*c]const u8) []const u8 {
    if (value == null) return "";
    return std.mem.span(value);
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}

fn writeStdout(bytes: []const u8) !void {
    try writeAll(c.STDOUT_FILENO, bytes);
}

fn writeStderr(bytes: []const u8) !void {
    try writeAll(c.STDERR_FILENO, bytes);
}

fn writeAll(fd: c_int, bytes: []const u8) !void {
    var offset: usize = 0;
    while (offset < bytes.len) {
        const chunk = @min(bytes.len - offset, std.math.maxInt(c_uint));
        const written = c.write(fd, bytes[offset..].ptr, @intCast(chunk));
        if (written < 0) return error.WriteFailed;
        if (written == 0) return error.WriteFailed;
        offset += @intCast(written);
    }
}
