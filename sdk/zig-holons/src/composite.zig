const std = @import("std");
const grpc_client = @import("grpc/client.zig");
const member_relay = @import("member_relay.zig");
const observability = @import("observability.zig");
const transport = @import("transport.zig");

const c = @cImport({
    @cInclude("fcntl.h");
    @cInclude("signal.h");
    @cInclude("stdlib.h");
    @cInclude("sys/wait.h");
    @cInclude("unistd.h");
});

pub const Error = error{
    ExecutablePathRequired,
    MemberDirectoryNotFound,
    MemberIdRequired,
    NoExecutableFound,
};

pub const TransportCoverageSequence = [_][]const u8{
    "stdio", "stdio", "tcp", "unix", "tcp", "tcp", "stdio", "unix", "unix", "stdio",
};

pub const ChildSpec = struct {
    slug: []const u8,
    binary: []const u8,
};

pub const EnvEntry = struct {
    key: []const u8,
    value: []const u8,
};

pub const DialOptions = struct {
    transitive_observability: ?bool = null,
};

pub fn WithTransitiveObservability(enabled: bool) DialOptions {
    return .{ .transitive_observability = enabled };
}

pub const SpawnOptions = struct {
    slug: []const u8,
    binary_path: []const u8,
    transport_name: []const u8 = "stdio",
    instance_uid: []const u8 = "",
    downstream_chain: []const ChildSpec = &.{},
    extra_env: []const EnvEntry = &.{},
    dial_options: DialOptions = .{},
};

pub const SpawnedMember = struct {
    allocator: std.mem.Allocator,
    slug: []u8,
    uid: []u8,
    listen_uri: []u8,
    conn: *grpc_client.Channel,
    process: c.pid_t = 0,
    relays: []member_relay.MemberRelay = &.{},
    stopped: bool = false,

    pub fn Stop(self: *SpawnedMember) void {
        if (self.stopped) return;
        self.stopped = true;
        // Teardown order is relay, channel, parent stdio fds/pumps, then child signal/wait.
        if (self.relays.len != 0) member_relay.stopAll(self.allocator, self.relays);
        self.conn.deinit();
        self.allocator.destroy(self.conn);
        if (self.process > 0) {
            terminateProcess(&self.process);
        }
        self.allocator.free(self.slug);
        self.allocator.free(self.uid);
        self.allocator.free(self.listen_uri);
    }
};

pub const CascadeOptions = struct {
    transport_name: []const u8,
    members: []const ChildSpec,
    extra_env: []const EnvEntry = &.{},
};

pub const Cascade = struct {
    top: SpawnedMember,

    pub fn Stop(self: *Cascade) void {
        self.top.Stop();
    }
};

pub fn SpawnMember(allocator: std.mem.Allocator, opts: SpawnOptions) !SpawnedMember {
    const slug = std.mem.trim(u8, opts.slug, " \t\r\n");
    const binary = std.mem.trim(u8, opts.binary_path, " \t\r\n");
    if (slug.len == 0) return error.MemberIdRequired;
    if (binary.len == 0) return error.ExecutablePathRequired;
    const transport_name = if (std.mem.trim(u8, opts.transport_name, " \t\r\n").len == 0) "stdio" else std.mem.trim(u8, opts.transport_name, " \t\r\n");
    const uid = if (std.mem.trim(u8, opts.instance_uid, " \t\r\n").len == 0)
        try newInstanceUid(allocator)
    else
        try allocator.dupe(u8, std.mem.trim(u8, opts.instance_uid, " \t\r\n"));
    errdefer allocator.free(uid);

    const listen_uri = try listenURIForSpawn(allocator, transport_name, uid);
    errdefer allocator.free(listen_uri);
    if (std.mem.startsWith(u8, listen_uri, "unix://")) {
        const socket_path = try allocator.dupeZ(u8, listen_uri["unix://".len..]);
        defer allocator.free(socket_path);
        _ = c.unlink(socket_path.ptr);
    }

    const run_root = try spawnRunRoot(allocator);
    defer allocator.free(run_root);
    try prepareSpawnEnv(allocator, uid, run_root, opts.extra_env);
    const argv = try childArgv(allocator, binary, listen_uri, transport_name, opts.downstream_chain);
    defer freeArgv(allocator, argv);

    const conn = try allocator.create(grpc_client.Channel);
    var conn_owned = true;
    errdefer if (conn_owned) allocator.destroy(conn);
    var conn_initialized = false;
    errdefer if (conn_initialized) conn.deinit();
    var process: c.pid_t = 0;
    var process_owned = false;
    errdefer if (process_owned and process > 0) terminateProcess(&process);
    var owned_listen = try allocator.dupe(u8, listen_uri);
    var owned_listen_owned = true;
    errdefer if (owned_listen_owned) allocator.free(owned_listen);

    if (std.mem.eql(u8, transport_name, "stdio")) {
        const cwd_z = try dirnameZ(allocator, binary);
        defer allocator.free(cwd_z);
        conn.* = try grpc_client.connectStdioCommand(allocator, .{ .argv = argv.items, .cwd = cwd_z });
        conn_initialized = true;
        try waitChannelReady(allocator, conn, 10_000);
        const stdio_listen = try allocator.dupe(u8, "stdio://");
        allocator.free(owned_listen);
        owned_listen = stdio_listen;
    } else {
        process = try spawnProcess(allocator, argv.items, binary);
        process_owned = true;
        const meta_address = try waitSpawnMeta(allocator, run_root, slug, uid, 10_000);
        defer allocator.free(meta_address);
        allocator.free(owned_listen);
        owned_listen = try allocator.dupe(u8, meta_address);
        conn.* = try dialReady(allocator, meta_address, 10_000);
        conn_initialized = true;
    }

    const spawned_slug = try allocator.dupe(u8, slug);
    var spawned_slug_owned = true;
    errdefer if (spawned_slug_owned) allocator.free(spawned_slug);
    const spawned_uid = try allocator.dupe(u8, uid);
    var spawned_uid_owned = true;
    errdefer if (spawned_uid_owned) allocator.free(spawned_uid);

    var spawned = SpawnedMember{
        .allocator = allocator,
        .slug = spawned_slug,
        .uid = spawned_uid,
        .listen_uri = owned_listen,
        .conn = conn,
        .process = process,
    };
    conn_initialized = false;
    conn_owned = false;
    process_owned = false;
    owned_listen_owned = false;
    spawned_slug_owned = false;
    spawned_uid_owned = false;
    errdefer spawned.Stop();

    const transitive = opts.dial_options.transitive_observability orelse true;
    if (transitive) {
        if (observability.current()) |obs| {
            spawned.relays = try member_relay.startForChannel(allocator, obs, spawned.slug, spawned.uid, spawned.conn);
        }
    }
    return spawned;
}

pub fn BuildCascade(allocator: std.mem.Allocator, opts: CascadeOptions) !Cascade {
    if (opts.members.len == 0) return error.MemberIdRequired;
    var top = try SpawnMember(allocator, .{
        .slug = opts.members[0].slug,
        .binary_path = opts.members[0].binary,
        .transport_name = opts.transport_name,
        .downstream_chain = opts.members[1..],
        .extra_env = opts.extra_env,
    });
    errdefer top.Stop();
    return .{ .top = top };
}

pub fn Dial(allocator: std.mem.Allocator, address: []const u8, opts: DialOptions) !*grpc_client.Channel {
    if (std.mem.startsWith(u8, std.mem.trim(u8, address, " \t\r\n"), "stdio://")) return error.UnsupportedListenTransport;
    const normalized = try normalizeDialAddress(allocator, address);
    defer allocator.free(normalized);
    const conn = try allocator.create(grpc_client.Channel);
    errdefer allocator.destroy(conn);
    conn.* = try dialReady(allocator, normalized, 10_000);
    const transitive = opts.transitive_observability orelse false;
    if (transitive) {
        if (observability.current()) |obs| {
            const identity = try resolveRelayIdentity(allocator, conn);
            defer {
                allocator.free(identity.slug);
                allocator.free(identity.uid);
            }
            _ = try member_relay.startForChannel(allocator, obs, identity.slug, identity.uid, conn);
        }
    }
    return conn;
}

pub const ChainHop = struct {
    slug: []const u8,
    instance_uid: []const u8,
};

pub const CheckOutcome = struct {
    pass: bool = false,
    evidence: []const u8 = "",
};

pub const LogCheckOptions = struct {
    sender: []const u8,
    leaf_uid: []const u8,
    expected_chain: []const ChainHop,
    timeout_ms: i64 = 3_000,
    poll_ms: i64 = 100,
    live: bool = false,
};

pub const EventCheckOptions = struct {
    event_type: observability.EventType,
    leaf_uid: []const u8,
    expected_chain: []const ChainHop,
    timeout_ms: i64 = 3_000,
    poll_ms: i64 = 100,
    live: bool = false,
};

pub fn CheckRelayedLog(allocator: std.mem.Allocator, opts: LogCheckOptions) CheckOutcome {
    _ = opts.live;
    const deadline = nowNs() + (@as(i128, opts.timeout_ms) * std.time.ns_per_ms);
    while (nowNs() < deadline) {
        if (observability.current()) |obs| {
            if (obs.log_ring) |*ring| {
                const entries = ring.drain(allocator) catch return .{ .evidence = "log drain failed" };
                defer freeLogs(allocator, entries);
                for (entries) |entry| {
                    if (!std.mem.eql(u8, entry.message, "tick received")) continue;
                    if (!std.mem.eql(u8, entry.instance_uid, opts.leaf_uid)) continue;
                    if (!hasLabel(entry.fields, "sender", opts.sender)) continue;
                    if (chainMatches(entry.chain, opts.expected_chain)) return .{ .pass = true, .evidence = "ok" };
                }
            }
        }
        sleepMillis(opts.poll_ms);
    }
    return .{ .evidence = "relayed log not observed" };
}

pub fn CheckRelayedEvent(allocator: std.mem.Allocator, opts: EventCheckOptions) CheckOutcome {
    _ = opts.live;
    const deadline = nowNs() + (@as(i128, opts.timeout_ms) * std.time.ns_per_ms);
    while (nowNs() < deadline) {
        if (observability.current()) |obs| {
            if (obs.event_bus) |*bus| {
                const events = bus.drain(allocator) catch return .{ .evidence = "event drain failed" };
                defer freeEvents(allocator, events);
                for (events) |event| {
                    if (event.event_type != opts.event_type) continue;
                    if (!std.mem.eql(u8, event.instance_uid, opts.leaf_uid)) continue;
                    if (chainMatches(event.chain, opts.expected_chain)) return .{ .pass = true, .evidence = "ok" };
                }
            }
        }
        sleepMillis(opts.poll_ms);
    }
    return .{ .evidence = "relayed event not observed" };
}

/// Resolves a declared member's primary binary relative to this composite's
/// own executable. The caller owns the returned path.
pub fn member(allocator: std.mem.Allocator, id: []const u8) ![]u8 {
    var buffer: [std.fs.max_path_bytes]u8 = undefined;
    const io = std.Io.Threaded.global_single_threaded.io();
    const len = try std.process.executablePath(io, &buffer);
    return memberFromExecutable(allocator, buffer[0..len], id);
}

/// Resolves a member binary relative to an explicit composite executable path.
/// The caller owns the returned path.
pub fn memberFromExecutable(allocator: std.mem.Allocator, executable: []const u8, id: []const u8) ![]u8 {
    const trimmed_id = std.mem.trim(u8, id, " \t\r\n");
    if (trimmed_id.len == 0) return error.MemberIdRequired;

    const absolute_executable = try absolutePath(allocator, executable);
    defer allocator.free(absolute_executable);

    const executable_dir = std.fs.path.dirname(absolute_executable) orelse return error.ExecutablePathRequired;
    const member_dir = try std.fs.path.join(allocator, &.{ executable_dir, "holons", trimmed_id });
    defer allocator.free(member_dir);

    const io = std.Io.Threaded.global_single_threaded.io();
    var dir = std.Io.Dir.cwd().openDir(io, member_dir, .{ .iterate = true }) catch |err| switch (err) {
        error.FileNotFound => return error.MemberDirectoryNotFound,
        else => |e| return e,
    };
    defer dir.close(io);

    var iter = dir.iterate();
    while (try iter.next(io)) |entry| {
        if (entry.kind != .file) continue;
        if (try isExecutableCandidate(dir, entry.name)) {
            return std.fs.path.join(allocator, &.{ member_dir, entry.name });
        }
    }
    return error.NoExecutableFound;
}

fn absolutePath(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    if (path.len == 0) return error.ExecutablePathRequired;
    if (std.fs.path.isAbsolute(path)) return allocator.dupe(u8, path);

    var cwd_buffer: [std.fs.max_path_bytes]u8 = undefined;
    const io = std.Io.Threaded.global_single_threaded.io();
    const len = try std.process.currentPath(io, &cwd_buffer);
    return std.fs.path.join(allocator, &.{ cwd_buffer[0..len], path });
}

fn isExecutableCandidate(dir: std.Io.Dir, name: []const u8) !bool {
    if (name.len == 0 or name[0] == '.') return false;
    if (std.ascii.endsWithIgnoreCase(name, ".exe")) return true;
    if (std.fs.path.extension(name).len != 0) return false;

    const io = std.Io.Threaded.global_single_threaded.io();
    const stat = dir.statFile(io, name, .{}) catch return false;
    const Permissions = @TypeOf(stat.permissions);
    if (comptime Permissions.has_executable_bit) {
        return stat.permissions.toMode() & 0o111 != 0;
    }
    return false;
}

fn childArgv(
    allocator: std.mem.Allocator,
    binary: []const u8,
    listen_uri: []const u8,
    transport_name: []const u8,
    downstream: []const ChildSpec,
) !std.ArrayList([:0]u8) {
    var argv: std.ArrayList([:0]u8) = .empty;
    errdefer freeArgv(allocator, argv);
    try argv.append(allocator, try allocator.dupeZ(u8, binary));
    try argv.append(allocator, try allocator.dupeZ(u8, "serve"));
    try argv.append(allocator, try allocator.dupeZ(u8, "--listen"));
    try argv.append(allocator, try allocator.dupeZ(u8, listen_uri));
    try argv.append(allocator, try allocator.dupeZ(u8, "--transport"));
    try argv.append(allocator, try allocator.dupeZ(u8, transport_name));
    for (downstream) |child| {
        try argv.append(allocator, try allocator.dupeZ(u8, "--child"));
        const spec = try std.fmt.allocPrint(allocator, "{s}={s}", .{ child.slug, child.binary });
        defer allocator.free(spec);
        try argv.append(allocator, try allocator.dupeZ(u8, spec));
    }
    return argv;
}

fn freeArgv(allocator: std.mem.Allocator, argv: std.ArrayList([:0]u8)) void {
    for (argv.items) |arg| allocator.free(arg);
    var copy = argv;
    copy.deinit(allocator);
}

fn spawnProcess(allocator: std.mem.Allocator, args: []const [:0]const u8, binary: []const u8) !c.pid_t {
    if (args.len == 0) return error.ExecutablePathRequired;
    const argv = try allocator.alloc(?[*:0]const u8, args.len + 1);
    defer allocator.free(argv);
    for (args, 0..) |arg, index| argv[index] = arg.ptr;
    argv[args.len] = null;

    const cwd_z = try dirnameZ(allocator, binary);
    defer allocator.free(cwd_z);

    const pid = c.fork();
    if (pid < 0) return error.ForkFailed;
    if (pid == 0) {
        _ = c.chdir(cwd_z.ptr);
        const dev_null = c.open("/dev/null", c.O_RDWR);
        if (dev_null >= 0) {
            _ = c.dup2(dev_null, 0);
            _ = c.dup2(dev_null, 1);
            _ = c.dup2(dev_null, 2);
            if (dev_null > 2) _ = c.close(dev_null);
        }
        closeInheritedFds();
        _ = c.execvp(argv[0].?, @ptrCast(argv.ptr));
        c._exit(127);
    }
    return pid;
}

fn dirnameZ(allocator: std.mem.Allocator, binary: []const u8) ![:0]u8 {
    const dir = std.fs.path.dirname(binary) orelse ".";
    return allocator.dupeZ(u8, dir);
}

fn listenURIForSpawn(allocator: std.mem.Allocator, transport_name: []const u8, uid: []const u8) ![]u8 {
    if (std.mem.eql(u8, transport_name, "stdio")) return allocator.dupe(u8, "stdio://");
    if (std.mem.eql(u8, transport_name, "tcp")) return allocator.dupe(u8, "tcp://127.0.0.1:0");
    if (std.mem.eql(u8, transport_name, "unix")) {
        const clean = try cleanSocketToken(allocator, uid);
        defer allocator.free(clean);
        return std.fmt.allocPrint(allocator, "unix://{s}/op-{s}.sock", .{ tmpDir(), clean });
    }
    return error.UnsupportedListenTransport;
}

fn cleanSocketToken(allocator: std.mem.Allocator, raw: []const u8) ![]u8 {
    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);
    for (raw) |ch| {
        if (std.ascii.isAlphanumeric(ch) or ch == '-' or ch == '_') {
            try out.append(allocator, ch);
        }
    }
    if (out.items.len == 0) try out.appendSlice(allocator, "zig");
    return out.toOwnedSlice(allocator);
}

fn tmpDir() []const u8 {
    return getenv("TMPDIR") orelse "/tmp";
}

fn closeInheritedFds() void {
    var fd: c_int = 3;
    while (fd < 4096) : (fd += 1) {
        _ = c.close(fd);
    }
}

fn prepareSpawnEnv(allocator: std.mem.Allocator, uid: []const u8, run_root: []const u8, extra: []const EnvEntry) !void {
    try setEnv("OP_INSTANCE_UID", uid);
    try setEnv("OP_RUN_DIR", run_root);
    const parent_pid = try std.fmt.allocPrint(allocator, "{d}", .{c.getpid()});
    defer allocator.free(parent_pid);
    try setEnv("HOLONS_PARENT_PID", parent_pid);
    if (activeObservabilityFamilies(allocator)) |families| {
        defer allocator.free(families);
        if (families.len != 0) try setEnv("OP_OBS", families);
    } else |_| {}
    for (extra) |entry| try setEnv(entry.key, entry.value);
}

fn activeObservabilityFamilies(allocator: std.mem.Allocator) ![]u8 {
    const obs = observability.current() orelse return allocator.dupe(u8, getenv("OP_OBS") orelse "");
    var parts: std.ArrayList([]const u8) = .empty;
    defer parts.deinit(allocator);
    if (obs.enabled(.logs)) try parts.append(allocator, "logs");
    if (obs.enabled(.metrics)) try parts.append(allocator, "metrics");
    if (obs.enabled(.events)) try parts.append(allocator, "events");
    if (obs.enabled(.prom)) try parts.append(allocator, "prom");
    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);
    for (parts.items, 0..) |part, index| {
        if (index != 0) try out.append(allocator, ',');
        try out.appendSlice(allocator, part);
    }
    return out.toOwnedSlice(allocator);
}

fn spawnRunRoot(allocator: std.mem.Allocator) ![]u8 {
    if (getenv("OP_RUN_DIR")) |value| {
        if (std.mem.trim(u8, value, " \t\r\n").len != 0) return allocator.dupe(u8, value);
    }
    if (getenv("OPPATH")) |value| {
        if (std.mem.trim(u8, value, " \t\r\n").len != 0) return std.fs.path.join(allocator, &.{ value, "run" });
    }
    if (getenv("HOME")) |home| return std.fs.path.join(allocator, &.{ home, ".op", "run" });
    return std.fs.path.join(allocator, &.{ tmpDir(), ".op", "run" });
}

fn waitSpawnMeta(allocator: std.mem.Allocator, run_root: []const u8, slug: []const u8, uid: []const u8, timeout_ms: i64) ![]u8 {
    const dir = try std.fs.path.join(allocator, &.{ run_root, slug, uid });
    defer allocator.free(dir);
    const path = try std.fs.path.join(allocator, &.{ dir, "meta.json" });
    defer allocator.free(path);
    const deadline = nowNs() + (@as(i128, timeout_ms) * std.time.ns_per_ms);
    while (nowNs() < deadline) {
        const body = std.Io.Dir.cwd().readFileAlloc(std.Options.debug_io, path, allocator, .limited(1024 * 1024)) catch {
            sleepMillis(50);
            continue;
        };
        defer allocator.free(body);
        if (std.mem.indexOf(u8, body, uid) != null) {
            if (jsonString(allocator, body, "address")) |address| {
                if (address.len != 0) return address;
                allocator.free(address);
            } else |_| {}
            if (jsonString(allocator, body, "listener")) |listener| {
                if (listener.len != 0) return listener;
                allocator.free(listener);
            } else |_| {}
        }
        sleepMillis(50);
    }
    return error.MetaTimeout;
}

fn dialReady(allocator: std.mem.Allocator, raw_uri: []const u8, timeout_ms: i64) !grpc_client.Channel {
    const normalized = try normalizeDialAddress(allocator, raw_uri);
    defer allocator.free(normalized);
    const deadline = nowNs() + (@as(i128, timeout_ms) * std.time.ns_per_ms);
    var last: anyerror = error.DialTimeout;
    while (nowNs() < deadline) {
        var channel = grpc_client.connect(allocator, normalized) catch |err| {
            last = err;
            sleepMillis(50);
            continue;
        };
        if (channel.describe(allocator)) |desc| {
            var d = desc;
            d.deinit();
            return channel;
        } else |err| {
            last = err;
            channel.deinit();
            sleepMillis(50);
        }
    }
    return last;
}

fn waitChannelReady(allocator: std.mem.Allocator, channel: *grpc_client.Channel, timeout_ms: i64) !void {
    const deadline = nowNs() + (@as(i128, timeout_ms) * std.time.ns_per_ms);
    var last: anyerror = error.DialTimeout;
    while (nowNs() < deadline) {
        if (channel.describe(allocator)) |desc| {
            var d = desc;
            d.deinit();
            return;
        } else |err| {
            last = err;
            sleepMillis(50);
        }
    }
    return last;
}

fn normalizeDialAddress(allocator: std.mem.Allocator, raw: []const u8) ![]u8 {
    const trimmed = std.mem.trim(u8, raw, " \t\r\n");
    if (trimmed.len == 0) return error.EmptyUri;
    if (std.mem.startsWith(u8, trimmed, "tcp://")) {
        const address = trimmed["tcp://".len..];
        if (std.mem.startsWith(u8, address, "0.0.0.0:")) {
            return std.fmt.allocPrint(allocator, "tcp://127.0.0.1:{s}", .{address["0.0.0.0:".len..]});
        }
        return allocator.dupe(u8, trimmed);
    }
    if (std.mem.startsWith(u8, trimmed, "unix://")) return allocator.dupe(u8, trimmed);
    if (std.mem.indexOf(u8, trimmed, "://") != null) return error.UnsupportedScheme;
    if (std.mem.indexOfScalar(u8, trimmed, ':') != null) return std.fmt.allocPrint(allocator, "tcp://{s}", .{trimmed});
    return error.MissingScheme;
}

const RelayIdentity = struct {
    slug: []u8,
    uid: []u8,
};

fn resolveRelayIdentity(allocator: std.mem.Allocator, conn: *grpc_client.Channel) !RelayIdentity {
    const instance_ready = [_]i32{@intFromEnum(observability.EventType.instance_ready)};
    const request = try @import("protobuf/runtime.zig").packEventsRequest(allocator, .{ .follow = false, .types = instance_ready[0..] });
    defer allocator.free(request);
    var stream = try conn.serverStream(allocator, "/holons.v1.HolonObservability/Events", request, 1_000);
    defer stream.deinit();
    while (try stream.next(allocator)) |bytes| {
        defer allocator.free(bytes);
        var event = try observability.eventFromBytes(allocator, bytes);
        defer event.deinit(allocator);
        if (event.chain.len != 0 or event.instance_uid.len == 0) continue;
        return .{
            .slug = try allocator.dupe(u8, event.slug),
            .uid = try allocator.dupe(u8, event.instance_uid),
        };
    }
    return error.IdentityNotResolved;
}

fn hasLabel(labels: []const observability.Field, key: []const u8, value: []const u8) bool {
    for (labels) |label| {
        if (std.mem.eql(u8, label.key, key) and label.value == .string_value and std.mem.eql(u8, label.value.string_value, value)) return true;
    }
    return false;
}

fn chainMatches(got: []const observability.Hop, want: []const ChainHop) bool {
    if (got.len != want.len) return false;
    for (want, 0..) |hop, index| {
        if (!std.mem.eql(u8, got[index].slug, hop.slug)) return false;
        if (!std.mem.eql(u8, got[index].instance_uid, hop.instance_uid)) return false;
    }
    return true;
}

fn freeLogs(allocator: std.mem.Allocator, entries: []observability.LogRecord) void {
    for (entries) |*entry| entry.deinit(allocator);
    allocator.free(entries);
}

fn freeEvents(allocator: std.mem.Allocator, entries: []observability.Event) void {
    for (entries) |*event| event.deinit(allocator);
    allocator.free(entries);
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
    const start = cursor + 1;
    const rel_end = std.mem.indexOfScalar(u8, body[start..], '"') orelse return error.JsonStringEndMissing;
    return allocator.dupe(u8, body[start .. start + rel_end]);
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

fn newInstanceUid(allocator: std.mem.Allocator) ![]u8 {
    return std.fmt.allocPrint(allocator, "zig-{d}", .{nowNs()});
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}

fn terminateProcess(pid: *c.pid_t) void {
    var status: c_int = 0;
    if (c.waitpid(pid.*, &status, c.WNOHANG) == pid.*) {
        pid.* = 0;
        return;
    }
    _ = c.kill(pid.*, c.SIGTERM);
    const deadline_ns = nowNs() + (3_000 * std.time.ns_per_ms);
    while (nowNs() < deadline_ns) {
        if (c.waitpid(pid.*, &status, c.WNOHANG) == pid.*) {
            pid.* = 0;
            return;
        }
        sleepMillis(50);
    }
    _ = c.kill(pid.*, c.SIGKILL);
    _ = c.waitpid(pid.*, &status, 0);
    pid.* = 0;
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(
        std.Io.Threaded.global_single_threaded.io(),
        std.Io.Duration.fromMilliseconds(@intCast(ms)),
        .awake,
    ) catch {};
}

test "memberFromExecutable resolves embedded member binary" {
    const allocator = std.testing.allocator;
    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const rel_root = try std.fs.path.join(allocator, &.{ ".zig-cache", "tmp", tmp.sub_path[0..] });
    defer allocator.free(rel_root);

    const member_dir = try std.fs.path.join(allocator, &.{ rel_root, "observability-cascade-zig.holon", "bin", "darwin_arm64", "holons", "zig-node" });
    defer allocator.free(member_dir);
    try std.Io.Dir.cwd().createDirPath(std.testing.io, member_dir);

    const self = try std.fs.path.join(allocator, &.{ rel_root, "observability-cascade-zig.holon", "bin", "darwin_arm64", "observability-cascade-zig" });
    defer allocator.free(self);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{
        .sub_path = self,
        .data = "composite",
        .flags = .{ .permissions = .executable_file },
    });

    const readme = try std.fs.path.join(allocator, &.{ member_dir, "README.txt" });
    defer allocator.free(readme);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{ .sub_path = readme, .data = "not executable" });

    const member_bin = try std.fs.path.join(allocator, &.{ member_dir, "observability-cascade-zig-node" });
    defer allocator.free(member_bin);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{
        .sub_path = member_bin,
        .data = "member",
        .flags = .{ .permissions = .executable_file },
    });

    const got = try memberFromExecutable(allocator, self, "zig-node");
    defer allocator.free(got);

    var cwd_buffer: [std.fs.max_path_bytes]u8 = undefined;
    const cwd_len = try std.process.currentPath(std.testing.io, &cwd_buffer);
    const want = try std.fs.path.join(allocator, &.{ cwd_buffer[0..cwd_len], member_bin });
    defer allocator.free(want);
    try std.testing.expectEqualStrings(want, got);
}

test "memberFromExecutable rejects empty member id" {
    try std.testing.expectError(error.MemberIdRequired, memberFromExecutable(std.testing.allocator, "/tmp/composite", " "));
}

test "memberFromExecutable reports missing member executable" {
    const allocator = std.testing.allocator;
    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const rel_root = try std.fs.path.join(allocator, &.{ ".zig-cache", "tmp", tmp.sub_path[0..] });
    defer allocator.free(rel_root);

    const member_dir = try std.fs.path.join(allocator, &.{ rel_root, "composite.holon", "bin", "darwin_arm64", "holons", "node-a" });
    defer allocator.free(member_dir);
    try std.Io.Dir.cwd().createDirPath(std.testing.io, member_dir);

    const self = try std.fs.path.join(allocator, &.{ rel_root, "composite.holon", "bin", "darwin_arm64", "composite" });
    defer allocator.free(self);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{
        .sub_path = self,
        .data = "composite",
        .flags = .{ .permissions = .executable_file },
    });

    const ignored = try std.fs.path.join(allocator, &.{ member_dir, "member.txt" });
    defer allocator.free(ignored);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{ .sub_path = ignored, .data = "not executable" });

    try std.testing.expectError(error.NoExecutableFound, memberFromExecutable(allocator, self, "node-a"));
}
