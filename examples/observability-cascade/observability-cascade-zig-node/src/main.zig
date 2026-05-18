const std = @import("std");
const holons = @import("zig_holons");
const describe_generated = @import("describe_generated");

const c = @cImport({
    @cInclude("unistd.h");
});

const slug = "observability-cascade-zig-node";
const version = "0.1.0";

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
        try writeStderr("usage: observability-cascade-zig-node serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]\n");
        return error.InvalidCommand;
    }

    var parsed_children = try holons.serve.ParseChildFlags(std.heap.c_allocator, args[1..]);
    defer parsed_children.deinit(std.heap.c_allocator);
    var options = try holons.serve.parseOptions(parsed_children.remaining);

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

    holons.relay.requireDownstream(parsed_children.children.len != 0);
    options.methods = holons.relay.serviceMethods(null);
    var server = try holons.serve.bind(std.heap.c_allocator, options);
    defer server.deinit();
    try server.start();
    if (obs.cfg.run_dir.len != 0) {
        try writeMeta(allocator, obs, server.endpoint.raw, metrics_addr);
    }

    var downstream: ?holons.composite.SpawnedMember = null;
    if (parsed_children.children.len != 0) {
        downstream = try holons.composite.SpawnMember(std.heap.c_allocator, .{
            .slug = parsed_children.children[0].slug,
            .binary_path = parsed_children.children[0].binary,
            .transport_name = parseTransport(parsed_children.remaining),
            .downstream_chain = parsed_children.children[1..],
        });
        if (downstream) |*member| holons.relay.setDownstream(member.conn);
    }
    defer if (downstream) |*member| member.Stop();

    try obs.emit(.instance_ready, &.{
        holons.observability.Field.string("listener", server.endpoint.raw),
        holons.observability.Field.string("metrics_addr", metrics_addr),
    });
    try holons.serve.waitStarted(&server);
}

fn parseTransport(args: []const []const u8) []const u8 {
    for (args, 0..) |arg, index| {
        if (std.mem.eql(u8, arg, "--transport") and index + 1 < args.len) return args[index + 1];
        if (std.mem.startsWith(u8, arg, "--transport=")) return arg["--transport=".len..];
    }
    return "stdio";
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
