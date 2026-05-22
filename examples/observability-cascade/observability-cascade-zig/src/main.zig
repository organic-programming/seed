const std = @import("std");
const holons = @import("zig_holons");
const describe_generated = @import("describe_generated");

const c = @cImport({
    @cInclude("stdlib.h");
    @cInclude("unistd.h");
    @cInclude("protobuf-c/protobuf-c.h");
    @cInclude("observability_cascade/v1/service.pb-c.h");
});

const zig_slug = "observability-cascade-zig-node";
const go_slug = "observability-cascade-go-node";
const run_ticks = 3;

const rpc_methods = [_]holons.grpc.server.Method{
    .{ .path = "/observability_cascade.v1.ObservabilityCascadeService/RunDefault", .handler = runDefaultRpc },
    .{ .path = "/observability_cascade.v1.ObservabilityCascadeService/RunLiveStream", .handler = runLiveStreamRpc },
    .{ .path = "/observability_cascade.v1.ObservabilityCascadeService/RunMultiPattern", .handler = runMultiPatternRpc },
};

const LanguageMember = struct {
    lang: []const u8,
    slug: []const u8,
    binary: []const u8,
};

const PhaseResult = struct {
    name: []const u8,
    pass: i32 = 0,
    fail: i32 = 0,
    failures: std.ArrayList([]const u8) = .empty,
    elapsed_us: i64 = 0,
};

const CascadeReport = struct {
    name: []const u8,
    ticks: i32 = 0,
    pass: i32 = 0,
    fail: i32 = 0,
    phases: std.ArrayList(PhaseResult) = .empty,
    elapsed_us: i64 = 0,
};

const MultiPatternReport = struct {
    patterns: std.ArrayList(CascadeReport) = .empty,
    total_pass: i32 = 0,
    total_fail: i32 = 0,
    total_elapsed_us: i64 = 0,
};

const TickResult = struct {
    pass: bool = false,
    log: holons.composite.CheckOutcome = .{},
    event: holons.composite.CheckOutcome = .{},
    hops: holons.composite.CheckOutcome = .{},
};

pub fn main(init: std.process.Init.Minimal) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.c_allocator);
    defer arena.deinit();
    const allocator = arena.allocator();
    const args = try init.args.toSlice(allocator);
    const mode = if (args.len > 1) args[1] else "";

    holons.describe.useStaticResponse(describe_generated.staticDescribeResponse());
    if (std.mem.eql(u8, mode, "serve")) {
        try ensureCascadeObservability();
        var options = try holons.serve.parseOptions(args[1..]);
        options.methods = rpc_methods[0..];
        try holons.serve.runSingle(options);
        return;
    }

    var failed: i32 = 0;
    if (std.mem.eql(u8, mode, "--multi-pattern")) {
        const report = try runMultiPattern(allocator, true);
        failed = report.total_fail;
    } else {
        const live = std.mem.eql(u8, mode, "--live-stream");
        const report = try runReport(allocator, if (live) "live-stream" else "default", try ownMembers(allocator), live, true);
        failed = report.fail;
    }
    if (failed > 0) return error.CascadeFailed;
}

fn runDefaultRpc(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    _ = bytes;
    const report = try runReport(allocator, "default", try ownMembers(allocator), false, false);
    return packCascadeReport(allocator, report);
}

fn runLiveStreamRpc(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    _ = bytes;
    const report = try runReport(allocator, "live-stream", try ownMembers(allocator), true, false);
    return packCascadeReport(allocator, report);
}

fn runMultiPatternRpc(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    _ = bytes;
    const report = try runMultiPattern(allocator, false);
    return packMultiPatternReport(allocator, report);
}

fn runMultiPattern(allocator: std.mem.Allocator, emit: bool) !MultiPatternReport {
    const start = nowNs();
    const patterns = try zigPatterns(allocator);
    var out = MultiPatternReport{};
    if (emit) try print("=== observability-cascade-zig --multi-pattern ===\n\n", .{});
    for (patterns, 0..) |pattern, index| {
        if (emit) try print("Pattern {}/{}: {s}\n", .{ index + 1, patterns.len, pattern.name });
        const report = try runReport(allocator, pattern.name, pattern.members[0..], true, emit);
        try out.patterns.append(allocator, report);
        out.total_pass += report.pass;
        out.total_fail += report.fail;
        if (emit) {
            try print("Pattern {s}: {}/{} {s} (elapsed={}us)\n\n", .{
                pattern.name,
                report.pass,
                report.ticks,
                if (report.fail == 0) "PASS" else "FAIL",
                report.elapsed_us,
            });
        }
    }
    out.total_elapsed_us = elapsedUs(start);
    if (emit) try print("Summary: {} PASS / {} FAIL across {} ticks (total elapsed={}us)\n", .{
        out.total_pass,
        out.total_fail,
        out.total_pass + out.total_fail,
        out.total_elapsed_us,
    });
    return out;
}

fn runReport(allocator: std.mem.Allocator, name: []const u8, members: []const LanguageMember, live: bool, emit: bool) !CascadeReport {
    try ensureCascadeObservability();
    const start = nowNs();
    var report = CascadeReport{ .name = name };
    const poll_ms: i64 = if (live) 50 else 100;
    const timeout_ms: i64 = 5_000;

    if (emit) try print("=== observability-cascade-zig {s}===\n\n", .{modeSuffix(name)});
    for (holons.composite.TransportCoverageSequence, 0..) |transport_name, phase_idx| {
        const phase_start = nowNs();
        const from = if (phase_idx == 0) transport_name else holons.composite.TransportCoverageSequence[phase_idx - 1];
        var phase = PhaseResult{
            .name = try std.fmt.allocPrint(allocator, "{d:0>2}-{s}→{s}", .{ phase_idx + 1, from, transport_name }),
        };
        if (emit) try print("Phase {}/{}: {s}\n", .{ phase_idx + 1, holons.composite.TransportCoverageSequence.len, phase.name });

        const specs = try childSpecs(allocator, members);
        var cascade = holons.composite.BuildCascade(allocator, .{
            .transport_name = transport_name,
            .members = specs,
            .extra_env = &.{
                .{ .key = "OP_OBS", .value = "logs,events,metrics,prom" },
                .{ .key = "OP_PROM_ADDR", .value = "127.0.0.1:0" },
            },
        }) catch |err| {
            phase.fail += run_ticks;
            try phase.failures.append(allocator, try std.fmt.allocPrint(allocator, "spawn: {s}", .{@errorName(err)}));
            phase.elapsed_us = elapsedUs(phase_start);
            addPhase(&report, phase);
            if (emit) try printPhaseSummary(phase);
            continue;
        };
        defer cascade.Stop();

        for (1..run_ticks + 1) |tick_no| {
            const sender = try std.fmt.allocPrint(allocator, "{s}-phase-{d:0>2}-tick-{}", .{ name, phase_idx + 1, tick_no });
            const result = runTick(allocator, &cascade, sender, transport_name, members, timeout_ms, poll_ms, live);
            if (result.pass) {
                phase.pass += 1;
            } else {
                phase.fail += 1;
                try phase.failures.append(allocator, try evidenceLine(allocator, tick_no, result));
            }
            if (emit) {
                try print("  Tick {}/{}: {s}\n", .{ tick_no, run_ticks, if (result.pass) "PASS" else "FAIL" });
            }
        }
        cascade.Stop();
        phase.elapsed_us = elapsedUs(phase_start);
        addPhase(&report, phase);
        if (emit) try printPhaseSummary(phase);
    }
    report.elapsed_us = elapsedUs(start);
    if (emit) try print("\nSummary: {} ticks, {} PASS, {} FAIL (total elapsed={}us)\n", .{
        report.ticks,
        report.pass,
        report.fail,
        report.elapsed_us,
    });
    return report;
}

fn runTick(
    allocator: std.mem.Allocator,
    cascade: *holons.composite.Cascade,
    sender: []const u8,
    note: []const u8,
    members: []const LanguageMember,
    timeout_ms: i64,
    poll_ms: i64,
    live: bool,
) TickResult {
    const request = holons.relay.packTickRequest(allocator, sender, note) catch |err| {
        const out = holons.composite.CheckOutcome{ .evidence = @errorName(err) };
        return .{ .log = out, .event = out, .hops = out };
    };
    defer allocator.free(request);
    const response_bytes = cascade.top.conn.unaryAlloc(allocator, "/relay.v1.RelayService/Tick", request, 5_000) catch |err| {
        const out = holons.composite.CheckOutcome{ .evidence = @errorName(err) };
        return .{ .log = out, .event = out, .hops = out };
    };
    defer allocator.free(response_bytes);
    var response = holons.relay.unpackTickResponse(allocator, response_bytes) catch |err| {
        const out = holons.composite.CheckOutcome{ .evidence = @errorName(err) };
        return .{ .log = out, .event = out, .hops = out };
    };
    defer response.deinit(allocator);

    const hops = checkHops(response.hops, members);
    if (!hops.pass) return .{ .hops = hops, .log = .{ .evidence = "skipped" }, .event = .{ .evidence = "skipped" } };

    const expected = hopChain(allocator, response.hops) catch return .{ .hops = .{ .evidence = "chain allocation failed" } };
    defer allocator.free(expected);
    const leaf_uid = response.hops[0].uid;
    const log = holons.composite.CheckRelayedLog(allocator, .{
        .sender = sender,
        .leaf_uid = leaf_uid,
        .expected_chain = expected,
        .timeout_ms = timeout_ms,
        .poll_ms = poll_ms,
        .live = live,
    });
    const event = holons.composite.CheckRelayedEvent(allocator, .{
        .event_type = .instance_ready,
        .leaf_uid = leaf_uid,
        .expected_chain = expected,
        .timeout_ms = timeout_ms,
        .poll_ms = poll_ms,
        .live = live,
    });
    return .{ .pass = hops.pass and log.pass and event.pass, .hops = hops, .log = log, .event = event };
}

fn checkHops(hops: []const holons.relay.HopReceipt, members: []const LanguageMember) holons.composite.CheckOutcome {
    if (hops.len != members.len) return .{ .evidence = "wrong hop count" };
    for (hops, 0..) |hop, index| {
        const want = members[members.len - 1 - index];
        if (!std.mem.eql(u8, hop.slug, want.slug)) return .{ .evidence = "wrong hop slug" };
        if (hop.uid.len == 0) return .{ .evidence = "empty hop uid" };
        if (hop.received <= 0) return .{ .evidence = "invalid received count" };
    }
    return .{ .pass = true, .evidence = "ok" };
}

fn hopChain(allocator: std.mem.Allocator, hops: []const holons.relay.HopReceipt) ![]holons.composite.ChainHop {
    var out = try allocator.alloc(holons.composite.ChainHop, hops.len);
    for (hops, 0..) |hop, index| {
        out[index] = .{ .slug = hop.slug, .instance_uid = hop.uid };
    }
    return out;
}

fn ownMembers(allocator: std.mem.Allocator) ![]const LanguageMember {
    const zig_bin = try holons.composite.member(allocator, "zig-node");
    return try allocator.dupe(LanguageMember, &.{
        .{ .lang = "zig", .slug = zig_slug, .binary = zig_bin },
        .{ .lang = "zig", .slug = zig_slug, .binary = zig_bin },
        .{ .lang = "zig", .slug = zig_slug, .binary = zig_bin },
    });
}

const NamedPattern = struct {
    name: []const u8,
    members: [3]LanguageMember,
};

fn zigPatterns(allocator: std.mem.Allocator) ![]const NamedPattern {
    const zig_bin = try holons.composite.member(allocator, "zig-node");
    const go_bin = try holons.composite.member(allocator, "go-node");
    const zig_member = LanguageMember{ .lang = "zig", .slug = zig_slug, .binary = zig_bin };
    const go_member = LanguageMember{ .lang = "go", .slug = go_slug, .binary = go_bin };
    return try allocator.dupe(NamedPattern, &.{
        .{ .name = "zig-zig-zig", .members = .{ zig_member, zig_member, zig_member } },
        .{ .name = "zig-zig-go", .members = .{ zig_member, zig_member, go_member } },
        .{ .name = "zig-go-zig", .members = .{ zig_member, go_member, zig_member } },
        .{ .name = "zig-go-go", .members = .{ zig_member, go_member, go_member } },
        .{ .name = "go-zig-zig", .members = .{ go_member, zig_member, zig_member } },
        .{ .name = "go-zig-go", .members = .{ go_member, zig_member, go_member } },
        .{ .name = "go-go-zig", .members = .{ go_member, go_member, zig_member } },
        .{ .name = "go-go-go", .members = .{ go_member, go_member, go_member } },
    });
}

fn childSpecs(allocator: std.mem.Allocator, members: []const LanguageMember) ![]holons.composite.ChildSpec {
    var out = try allocator.alloc(holons.composite.ChildSpec, members.len);
    for (members, 0..) |member, index| {
        out[index] = .{ .slug = member.slug, .binary = member.binary };
    }
    return out;
}

fn addPhase(report: *CascadeReport, phase: PhaseResult) void {
    report.phases.append(std.heap.c_allocator, phase) catch return;
    report.pass += phase.pass;
    report.fail += phase.fail;
    report.ticks += phase.pass + phase.fail;
}

fn ensureCascadeObservability() !void {
    if (holons.observability.current()) |obs| {
        if (obs.enabled(.logs) and obs.enabled(.events) and obs.enabled(.metrics)) return;
    }
    try setEnv("OP_OBS", "logs,events,metrics,prom");
    try setEnv("OP_PROM_ADDR", "127.0.0.1:0");
    _ = try holons.observability.configure(std.heap.c_allocator, .{
        .slug = "observability-cascade-zig",
        .logs_ring_size = 4096,
        .events_ring_size = 4096,
    });
}

fn packCascadeReport(allocator: std.mem.Allocator, report: CascadeReport) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();
    var response: c.ObservabilityCascade__V1__CascadeReport = undefined;
    try fillCascadeReport(a, report, &response);
    const len = c.observability_cascade__v1__cascade_report__get_packed_size(&response);
    const out = try allocator.alloc(u8, len);
    const encoded = c.observability_cascade__v1__cascade_report__pack(&response, out.ptr);
    if (encoded != len) return error.EncodeCascadeReportFailed;
    return out;
}

fn packMultiPatternReport(allocator: std.mem.Allocator, report: MultiPatternReport) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();
    var response: c.ObservabilityCascade__V1__MultiPatternReport = undefined;
    c.observability_cascade__v1__multi_pattern_report__init(&response);
    response.total_pass = report.total_pass;
    response.total_fail = report.total_fail;
    response.total_elapsed_us = report.total_elapsed_us;
    response.n_patterns = report.patterns.items.len;
    const pattern_ptrs = try a.alloc([*c]c.ObservabilityCascade__V1__CascadeReport, report.patterns.items.len);
    const pattern_msgs = try a.alloc(c.ObservabilityCascade__V1__CascadeReport, report.patterns.items.len);
    for (report.patterns.items, 0..) |pattern, index| {
        try fillCascadeReport(a, pattern, &pattern_msgs[index]);
        pattern_ptrs[index] = &pattern_msgs[index];
    }
    response.patterns = if (pattern_ptrs.len == 0) null else pattern_ptrs.ptr;
    const len = c.observability_cascade__v1__multi_pattern_report__get_packed_size(&response);
    const out = try allocator.alloc(u8, len);
    const encoded = c.observability_cascade__v1__multi_pattern_report__pack(&response, out.ptr);
    if (encoded != len) return error.EncodeMultiPatternReportFailed;
    return out;
}

fn fillCascadeReport(
    allocator: std.mem.Allocator,
    report: CascadeReport,
    response: *c.ObservabilityCascade__V1__CascadeReport,
) !void {
    c.observability_cascade__v1__cascade_report__init(response);
    response.name = (try allocator.dupeZ(u8, report.name)).ptr;
    response.ticks = report.ticks;
    response.pass = report.pass;
    response.fail = report.fail;
    response.elapsed_us = report.elapsed_us;
    response.n_phases = report.phases.items.len;
    const phase_ptrs = try allocator.alloc([*c]c.ObservabilityCascade__V1__PhaseResult, report.phases.items.len);
    const phase_msgs = try allocator.alloc(c.ObservabilityCascade__V1__PhaseResult, report.phases.items.len);
    for (report.phases.items, 0..) |phase, index| {
        c.observability_cascade__v1__phase_result__init(&phase_msgs[index]);
        phase_msgs[index].name = (try allocator.dupeZ(u8, phase.name)).ptr;
        phase_msgs[index].pass = phase.pass;
        phase_msgs[index].fail = phase.fail;
        phase_msgs[index].elapsed_us = phase.elapsed_us;
        phase_msgs[index].n_failures = phase.failures.items.len;
        const failures = try allocator.alloc([*c]u8, phase.failures.items.len);
        for (phase.failures.items, 0..) |failure, failure_index| {
            failures[failure_index] = (try allocator.dupeZ(u8, failure)).ptr;
        }
        phase_msgs[index].failures = if (failures.len == 0) null else failures.ptr;
        phase_ptrs[index] = &phase_msgs[index];
    }
    response.phases = if (phase_ptrs.len == 0) null else phase_ptrs.ptr;
}

fn evidenceLine(allocator: std.mem.Allocator, tick_no: usize, result: TickResult) ![]u8 {
    return std.fmt.allocPrint(allocator, "tick={} log={s} event={s} hops={s}", .{
        tick_no,
        evidenceText(result.log),
        evidenceText(result.event),
        evidenceText(result.hops),
    });
}

fn evidenceText(out: holons.composite.CheckOutcome) []const u8 {
    return if (out.pass) "ok" else if (out.evidence.len == 0) "<empty>" else out.evidence;
}

fn printPhaseSummary(phase: PhaseResult) !void {
    try print("Phase {s}: {}/{} {s} (elapsed={}us)\n", .{
        phase.name,
        phase.pass,
        phase.pass + phase.fail,
        if (phase.fail == 0) "PASS" else "FAIL",
        phase.elapsed_us,
    });
}

fn modeSuffix(name: []const u8) []const u8 {
    if (std.mem.eql(u8, name, "default")) return "";
    return "--live-stream ";
}

fn setEnv(key: []const u8, value: []const u8) !void {
    const key_z = try std.heap.c_allocator.dupeZ(u8, key);
    defer std.heap.c_allocator.free(key_z);
    const value_z = try std.heap.c_allocator.dupeZ(u8, value);
    defer std.heap.c_allocator.free(value_z);
    if (c.setenv(key_z.ptr, value_z.ptr, 1) != 0) return error.SetEnvFailed;
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}

fn elapsedUs(start_ns: i128) i64 {
    const elapsed = @max(nowNs() - start_ns, 1);
    return @intCast(@divTrunc(elapsed, std.time.ns_per_us));
}

fn print(comptime fmt: []const u8, args: anytype) !void {
    var buf: [1024]u8 = undefined;
    const out = try std.fmt.bufPrint(&buf, fmt, args);
    _ = c.write(c.STDOUT_FILENO, out.ptr, out.len);
}
