const std = @import("std");
const grpc_client = @import("grpc/client.zig");
const observability = @import("observability.zig");
const runtime = @import("protobuf/runtime.zig");

const STREAM_TIMEOUT_MS = 500;
const RESOLVE_TIMEOUT_MS = 2_000;
const RETRY_BACKOFF_MS = 100;

pub const MemberRef = struct {
    slug: []const u8 = "",
    uid: []const u8 = "",
    address: []const u8 = "",

    fn cloneTrimmed(allocator: std.mem.Allocator, raw: MemberRef) !MemberRef {
        return .{
            .slug = try allocator.dupe(u8, std.mem.trim(u8, raw.slug, " \t\r\n")),
            .uid = try allocator.dupe(u8, std.mem.trim(u8, raw.uid, " \t\r\n")),
            .address = try allocator.dupe(u8, std.mem.trim(u8, raw.address, " \t\r\n")),
        };
    }

    pub fn deinit(self: *MemberRef, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.uid);
        allocator.free(self.address);
        self.* = .{};
    }
};

pub const ParseError = error{
    InvalidMemberSpec,
    MissingMemberSlug,
    MissingMemberAddress,
};

pub const MemberRelay = struct {
    allocator: std.mem.Allocator,
    obs: *observability.Observability,
    member: MemberRef,
    stop_requested: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),
    logs_thread: ?std.Thread = null,
    events_thread: ?std.Thread = null,

    fn start(self: *MemberRelay) !void {
        if (self.obs.enabled(.logs) and self.obs.log_ring != null) {
            self.logs_thread = try std.Thread.spawn(.{}, pumpLogs, .{self});
        }
        if (self.obs.enabled(.events) and self.obs.event_bus != null) {
            self.events_thread = try std.Thread.spawn(.{}, pumpEvents, .{self});
        }
    }

    fn stop(self: *MemberRelay) void {
        self.stop_requested.store(true, .release);
        if (self.logs_thread) |thread| thread.join();
        if (self.events_thread) |thread| thread.join();
        self.member.deinit(self.allocator);
    }

    fn stopping(self: *MemberRelay) bool {
        return self.stop_requested.load(.acquire);
    }
};

pub fn parseMember(raw: []const u8) !MemberRef {
    const first_eq = std.mem.indexOfScalar(u8, raw, '=') orelse return error.InvalidMemberSpec;
    const left = std.mem.trim(u8, raw[0..first_eq], " \t\r\n");
    const address = std.mem.trim(u8, raw[first_eq + 1 ..], " \t\r\n");
    if (left.len == 0) return error.MissingMemberSlug;
    if (address.len == 0) return error.MissingMemberAddress;
    const at = std.mem.indexOfScalar(u8, left, '@');
    return .{
        .slug = if (at) |index| std.mem.trim(u8, left[0..index], " \t\r\n") else left,
        .uid = if (at) |index| std.mem.trim(u8, left[index + 1 ..], " \t\r\n") else "",
        .address = address,
    };
}

pub fn startAll(allocator: std.mem.Allocator, obs: *observability.Observability, members: []const MemberRef) ![]MemberRelay {
    var relays = try allocator.alloc(MemberRelay, members.len);
    errdefer allocator.free(relays);
    var count: usize = 0;
    errdefer {
        for (relays[0..count]) |*relay| relay.stop();
    }

    if (!obs.enabled(.logs) and !obs.enabled(.events)) return relays[0..0];
    for (members) |raw| {
        var member = try MemberRef.cloneTrimmed(allocator, raw);
        errdefer member.deinit(allocator);
        if (member.slug.len == 0 or member.address.len == 0) {
            member.deinit(allocator);
            continue;
        }
        resolveMemberIdentity(allocator, &member) catch {};
        try emitResolvedMemberReady(allocator, obs, member);
        relays[count] = .{
            .allocator = allocator,
            .obs = obs,
            .member = member,
        };
        try relays[count].start();
        count += 1;
    }
    return relays[0..count];
}

pub fn stopAll(allocator: std.mem.Allocator, relays: []MemberRelay) void {
    for (relays) |*relay| relay.stop();
    allocator.free(relays);
}

fn resolveMemberIdentity(allocator: std.mem.Allocator, member: *MemberRef) !void {
    if (member.uid.len != 0) return;
    var channel = try grpc_client.connect(allocator, member.address);
    defer channel.deinit();

    const instance_ready = [_]i32{@intFromEnum(observability.EventType.instance_ready)};
    const request = try runtime.packEventsRequest(allocator, .{ .follow = false, .types = instance_ready[0..] });
    defer allocator.free(request);
    var stream = try channel.serverStream(allocator, "/holons.v1.HolonObservability/Events", request, RESOLVE_TIMEOUT_MS);
    defer stream.deinit();

    while (try stream.next(allocator)) |bytes| {
        defer allocator.free(bytes);
        var event = try runtime.unpackEventInfo(bytes);
        defer event.deinit(allocator);
        if (event.instanceUid().len == 0 or event.chainLen() != 0) continue;
        if (event.slug().len != 0) {
            allocator.free(member.slug);
            member.slug = try allocator.dupe(u8, std.mem.trim(u8, event.slug(), " \t\r\n"));
        }
        allocator.free(member.uid);
        member.uid = try allocator.dupe(u8, std.mem.trim(u8, event.instanceUid(), " \t\r\n"));
        return;
    }
}

fn emitResolvedMemberReady(allocator: std.mem.Allocator, obs: *observability.Observability, member: MemberRef) !void {
    if (!obs.enabled(.events) or obs.event_bus == null or member.uid.len == 0) return;
    var event: observability.Event = .{
        .timestamp_ns = nowNs(),
        .event_type = .instance_ready,
        .slug = try allocator.dupe(u8, member.slug),
        .instance_uid = try allocator.dupe(u8, member.uid),
        .chain = try allocator.alloc(observability.Hop, 1),
    };
    event.chain[0] = .{
        .slug = try allocator.dupe(u8, member.slug),
        .instance_uid = try allocator.dupe(u8, member.uid),
    };
    defer event.deinit(allocator);
    try obs.event_bus.?.emit(event);
}

fn pumpLogs(relay: *MemberRelay) void {
    while (!relay.stopping()) {
        pumpLogsOnce(relay) catch {};
        sleepBackoff(relay);
    }
}

fn pumpEvents(relay: *MemberRelay) void {
    while (!relay.stopping()) {
        pumpEventsOnce(relay) catch {};
        sleepBackoff(relay);
    }
}

fn pumpLogsOnce(relay: *MemberRelay) !void {
    var channel = try grpc_client.connect(relay.allocator, relay.member.address);
    defer channel.deinit();
    const request = try runtime.packLogsRequest(relay.allocator, .{ .follow = false });
    defer relay.allocator.free(request);
    var stream = try channel.serverStream(relay.allocator, "/holons.v1.HolonObservability/Logs", request, STREAM_TIMEOUT_MS);
    defer stream.deinit();

    while (!relay.stopping()) {
        const maybe_bytes = stream.next(relay.allocator) catch break;
        const bytes = maybe_bytes orelse break;
        defer relay.allocator.free(bytes);
        var entry = observability.logEntryFromBytes(relay.allocator, bytes) catch continue;
        defer entry.deinit(relay.allocator);
        try appendRelayHop(relay.allocator, &entry.chain, relay.member.slug, relay.member.uid);
        if (relay.obs.log_ring) |*ring| try ring.push(entry);
    }
}

fn pumpEventsOnce(relay: *MemberRelay) !void {
    emitResolvedMemberReady(relay.allocator, relay.obs, relay.member) catch {};
    var channel = try grpc_client.connect(relay.allocator, relay.member.address);
    defer channel.deinit();
    const request = try runtime.packEventsRequest(relay.allocator, .{ .follow = false });
    defer relay.allocator.free(request);
    var stream = try channel.serverStream(relay.allocator, "/holons.v1.HolonObservability/Events", request, STREAM_TIMEOUT_MS);
    defer stream.deinit();

    while (!relay.stopping()) {
        const maybe_bytes = stream.next(relay.allocator) catch break;
        const bytes = maybe_bytes orelse break;
        defer relay.allocator.free(bytes);
        var event = observability.eventFromBytes(relay.allocator, bytes) catch continue;
        defer event.deinit(relay.allocator);
        try appendRelayHop(relay.allocator, &event.chain, relay.member.slug, relay.member.uid);
        if (relay.obs.event_bus) |*bus| try bus.emit(event);
    }
}

fn appendRelayHop(
    allocator: std.mem.Allocator,
    chain: *[]observability.Hop,
    child_slug: []const u8,
    child_uid: []const u8,
) !void {
    const old = chain.*;
    const next = try observability.appendDirectChild(allocator, old, child_slug, child_uid);
    for (old) |*hop| hop.deinit(allocator);
    allocator.free(old);
    chain.* = next;
}

fn sleepBackoff(relay: *MemberRelay) void {
    var remaining: i64 = RETRY_BACKOFF_MS;
    while (remaining > 0 and !relay.stopping()) {
        const chunk = @min(remaining, 100);
        sleepMillis(chunk);
        remaining -= chunk;
    }
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(
        std.Io.Threaded.global_single_threaded.io(),
        std.Io.Duration.fromMilliseconds(ms),
        .awake,
    ) catch {};
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}
