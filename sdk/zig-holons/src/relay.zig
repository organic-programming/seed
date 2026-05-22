const std = @import("std");
const grpc_client = @import("grpc/client.zig");
const grpc_server = @import("grpc/server.zig");
const observability = @import("observability.zig");

const c = @cImport({
    @cInclude("protobuf-c/protobuf-c.h");
    @cInclude("relay/v1/relay.pb-c.h");
});

const tick_method = "/relay.v1.RelayService/Tick";

var downstream_mutex: std.Io.Mutex = .init;
var downstream_conn: ?*grpc_client.Channel = null;
var downstream_required = std.atomic.Value(bool).init(false);
var received_count = std.atomic.Value(i64).init(0);

pub const HopReceipt = struct {
    slug: []const u8,
    uid: []const u8,
    received: i64,
};

pub const TickResponse = struct {
    responder_slug: []u8,
    responder_instance_uid: []u8,
    hops: []HopReceipt,

    pub fn deinit(self: *TickResponse, allocator: std.mem.Allocator) void {
        allocator.free(self.responder_slug);
        allocator.free(self.responder_instance_uid);
        for (self.hops) |hop| {
            allocator.free(hop.slug);
            allocator.free(hop.uid);
        }
        allocator.free(self.hops);
        self.* = .{ .responder_slug = &.{}, .responder_instance_uid = &.{}, .hops = &.{} };
    }
};

const service_methods = [_]grpc_server.Method{
    .{ .path = tick_method, .handler = tick },
};

pub fn serviceMethods(conn: ?*grpc_client.Channel) []const grpc_server.Method {
    setDownstream(conn);
    return service_methods[0..];
}

pub fn setDownstream(conn: ?*grpc_client.Channel) void {
    downstream_mutex.lockUncancelable(std.Options.debug_io);
    downstream_conn = conn;
    downstream_mutex.unlock(std.Options.debug_io);
}

pub fn requireDownstream(required: bool) void {
    downstream_required.store(required, .release);
}

pub fn packTickRequest(allocator: std.mem.Allocator, sender: []const u8, note: []const u8) ![]u8 {
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
    const encoded = c.relay__v1__tick_request__pack(&request, out.ptr);
    if (encoded != len) return error.EncodeTickRequestFailed;
    return out;
}

pub fn unpackTickResponse(allocator: std.mem.Allocator, bytes: []const u8) !TickResponse {
    const raw = c.relay__v1__tick_response__unpack(null, bytes.len, bytes.ptr) orelse return error.DecodeTickResponseFailed;
    defer c.relay__v1__tick_response__free_unpacked(raw, null);
    var hops = try allocator.alloc(HopReceipt, raw.*.n_hops);
    errdefer allocator.free(hops);
    for (raw.*.hops[0..raw.*.n_hops], 0..) |hop_raw, index| {
        hops[index] = .{
            .slug = try allocator.dupe(u8, cstr(hop_raw.*.slug)),
            .uid = try allocator.dupe(u8, cstr(hop_raw.*.uid)),
            .received = hop_raw.*.received,
        };
    }
    return .{
        .responder_slug = try allocator.dupe(u8, cstr(raw.*.responder_slug)),
        .responder_instance_uid = try allocator.dupe(u8, cstr(raw.*.responder_instance_uid)),
        .hops = hops,
    };
}

fn tick(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    const request = c.relay__v1__tick_request__unpack(null, bytes.len, bytes.ptr) orelse return error.DecodeTickRequestFailed;
    defer c.relay__v1__tick_request__free_unpacked(request, null);

    const obs = observability.current() orelse return error.ObservabilityNotConfigured;
    const slug = responderSlug(obs);
    const uid = obs.cfg.instance_uid;
    const count = received_count.fetchAdd(1, .acq_rel) + 1;

    var logger = obs.logger("tick");
    try logger.info("tick received", &.{
        observability.Field.string("sender", cstr(request.*.sender)),
        observability.Field.string("note", cstr(request.*.note)),
        observability.Field.string("responder_slug", slug),
        observability.Field.string("responder_uid", uid),
    });
    if (try obs.counter("cascade_ticks_total", "Ticks received by this cascade node.", &.{
        .{ .key = "responder_uid", .value = uid },
    })) |counter| {
        counter.inc();
    }

    var downstream_response: ?TickResponse = null;
    if (currentDownstreamForTick()) |conn| {
        const response_bytes = try conn.unaryAlloc(allocator, tick_method, bytes, 5_000);
        defer allocator.free(response_bytes);
        downstream_response = try unpackTickResponse(allocator, response_bytes);
    } else if (downstream_required.load(.acquire)) {
        return error.DownstreamNotReady;
    }
    defer if (downstream_response) |*response| response.deinit(allocator);

    const downstream_hops = if (downstream_response) |response| response.hops else &.{};
    var response = try buildTickResponse(allocator, slug, uid, count, downstream_hops);
    defer response.deinit(allocator);
    return packTickResponseMessage(allocator, response);
}

fn currentDownstream() ?*grpc_client.Channel {
    downstream_mutex.lockUncancelable(std.Options.debug_io);
    defer downstream_mutex.unlock(std.Options.debug_io);
    return downstream_conn;
}

fn currentDownstreamForTick() ?*grpc_client.Channel {
    const deadline_ns = nowNs() + (5_000 * std.time.ns_per_ms);
    while (true) {
        if (currentDownstream()) |conn| return conn;
        if (!downstream_required.load(.acquire) or nowNs() >= deadline_ns) return null;
        sleepMillis(25);
    }
}

fn buildTickResponse(
    allocator: std.mem.Allocator,
    slug: []const u8,
    uid: []const u8,
    count: i64,
    downstream_hops: []const HopReceipt,
) !TickResponse {
    var hops = try allocator.alloc(HopReceipt, downstream_hops.len + 1);
    errdefer allocator.free(hops);
    for (downstream_hops, 0..) |hop, index| {
        hops[index] = .{
            .slug = try allocator.dupe(u8, hop.slug),
            .uid = try allocator.dupe(u8, hop.uid),
            .received = hop.received,
        };
    }
    hops[downstream_hops.len] = .{
        .slug = try allocator.dupe(u8, slug),
        .uid = try allocator.dupe(u8, uid),
        .received = count,
    };
    return .{
        .responder_slug = try allocator.dupe(u8, slug),
        .responder_instance_uid = try allocator.dupe(u8, uid),
        .hops = hops,
    };
}

fn packTickResponseMessage(allocator: std.mem.Allocator, response: TickResponse) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var msg: c.Relay__V1__TickResponse = undefined;
    c.relay__v1__tick_response__init(&msg);
    msg.responder_slug = (try a.dupeZ(u8, response.responder_slug)).ptr;
    msg.responder_instance_uid = (try a.dupeZ(u8, response.responder_instance_uid)).ptr;
    msg.n_hops = response.hops.len;
    const hop_ptrs = try a.alloc([*c]c.Relay__V1__HopReceipt, response.hops.len);
    const hop_msgs = try a.alloc(c.Relay__V1__HopReceipt, response.hops.len);
    for (response.hops, 0..) |hop, index| {
        c.relay__v1__hop_receipt__init(&hop_msgs[index]);
        hop_msgs[index].slug = (try a.dupeZ(u8, hop.slug)).ptr;
        hop_msgs[index].uid = (try a.dupeZ(u8, hop.uid)).ptr;
        hop_msgs[index].received = hop.received;
        hop_ptrs[index] = &hop_msgs[index];
    }
    msg.hops = if (hop_ptrs.len == 0) null else hop_ptrs.ptr;
    const len = c.relay__v1__tick_response__get_packed_size(&msg);
    const out = try allocator.alloc(u8, len);
    const encoded = c.relay__v1__tick_response__pack(&msg, out.ptr);
    if (encoded != len) return error.EncodeTickResponseFailed;
    return out;
}

fn responderSlug(obs: *observability.Observability) []const u8 {
    if (std.mem.trim(u8, obs.cfg.slug, " \t\r\n").len != 0) return obs.cfg.slug;
    return "zig-holon";
}

fn cstr(value: [*c]const u8) []const u8 {
    if (value == null) return "";
    return std.mem.span(value);
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(
        std.Io.Threaded.global_single_threaded.io(),
        std.Io.Duration.fromMilliseconds(@intCast(ms)),
        .awake,
    ) catch {};
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}
