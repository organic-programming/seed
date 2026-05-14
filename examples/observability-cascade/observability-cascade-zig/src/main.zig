const std = @import("std");
const holons = @import("zig_holons");
const describe_generated = @import("describe_generated");

const c = @cImport({
    @cInclude("fcntl.h");
    @cInclude("signal.h");
    @cInclude("stdlib.h");
    @cInclude("unistd.h");
    @cInclude("arpa/inet.h");
    @cInclude("netinet/in.h");
    @cInclude("protobuf-c/protobuf-c.h");
    @cInclude("sys/socket.h");
    @cInclude("sys/wait.h");
    @cInclude("observability_cascade/v1/service.pb-c.h");
    @cInclude("relay/v1/relay.pb-c.h");
});

const zig_slug = "observability-cascade-node-zig";
const go_slug = "observability-cascade-node-go";
const run_ticks = 3;
const roles = [_][]const u8{ "D", "C", "B", "A" };
const transports = [_][]const u8{ "tcp", "unix", "tcp", "unix" };

const rpc_methods = [_]holons.grpc.server.Method{
    .{ .path = "/observability_cascade.v1.ObservabilityCascadeService/RunDefault", .handler = runDefaultRpc },
    .{ .path = "/observability_cascade.v1.ObservabilityCascadeService/RunLiveStream", .handler = runLiveStreamRpc },
    .{ .path = "/observability_cascade.v1.ObservabilityCascadeService/RunMultiPattern", .handler = runMultiPatternRpc },
};

const CascadeReportData = struct {
    ticks: usize,
    pass: usize,
    fail: usize,
};

const MultiPatternReportData = struct {
    total_pass: usize,
    total_fail: usize,
};

pub fn main(init: std.process.Init.Minimal) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.c_allocator);
    defer arena.deinit();
    const allocator = arena.allocator();
    const args = try init.args.toSlice(allocator);
    const mode = if (args.len > 1) args[1] else "";
    holons.describe.useStaticResponse(describe_generated.staticDescribeResponse());
    if (std.mem.eql(u8, mode, "serve")) {
        var options = try holons.serve.parseOptions(args[1..]);
        options.methods = rpc_methods[0..];
        try holons.serve.runSingle(options);
        return;
    }
    if (std.mem.eql(u8, mode, "--multi-pattern")) {
        _ = try runMultiPattern(allocator, true);
    } else if (std.mem.eql(u8, mode, "--live-stream")) {
        _ = try runMode(allocator, "=== observability-cascade-zig --live-stream ===", "Summary: {} PASS / {} FAIL across {} ticks", true, true);
    } else {
        _ = try runMode(allocator, "=== observability-cascade-zig ===", "Summary: {} ticks, {} PASS, {} FAIL", false, true);
    }
}

const RoleSpec = struct {
    slug: []const u8,
    binary: []const u8,
};

const Role = struct {
    name: []const u8,
    slug: []const u8,
    uid: []u8,
    listen_uri: []u8,
    member_address: []const u8 = "",
    metrics_addr: []u8 = "",
    process: c.pid_t = 0,
    conn: ?holons.grpc.client.Channel = null,
};

const Cascade = struct {
    allocator: std.mem.Allocator,
    phase: usize,
    transport: []const u8,
    run_root: []const u8,
    role_list: [4]Role,

    fn stop(self: *Cascade) void {
        var i: usize = 0;
        while (i < self.role_list.len) : (i += 1) {
            const index = self.role_list.len - 1 - i;
            if (self.role_list[index].conn) |*conn| conn.deinit();
            if (self.role_list[index].process > 0) {
                _ = c.kill(self.role_list[index].process, c.SIGTERM);
                var status: c_int = 0;
                _ = c.waitpid(self.role_list[index].process, &status, 0);
                self.role_list[index].process = 0;
            }
        }
    }

    fn deinit(self: *Cascade) void {
        for (&self.role_list) |*role| {
            self.allocator.free(role.uid);
            self.allocator.free(role.listen_uri);
            if (role.metrics_addr.len != 0) self.allocator.free(role.metrics_addr);
        }
    }
};

fn runDefaultRpc(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    _ = bytes;
    const report = try runMode(allocator, "=== observability-cascade-zig ===", "Summary: {} ticks, {} PASS, {} FAIL", false, false);
    return packCascadeReport(allocator, report);
}

fn runLiveStreamRpc(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    _ = bytes;
    const report = try runMode(allocator, "=== observability-cascade-zig --live-stream ===", "Summary: {} PASS / {} FAIL across {} ticks", true, false);
    return packCascadeReport(allocator, report);
}

fn runMultiPatternRpc(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    _ = bytes;
    const report = try runMultiPattern(allocator, false);
    return packMultiPatternReport(allocator, report);
}

fn packCascadeReport(allocator: std.mem.Allocator, report: CascadeReportData) ![]u8 {
    var response: c.ObservabilityCascade__V1__CascadeReport = undefined;
    c.observability_cascade__v1__cascade_report__init(&response);
    response.ticks = @intCast(report.ticks);
    response.pass = @intCast(report.pass);
    response.fail = @intCast(report.fail);
    const len = c.observability_cascade__v1__cascade_report__get_packed_size(&response);
    const out = try allocator.alloc(u8, len);
    const encoded = c.observability_cascade__v1__cascade_report__pack(&response, out.ptr);
    if (encoded != len) return error.EncodeCascadeReportFailed;
    return out;
}

fn packMultiPatternReport(allocator: std.mem.Allocator, report: MultiPatternReportData) ![]u8 {
    var response: c.ObservabilityCascade__V1__MultiPatternReport = undefined;
    c.observability_cascade__v1__multi_pattern_report__init(&response);
    response.total_pass = @intCast(report.total_pass);
    response.total_fail = @intCast(report.total_fail);
    const len = c.observability_cascade__v1__multi_pattern_report__get_packed_size(&response);
    const out = try allocator.alloc(u8, len);
    const encoded = c.observability_cascade__v1__multi_pattern_report__pack(&response, out.ptr);
    if (encoded != len) return error.EncodeMultiPatternReportFailed;
    return out;
}

fn runMode(allocator: std.mem.Allocator, header: []const u8, comptime summary_fmt: []const u8, live: bool, emit: bool) !CascadeReportData {
    const zig_bin = try findHolonBinary(allocator, zig_slug);
    defer allocator.free(zig_bin);
    const specs = [_]RoleSpec{
        .{ .slug = zig_slug, .binary = zig_bin },
        .{ .slug = zig_slug, .binary = zig_bin },
        .{ .slug = zig_slug, .binary = zig_bin },
        .{ .slug = zig_slug, .binary = zig_bin },
    };
    if (emit) try print("{s}\n\n", .{header});
    var pass: usize = 0;
    var fail: usize = 0;
    for (transports, 0..) |transport, phase_idx| {
        {
            var cascade = try spawnCascade(allocator, phase_idx + 1, transport, specs[0..]);
            defer cascade.deinit();
            defer cascade.stop();
            var previous: f64 = 0;
            for (1..run_ticks + 1) |tick_no| {
                const sender = try std.fmt.allocPrint(allocator, "phase-{}-tick-{}", .{ phase_idx + 1, tick_no });
                defer allocator.free(sender);
                const ok, previous = try runTick(allocator, &cascade, sender, previous);
                if (ok) pass += 1 else fail += 1;
                if (emit) try print("  Tick {}/{}: {s}\n", .{ tick_no, run_ticks, if (ok) "PASS" else "FAIL" });
            }
            if (live) {
                if (emit) try print("  Phase {}/4 ({s}) complete\n", .{ phase_idx + 1, transport });
            }
        }
    }
    if (emit and std.mem.eql(u8, summary_fmt, "Summary: {} ticks, {} PASS, {} FAIL")) {
        try print(summary_fmt ++ "\n", .{ pass + fail, pass, fail });
    } else if (emit) {
        try print(summary_fmt ++ "\n", .{ pass, fail, pass + fail });
    }
    if (fail != 0) return error.CascadeFailed;
    return .{ .ticks = pass + fail, .pass = pass, .fail = fail };
}

fn runMultiPattern(allocator: std.mem.Allocator, emit: bool) !MultiPatternReportData {
    const zig_bin = try findHolonBinary(allocator, zig_slug);
    defer allocator.free(zig_bin);
    const go_bin = try findHolonBinary(allocator, go_slug);
    defer allocator.free(go_bin);
    const patterns = [_]struct { name: []const u8, specs: [4]RoleSpec }{
        .{ .name = "zig-zig-zig-zig", .specs = .{
            .{ .slug = zig_slug, .binary = zig_bin }, .{ .slug = zig_slug, .binary = zig_bin },
            .{ .slug = zig_slug, .binary = zig_bin }, .{ .slug = zig_slug, .binary = zig_bin },
        } },
        .{ .name = "zig-zig-go-zig", .specs = .{
            .{ .slug = zig_slug, .binary = zig_bin }, .{ .slug = go_slug, .binary = go_bin },
            .{ .slug = zig_slug, .binary = zig_bin }, .{ .slug = zig_slug, .binary = zig_bin },
        } },
        .{ .name = "zig-zig-go-go", .specs = .{
            .{ .slug = go_slug, .binary = go_bin },   .{ .slug = go_slug, .binary = go_bin },
            .{ .slug = zig_slug, .binary = zig_bin }, .{ .slug = zig_slug, .binary = zig_bin },
        } },
    };
    if (emit) try print("=== observability-cascade-zig (multi-pattern) ===\n\n", .{});
    var pass: usize = 0;
    var fail: usize = 0;
    for (patterns, 0..) |pattern, pattern_idx| {
        if (emit) try print("Pattern: {s}\n", .{pattern.name});
        for (transports, 0..) |transport, phase_idx| {
            const phase_id = pattern_idx * transports.len + phase_idx + 1;
            {
                var cascade = try spawnCascade(allocator, phase_id, transport, pattern.specs[0..]);
                defer cascade.deinit();
                defer cascade.stop();
                var previous: f64 = 0;
                for (1..run_ticks + 1) |tick_no| {
                    const sender = try std.fmt.allocPrint(allocator, "{s}-phase-{}-tick-{}", .{ pattern.name, phase_idx + 1, tick_no });
                    defer allocator.free(sender);
                    const ok, previous = try runTick(allocator, &cascade, sender, previous);
                    if (ok) pass += 1 else fail += 1;
                }
            }
            if (emit) try print("  Phase {}/4 ({s}) complete\n", .{ phase_idx + 1, transport });
        }
    }
    if (emit) try print("Summary: {} PASS / {} FAIL across {} ticks\n", .{ pass, fail, pass + fail });
    if (fail != 0) return error.CascadeFailed;
    return .{ .total_pass = pass, .total_fail = fail };
}

fn spawnCascade(allocator: std.mem.Allocator, phase: usize, transport: []const u8, specs: []const RoleSpec) !Cascade {
    const home = getenv("HOME") orelse ".";
    const run_root = try std.fs.path.join(allocator, &.{ home, ".op", "run" });
    var cascade = Cascade{
        .allocator = allocator,
        .phase = phase,
        .transport = transport,
        .run_root = run_root,
        .role_list = undefined,
    };
    for (roles, 0..) |role_name, index| {
        const uid = try std.fmt.allocPrint(allocator, "relay-zig-p{d:0>2}-{s}", .{ phase, lowerRole(role_name) });
        cascade.role_list[index] = .{
            .name = role_name,
            .slug = specs[index].slug,
            .uid = uid,
            .listen_uri = try listenUri(allocator, phase, transport, role_name),
        };
    }
    for (&cascade.role_list, 0..) |*role, index| {
        if (index > 0) role.member_address = cascade.role_list[index - 1].listen_uri;
        try startRole(allocator, &cascade, role, specs[index].binary);
    }
    sleepMillis(1_500);
    _ = waitFor(allocator, &cascade, "", checkEvent);
    return cascade;
}

fn startRole(allocator: std.mem.Allocator, cascade: *Cascade, role: *Role, binary: []const u8) !void {
    const role_run_dir = try std.fs.path.join(allocator, &.{ cascade.run_root, role.slug, role.uid });
    defer allocator.free(role_run_dir);
    std.Io.Dir.cwd().deleteTree(std.Io.Threaded.global_single_threaded.io(), role_run_dir) catch {};
    if (std.mem.startsWith(u8, role.listen_uri, "unix://")) {
        const socket_path = try allocator.dupeZ(u8, role.listen_uri["unix://".len..]);
        _ = c.unlink(socket_path.ptr);
    }
    try setEnv("OP_OBS", "logs,events,metrics,prom");
    try setEnv("OP_RUN_DIR", cascade.run_root);
    try setEnv("OP_INSTANCE_UID", role.uid);
    try setEnv("OP_ORGANISM_UID", cascade.role_list[3].uid);
    try setEnv("OP_ORGANISM_SLUG", cascade.role_list[3].slug);
    try setEnv("OP_PROM_ADDR", "127.0.0.1:0");

    var argv_list: std.ArrayList([]const u8) = .empty;
    defer argv_list.deinit(allocator);
    try argv_list.appendSlice(allocator, &.{ binary, "serve", "--listen", role.listen_uri });
    if (role.member_address.len != 0) {
        const child = cascade.role_list[roleIndex(role.name) - 1];
        const member = try std.fmt.allocPrint(allocator, "{s}@{s}={s}", .{ child.slug, child.uid, role.member_address });
        try argv_list.appendSlice(allocator, &.{ "--member", member });
    }
    role.process = try spawnProcess(allocator, argv_list.items);
    role.metrics_addr = try waitMeta(allocator, cascade.run_root, role.slug, role.uid);
    role.conn = try dialReady(allocator, role.listen_uri);
    if (roleIndex(role.name) > 0) _ = waitRoleRelayReady(allocator, cascade, role);
}

fn spawnProcess(allocator: std.mem.Allocator, args: []const []const u8) !c.pid_t {
    if (args.len == 0) return error.EmptyCommand;

    const argv = try allocator.alloc(?[*:0]const u8, args.len + 1);
    for (args, 0..) |arg, index| {
        const zarg = try allocator.dupeZ(u8, arg);
        argv[index] = zarg.ptr;
    }
    argv[args.len] = null;

    const pid = c.fork();
    if (pid < 0) return error.ForkFailed;
    if (pid == 0) {
        const dev_null = c.open("/dev/null", c.O_RDWR);
        if (dev_null >= 0) {
            _ = c.dup2(dev_null, 0);
            _ = c.dup2(dev_null, 1);
            _ = c.dup2(dev_null, 2);
            if (dev_null > 2) _ = c.close(dev_null);
        }
        _ = c.execvp(argv[0].?, @ptrCast(argv.ptr));
        c._exit(127);
    }
    return pid;
}

fn runTick(allocator: std.mem.Allocator, cascade: *Cascade, sender: []const u8, previous: f64) !struct { bool, f64 } {
    var d = &cascade.role_list[0];
    const request = try packTickRequest(allocator, sender, cascade.transport);
    defer allocator.free(request);
    const response = try d.conn.?.unaryAlloc(allocator, "/relay.v1.RelayService/Tick", request, 5_000);
    allocator.free(response);
    const log_ok = waitFor(allocator, cascade, sender, checkLog);
    const event_ok = waitFor(allocator, cascade, sender, checkEvent);
    var next_metric = previous;
    const metric_ok = waitForMetric(allocator, cascade, previous, &next_metric);
    return .{ log_ok and event_ok and metric_ok, next_metric };
}

fn checkLog(allocator: std.mem.Allocator, cascade: *Cascade, sender: []const u8) bool {
    const entries = readLogs(allocator, &cascade.role_list[3]) catch return false;
    defer freeLogs(allocator, entries);
    for (entries) |entry| {
        if (!std.mem.eql(u8, entry.message, "tick received")) continue;
        if (!hasLabel(entry.fields, "sender", sender)) continue;
        if (!hasLabel(entry.fields, "responder_uid", cascade.role_list[0].uid)) continue;
        if (chainOk(cascade, entry.chain)) return true;
    }
    return false;
}

fn checkEvent(allocator: std.mem.Allocator, cascade: *Cascade, sender: []const u8) bool {
    _ = sender;
    const events = readEvents(allocator, &cascade.role_list[3]) catch return false;
    defer freeEvents(allocator, events);
    for (events) |event| {
        if (event.event_type != .instance_ready) continue;
        if (!std.mem.eql(u8, event.instance_uid, cascade.role_list[0].uid)) continue;
        if (chainOk(cascade, event.chain)) return true;
    }
    return false;
}

fn waitFor(allocator: std.mem.Allocator, cascade: *Cascade, sender: []const u8, comptime check: fn (std.mem.Allocator, *Cascade, []const u8) bool) bool {
    var attempts: usize = 0;
    while (attempts < 100) : (attempts += 1) {
        if (check(allocator, cascade, sender)) return true;
        sleepMillis(50);
    }
    return false;
}

fn waitForMetric(allocator: std.mem.Allocator, cascade: *Cascade, previous: f64, out: *f64) bool {
    var attempts: usize = 0;
    while (attempts < 100) : (attempts += 1) {
        const body = httpGet(allocator, cascade.role_list[0].metrics_addr) catch {
            sleepMillis(50);
            continue;
        };
        defer allocator.free(body);
        if (parseCascadeTicks(body, cascade.role_list[0].uid)) |value| {
            if (value > previous) {
                out.* = value;
                return true;
            }
        }
        sleepMillis(50);
    }
    return false;
}

fn waitRoleRelayReady(allocator: std.mem.Allocator, cascade: *Cascade, role: *Role) bool {
    var attempts: usize = 0;
    while (attempts < 100) : (attempts += 1) {
        if (roleRelayReady(allocator, cascade, role)) return true;
        sleepMillis(50);
    }
    return false;
}

fn roleRelayReady(allocator: std.mem.Allocator, cascade: *Cascade, role: *Role) bool {
    const events = readEvents(allocator, role) catch return false;
    defer freeEvents(allocator, events);
    const role_idx = roleIndex(role.name);
    for (events) |event| {
        if (event.event_type != .instance_ready) continue;
        if (!std.mem.eql(u8, event.instance_uid, cascade.role_list[0].uid)) continue;
        if (chainPrefixOk(cascade, event.chain, role_idx)) return true;
    }
    return false;
}

fn readLogs(allocator: std.mem.Allocator, role: *Role) ![]holons.observability.LogEntry {
    const req = try holons.protobuf.runtime.packLogsRequest(allocator, .{ .follow = false });
    defer allocator.free(req);
    var stream = try role.conn.?.serverStream(allocator, "/holons.v1.HolonObservability/Logs", req, 2_000);
    defer stream.deinit();
    var out: std.ArrayList(holons.observability.LogEntry) = .empty;
    while (try stream.next(allocator)) |bytes| {
        defer allocator.free(bytes);
        try out.append(allocator, try holons.observability.logEntryFromBytes(allocator, bytes));
    }
    return out.toOwnedSlice(allocator);
}

fn readEvents(allocator: std.mem.Allocator, role: *Role) ![]holons.observability.Event {
    const req = try holons.protobuf.runtime.packEventsRequest(allocator, .{ .follow = false });
    defer allocator.free(req);
    var stream = try role.conn.?.serverStream(allocator, "/holons.v1.HolonObservability/Events", req, 2_000);
    defer stream.deinit();
    var out: std.ArrayList(holons.observability.Event) = .empty;
    while (try stream.next(allocator)) |bytes| {
        defer allocator.free(bytes);
        try out.append(allocator, try holons.observability.eventFromBytes(allocator, bytes));
    }
    return out.toOwnedSlice(allocator);
}

fn packTickRequest(allocator: std.mem.Allocator, sender: []const u8, note: []const u8) ![]u8 {
    var request: c.Relay__V1__TickRequest = undefined;
    c.relay__v1__tick_request__init(&request);
    const sender_z = try allocator.dupeZ(u8, sender);
    defer allocator.free(sender_z);
    const note_z = try allocator.dupeZ(u8, note);
    defer allocator.free(note_z);
    request.sender = sender_z.ptr;
    request.note = note_z.ptr;
    const len = c.relay__v1__tick_request__get_packed_size(&request);
    const out = try allocator.alloc(u8, len);
    _ = c.relay__v1__tick_request__pack(&request, out.ptr);
    return out;
}

fn dialReady(allocator: std.mem.Allocator, uri: []const u8) !holons.grpc.client.Channel {
    var attempts: usize = 0;
    while (attempts < 100) : (attempts += 1) {
        var channel = holons.grpc.client.connect(allocator, uri) catch {
            sleepMillis(50);
            continue;
        };
        if (channel.describe(allocator)) |desc_value| {
            var desc = desc_value;
            desc.deinit();
            return channel;
        } else |_| {
            channel.deinit();
            sleepMillis(50);
        }
    }
    return error.DialTimeout;
}

fn waitMeta(allocator: std.mem.Allocator, run_root: []const u8, slug: []const u8, uid: []const u8) ![]u8 {
    const path = try std.fs.path.join(allocator, &.{ run_root, slug, uid, "meta.json" });
    defer allocator.free(path);
    var attempts: usize = 0;
    while (attempts < 200) : (attempts += 1) {
        const body = std.Io.Dir.cwd().readFileAlloc(std.Io.Threaded.global_single_threaded.io(), path, allocator, .limited(1024 * 1024)) catch {
            sleepMillis(50);
            continue;
        };
        defer allocator.free(body);
        if (std.mem.indexOf(u8, body, uid) != null) {
            if (jsonString(allocator, body, "metrics_addr")) |value| {
                if (value.len != 0) return value;
                allocator.free(value);
            } else |_| {}
        }
        sleepMillis(50);
    }
    return error.MetaTimeout;
}

fn httpGet(allocator: std.mem.Allocator, url: []const u8) ![]u8 {
    const prefix = "http://127.0.0.1:";
    if (!std.mem.startsWith(u8, url, prefix)) return error.UnsupportedUrl;
    const rest = url[prefix.len..];
    const slash = std.mem.indexOfScalar(u8, rest, '/') orelse return error.UnsupportedUrl;
    const port = try std.fmt.parseInt(u16, rest[0..slash], 10);
    const path = rest[slash..];
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
    var buf: [4096]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n < 0) return error.ReadFailed;
        if (n == 0) break;
        try out.appendSlice(allocator, buf[0..@intCast(n)]);
    }
    return out.toOwnedSlice(allocator);
}

fn parseCascadeTicks(body: []const u8, uid: []const u8) ?f64 {
    var lines = std.mem.splitScalar(u8, body, '\n');
    while (lines.next()) |line| {
        if (!std.mem.startsWith(u8, line, "cascade_ticks_total{")) continue;
        if (std.mem.indexOf(u8, line, uid) == null) continue;
        var fields = std.mem.tokenizeAny(u8, line, " \t\r");
        var last: []const u8 = "";
        while (fields.next()) |field| last = field;
        return std.fmt.parseFloat(f64, last) catch null;
    }
    return null;
}

fn chainOk(cascade: *Cascade, chain: []const holons.observability.Hop) bool {
    return chainPrefixOk(cascade, chain, 3);
}

fn chainPrefixOk(cascade: *Cascade, chain: []const holons.observability.Hop, count: usize) bool {
    if (chain.len < count) return false;
    for (0..count) |role_idx| {
        const role = cascade.role_list[role_idx];
        if (!std.mem.eql(u8, chain[role_idx].slug, role.slug)) return false;
        if (!std.mem.eql(u8, chain[role_idx].instance_uid, role.uid)) return false;
    }
    return true;
}

fn hasLabel(labels: []const holons.observability.Label, key: []const u8, value: []const u8) bool {
    for (labels) |label| if (std.mem.eql(u8, label.key, key) and std.mem.eql(u8, label.value, value)) return true;
    return false;
}

fn freeLogs(allocator: std.mem.Allocator, entries: []holons.observability.LogEntry) void {
    for (entries) |*entry| entry.deinit(allocator);
    allocator.free(entries);
}

fn freeEvents(allocator: std.mem.Allocator, entries: []holons.observability.Event) void {
    for (entries) |*entry| entry.deinit(allocator);
    allocator.free(entries);
}

fn findHolonBinary(allocator: std.mem.Allocator, slug: []const u8) ![]u8 {
    const env_name = try binaryEnvName(allocator, slug);
    defer allocator.free(env_name);
    const env_name_z = try allocator.dupeZ(u8, env_name);
    defer allocator.free(env_name_z);
    if (getenv(env_name_z.ptr)) |value| {
        const trimmed = std.mem.trim(u8, value, " \t\r\n");
        if (trimmed.len != 0) return allocator.dupe(u8, trimmed);
    }

    const source_root = try findSourceRoot(allocator);
    defer allocator.free(source_root);
    const examples_root = std.fs.path.dirname(source_root) orelse source_root;
    if (std.mem.eql(u8, slug, zig_slug)) {
        const node_root = try std.fs.path.join(allocator, &.{ source_root, "holons", "observability-cascade-node" });
        defer allocator.free(node_root);
        if (try findExecutableInBuildRoots(allocator, node_root, slug, "observability-cascade-node-zig")) |found| return found;
    }
    if (std.mem.eql(u8, slug, go_slug)) {
        const node_root = try std.fs.path.join(allocator, &.{ examples_root, "observability-cascade-go", "holons", "observability-cascade-node" });
        defer allocator.free(node_root);
        if (try findExecutableInBuildRoots(allocator, node_root, slug, "observability-cascade-node-go")) |found| return found;
    }

    const home = getenv("HOME") orelse return error.HomeMissing;
    const suffix = try std.fmt.allocPrint(allocator, ".op/bin/{s}.holon/bin", .{slug});
    defer allocator.free(suffix);
    const root = try std.fs.path.join(allocator, &.{ home, suffix });
    defer allocator.free(root);
    if (try findExecutable(allocator, root, slug)) |found| return found;
    return error.BinaryNotFound;
}

fn binaryEnvName(allocator: std.mem.Allocator, slug: []const u8) ![]u8 {
    const prefix = "observability-cascade-node-";
    const raw = if (std.mem.startsWith(u8, slug, prefix)) slug[prefix.len..] else slug;
    var out = try allocator.alloc(u8, "OBSERVABILITY_CASCADE_NODE_".len + raw.len + "_BIN".len);
    @memcpy(out[0.."OBSERVABILITY_CASCADE_NODE_".len], "OBSERVABILITY_CASCADE_NODE_");
    var cursor: usize = "OBSERVABILITY_CASCADE_NODE_".len;
    for (raw) |ch| {
        out[cursor] = if (ch == '-') '_' else std.ascii.toUpper(ch);
        cursor += 1;
    }
    @memcpy(out[cursor..][0.."_BIN".len], "_BIN");
    return out;
}

fn findSourceRoot(allocator: std.mem.Allocator) ![]u8 {
    const env_name = "OBSERVABILITY_CASCADE_ZIG_SOURCE_ROOT";
    if (getenv(env_name)) |value| {
        const trimmed = std.mem.trim(u8, value, " \t\r\n");
        if (trimmed.len != 0) return allocator.dupe(u8, trimmed);
    }
    var cwd_buffer: [std.Io.Dir.max_path_bytes]u8 = undefined;
    const cwd_len = try std.Io.Dir.cwd().realPath(std.Io.Threaded.global_single_threaded.io(), &cwd_buffer);
    var current = try allocator.dupe(u8, cwd_buffer[0..cwd_len]);
    errdefer allocator.free(current);
    while (true) {
        if (try isSourceRoot(allocator, current)) return current;
        const nested = try std.fs.path.join(allocator, &.{ current, "examples", "observability-cascade", "observability-cascade-zig" });
        defer allocator.free(nested);
        if (try isSourceRoot(allocator, nested)) {
            allocator.free(current);
            return allocator.dupe(u8, nested);
        }
        const parent = std.fs.path.dirname(current) orelse break;
        if (std.mem.eql(u8, parent, current)) break;
        const next = try allocator.dupe(u8, parent);
        allocator.free(current);
        current = next;
    }
    allocator.free(current);
    return error.SourceRootNotFound;
}

fn isSourceRoot(allocator: std.mem.Allocator, root: []const u8) !bool {
    const manifest = try std.fs.path.join(allocator, &.{ root, "api", "v1", "holon.proto" });
    defer allocator.free(manifest);
    const node = try std.fs.path.join(allocator, &.{ root, "holons", "observability-cascade-node" });
    defer allocator.free(node);
    return fileExists(allocator, manifest) and fileExists(allocator, node);
}

fn fileExists(allocator: std.mem.Allocator, path: []const u8) bool {
    const zpath = allocator.dupeZ(u8, path) catch return false;
    defer allocator.free(zpath);
    return c.access(zpath.ptr, c.F_OK) == 0;
}

fn findExecutableInBuildRoots(
    allocator: std.mem.Allocator,
    node_root: []const u8,
    slug: []const u8,
    package_slug: []const u8,
) !?[]u8 {
    const primary = try std.fs.path.join(allocator, &.{ node_root, ".op", "build", "observability-cascade-node.holon", "bin" });
    defer allocator.free(primary);
    if (try findExecutable(allocator, primary, slug)) |found| return found;
    const secondary_dir = try std.fmt.allocPrint(allocator, "{s}.holon", .{package_slug});
    defer allocator.free(secondary_dir);
    const secondary = try std.fs.path.join(allocator, &.{ node_root, ".op", "build", secondary_dir, "bin" });
    defer allocator.free(secondary);
    return try findExecutable(allocator, secondary, slug);
}

fn findExecutable(allocator: std.mem.Allocator, root: []const u8, slug: []const u8) !?[]u8 {
    var dir = std.Io.Dir.cwd().openDir(std.Io.Threaded.global_single_threaded.io(), root, .{ .iterate = true }) catch return null;
    defer dir.close(std.Io.Threaded.global_single_threaded.io());
    var walker = try dir.walk(allocator);
    defer walker.deinit();
    while (try walker.next(std.Io.Threaded.global_single_threaded.io())) |entry| {
        if (entry.kind == .file and std.mem.eql(u8, std.fs.path.basename(entry.path), slug)) {
            return try std.fs.path.join(allocator, &.{ root, entry.path });
        }
    }
    return null;
}

fn listenUri(allocator: std.mem.Allocator, phase: usize, transport: []const u8, role: []const u8) ![]u8 {
    const idx = roleIndex(role);
    if (std.mem.eql(u8, transport, "tcp")) return std.fmt.allocPrint(allocator, "tcp://127.0.0.1:{}", .{9090 + ((phase - 1) * 10) + idx});
    const path = try std.fmt.allocPrint(allocator, "/tmp/observability-cascade-zig-p{}-{s}.sock", .{ phase, lowerRole(role) });
    defer allocator.free(path);
    return std.fmt.allocPrint(allocator, "unix://{s}", .{path});
}

fn roleIndex(role: []const u8) usize {
    if (std.mem.eql(u8, role, "D")) return 0;
    if (std.mem.eql(u8, role, "C")) return 1;
    if (std.mem.eql(u8, role, "B")) return 2;
    return 3;
}

fn lowerRole(role: []const u8) []const u8 {
    return switch (role[0]) {
        'D' => "d",
        'C' => "c",
        'B' => "b",
        else => "a",
    };
}

fn jsonString(allocator: std.mem.Allocator, body: []const u8, key: []const u8) ![]u8 {
    const needle = try std.fmt.allocPrint(allocator, "\"{s}\"", .{key});
    defer allocator.free(needle);
    const key_start = std.mem.indexOf(u8, body, needle) orelse return error.JsonKeyMissing;
    var cursor = key_start + needle.len;
    while (cursor < body.len and std.ascii.isWhitespace(body[cursor])) : (cursor += 1) {}
    if (cursor >= body.len or body[cursor] != ':') return error.JsonColonMissing;
    cursor += 1;
    while (cursor < body.len and std.ascii.isWhitespace(body[cursor])) : (cursor += 1) {}
    if (cursor >= body.len or body[cursor] != '"') return error.JsonStringStartMissing;
    const value_start = cursor + 1;
    const rel_end = std.mem.indexOfScalar(u8, body[value_start..], '"') orelse return error.JsonStringEndMissing;
    return allocator.dupe(u8, body[value_start .. value_start + rel_end]);
}

fn setEnv(key: []const u8, value: []const u8) !void {
    const key_z = try std.heap.c_allocator.dupeZ(u8, key);
    defer std.heap.c_allocator.free(key_z);
    const value_z = try std.heap.c_allocator.dupeZ(u8, value);
    defer std.heap.c_allocator.free(value_z);
    if (c.setenv(key_z.ptr, value_z.ptr, 1) != 0) return error.SetEnvFailed;
}

fn getenv(key: [*:0]const u8) ?[]const u8 {
    const value = c.getenv(key) orelse return null;
    return std.mem.span(value);
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(std.Io.Threaded.global_single_threaded.io(), std.Io.Duration.fromMilliseconds(ms), .awake) catch {};
}

fn print(comptime fmt: []const u8, args: anytype) !void {
    var buf: [512]u8 = undefined;
    const out = try std.fmt.bufPrint(&buf, fmt, args);
    _ = c.write(c.STDOUT_FILENO, out.ptr, out.len);
}
