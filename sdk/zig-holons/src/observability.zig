const std = @import("std");
const grpc_server = @import("grpc/server.zig");
const runtime = @import("protobuf/runtime.zig");

const c = runtime.c;

const net_c = @cImport({
    @cInclude("arpa/inet.h");
    @cInclude("netinet/in.h");
    @cInclude("sys/socket.h");
    @cInclude("unistd.h");
});

pub const Family = enum {
    logs,
    metrics,
    events,
    prom,
    otel,

    pub fn asString(self: Family) []const u8 {
        return switch (self) {
            .logs => "logs",
            .metrics => "metrics",
            .events => "events",
            .prom => "prom",
            .otel => "otel",
        };
    }
};

pub const FamilySet = struct {
    logs: bool = false,
    metrics: bool = false,
    events: bool = false,
    prom: bool = false,

    pub fn contains(self: FamilySet, family: Family) bool {
        return switch (family) {
            .logs => self.logs,
            .metrics => self.metrics,
            .events => self.events,
            .prom => self.prom,
            .otel => false,
        };
    }

    pub fn enable(self: *FamilySet, family: Family) void {
        switch (family) {
            .logs => self.logs = true,
            .metrics => self.metrics = true,
            .events => self.events = true,
            .prom => self.prom = true,
            .otel => {},
        }
    }

    pub fn count(self: FamilySet) usize {
        return @intFromBool(self.logs) + @intFromBool(self.metrics) + @intFromBool(self.events) + @intFromBool(self.prom);
    }
};

pub const InvalidToken = struct {
    variable: []const u8,
    token: []const u8,
    reason: []const u8,
};

pub const Error = error{
    UnknownObservabilityToken,
    ReservedObservabilityToken,
    IoError,
};

const V1_TOKENS = [_][]const u8{ "logs", "metrics", "events", "prom", "all" };

pub fn parseOpObs(raw: []const u8) Error!FamilySet {
    var out: FamilySet = .{};
    const trimmed = std.mem.trim(u8, raw, " \t\r\n");
    if (trimmed.len == 0) return out;
    var it = std.mem.splitScalar(u8, trimmed, ',');
    while (it.next()) |part| {
        const token = std.mem.trim(u8, part, " \t\r\n");
        if (token.len == 0) continue;
        if (std.mem.eql(u8, token, "otel") or std.mem.eql(u8, token, "sessions")) {
            return error.ReservedObservabilityToken;
        }
        if (!knownToken(token)) return error.UnknownObservabilityToken;
        if (std.mem.eql(u8, token, "all")) {
            out.logs = true;
            out.metrics = true;
            out.events = true;
            out.prom = true;
        } else if (familyFromString(token)) |family| {
            out.enable(family);
        }
    }
    return out;
}

pub const EnvEntry = struct {
    key: []const u8,
    value: []const u8,
};

pub fn checkEnvFrom(env: []const EnvEntry) Error!void {
    const sessions = envValue(env, "OP_SESSIONS") orelse "";
    if (std.mem.trim(u8, sessions, " \t\r\n").len != 0) return error.ReservedObservabilityToken;
    const raw = envValue(env, "OP_OBS") orelse "";
    _ = try parseOpObs(raw);
}

pub fn checkEnv(allocator: std.mem.Allocator) !void {
    const sessions = try processEnvOwned(allocator, "OP_SESSIONS");
    defer if (sessions) |value| allocator.free(value);
    if (std.mem.trim(u8, sessions orelse "", " \t\r\n").len != 0) return error.ReservedObservabilityToken;
    const op_obs = try processEnvOwned(allocator, "OP_OBS");
    defer if (op_obs) |value| allocator.free(value);
    _ = try parseOpObs(op_obs orelse "");
}

pub const Level = enum(i32) {
    unset = 0,
    trace = 1,
    debug = 5,
    info = 9,
    warn = 13,
    err = 17,
    fatal = 21,

    pub fn name(self: Level) []const u8 {
        return switch (self) {
            .trace => "TRACE",
            .debug => "DEBUG",
            .info => "INFO",
            .warn => "WARN",
            .err => "ERROR",
            .fatal => "FATAL",
            .unset => "UNSPECIFIED",
        };
    }
};

pub fn parseLevel(value: []const u8) Level {
    if (std.ascii.eqlIgnoreCase(value, "TRACE")) return .trace;
    if (std.ascii.eqlIgnoreCase(value, "DEBUG")) return .debug;
    if (std.ascii.eqlIgnoreCase(value, "INFO")) return .info;
    if (std.ascii.eqlIgnoreCase(value, "WARN") or std.ascii.eqlIgnoreCase(value, "WARNING")) return .warn;
    if (std.ascii.eqlIgnoreCase(value, "ERROR")) return .err;
    if (std.ascii.eqlIgnoreCase(value, "FATAL")) return .fatal;
    return .info;
}

pub const EventType = enum(i32) {
    unspecified = 0,
    instance_spawned = 1,
    instance_ready = 2,
    instance_exited = 3,
    instance_crashed = 4,
    session_started = 5,
    session_ended = 6,
    handler_panic = 7,
    config_reloaded = 8,

    pub fn name(self: EventType) []const u8 {
        return switch (self) {
            .instance_spawned => "INSTANCE_SPAWNED",
            .instance_ready => "INSTANCE_READY",
            .instance_exited => "INSTANCE_EXITED",
            .instance_crashed => "INSTANCE_CRASHED",
            .session_started => "SESSION_STARTED",
            .session_ended => "SESSION_ENDED",
            .handler_panic => "HANDLER_PANIC",
            .config_reloaded => "CONFIG_RELOADED",
            .unspecified => "UNSPECIFIED",
        };
    }

    pub fn eventName(self: EventType) []const u8 {
        return switch (self) {
            .instance_spawned => "instance.spawned",
            .instance_ready => "instance.ready",
            .instance_exited => "instance.exited",
            .instance_crashed => "instance.crashed",
            .session_started => "session.started",
            .session_ended => "session.ended",
            .handler_panic => "handler.panic",
            .config_reloaded => "config.reloaded",
            .unspecified => "event.unspecified",
        };
    }
};

pub const Label = struct {
    key: []const u8,
    value: []const u8,
};

pub const AnyValue = union(enum) {
    string_value: []const u8,
    bool_value: bool,
    int_value: i64,
    double_value: f64,

    pub fn clone(allocator: std.mem.Allocator, value: AnyValue) !AnyValue {
        return switch (value) {
            .string_value => |v| .{ .string_value = try allocator.dupe(u8, v) },
            .bool_value => |v| .{ .bool_value = v },
            .int_value => |v| .{ .int_value = v },
            .double_value => |v| .{ .double_value = v },
        };
    }

    pub fn deinit(self: *AnyValue, allocator: std.mem.Allocator) void {
        switch (self.*) {
            .string_value => |value| allocator.free(value),
            else => {},
        }
        self.* = .{ .string_value = "" };
    }
};

pub const Field = struct {
    key: []const u8,
    value: AnyValue,

    pub fn string(key: []const u8, value: []const u8) Field {
        return .{ .key = key, .value = .{ .string_value = value } };
    }

    pub fn boolean(key: []const u8, value: bool) Field {
        return .{ .key = key, .value = .{ .bool_value = value } };
    }

    pub fn int(key: []const u8, value: i64) Field {
        return .{ .key = key, .value = .{ .int_value = value } };
    }

    pub fn double(key: []const u8, value: f64) Field {
        return .{ .key = key, .value = .{ .double_value = value } };
    }
};

pub const Hop = struct {
    slug: []const u8,
    instance_uid: []const u8,

    pub fn deinit(self: *Hop, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.instance_uid);
        self.* = .{ .slug = "", .instance_uid = "" };
    }
};

pub fn appendDirectChild(allocator: std.mem.Allocator, src: []const Hop, child_slug: []const u8, child_uid: []const u8) ![]Hop {
    var out = try allocator.alloc(Hop, src.len + 1);
    errdefer allocator.free(out);
    for (src, 0..) |hop, index| {
        out[index] = .{
            .slug = try allocator.dupe(u8, hop.slug),
            .instance_uid = try allocator.dupe(u8, hop.instance_uid),
        };
    }
    out[src.len] = .{
        .slug = try allocator.dupe(u8, child_slug),
        .instance_uid = try allocator.dupe(u8, child_uid),
    };
    return out;
}

pub fn enrichForMultilog(allocator: std.mem.Allocator, wire: []const Hop, source_slug: []const u8, source_uid: []const u8) ![]Hop {
    return appendDirectChild(allocator, wire, source_slug, source_uid);
}

pub const LogRecord = struct {
    sequence: u64 = 0,
    timestamp_ns: i128 = 0,
    level: Level = .info,
    slug: []const u8 = "",
    instance_uid: []const u8 = "",
    session_id: []const u8 = "",
    rpc_method: []const u8 = "",
    message: []const u8 = "",
    fields: []Field = &.{},
    caller: []const u8 = "",
    chain: []Hop = &.{},

    pub fn clone(allocator: std.mem.Allocator, entry: LogRecord) !LogRecord {
        return .{
            .sequence = entry.sequence,
            .timestamp_ns = entry.timestamp_ns,
            .level = entry.level,
            .slug = try allocator.dupe(u8, entry.slug),
            .instance_uid = try allocator.dupe(u8, entry.instance_uid),
            .session_id = try allocator.dupe(u8, entry.session_id),
            .rpc_method = try allocator.dupe(u8, entry.rpc_method),
            .message = try allocator.dupe(u8, entry.message),
            .fields = try cloneFields(allocator, entry.fields),
            .caller = try allocator.dupe(u8, entry.caller),
            .chain = try cloneHops(allocator, entry.chain),
        };
    }

    pub fn deinit(self: *LogRecord, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.instance_uid);
        allocator.free(self.session_id);
        allocator.free(self.rpc_method);
        allocator.free(self.message);
        freeFields(allocator, self.fields);
        allocator.free(self.caller);
        freeHops(allocator, self.chain);
        self.* = .{};
    }
};

pub const LogRing = struct {
    allocator: std.mem.Allocator,
    capacity: usize,
    inner: std.ArrayList(LogRecord),
    next_sequence: u64 = 1,
    mutex: std.Io.Mutex = .init,

    pub fn init(allocator: std.mem.Allocator, capacity: usize) LogRing {
        return .{ .allocator = allocator, .capacity = @max(capacity, 1), .inner = .empty };
    }

    pub fn deinit(self: *LogRing) void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        for (self.inner.items) |*entry| entry.deinit(self.allocator);
        self.inner.deinit(self.allocator);
    }

    pub fn push(self: *LogRing, entry: LogRecord) !void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        if (self.inner.items.len == self.capacity) {
            var old = self.inner.orderedRemove(0);
            old.deinit(self.allocator);
        }
        var cloned = try LogRecord.clone(self.allocator, entry);
        cloned.sequence = self.next_sequence;
        self.next_sequence += 1;
        try self.inner.append(self.allocator, cloned);
    }

    pub fn drain(self: *LogRing, allocator: std.mem.Allocator) ![]LogRecord {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        var out = try allocator.alloc(LogRecord, self.inner.items.len);
        errdefer allocator.free(out);
        for (self.inner.items, 0..) |entry, index| out[index] = try LogRecord.clone(allocator, entry);
        return out;
    }

    pub fn drainSince(self: *LogRing, allocator: std.mem.Allocator, cutoff_ns: i128) ![]LogRecord {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        var out: std.ArrayList(LogRecord) = .empty;
        errdefer {
            for (out.items) |*entry| entry.deinit(allocator);
            out.deinit(allocator);
        }
        for (self.inner.items) |entry| {
            if (entry.timestamp_ns >= cutoff_ns) try out.append(allocator, try LogRecord.clone(allocator, entry));
        }
        return out.toOwnedSlice(allocator);
    }

    pub fn len(self: *LogRing) usize {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        return self.inner.items.len;
    }
};

pub const Event = struct {
    sequence: u64 = 0,
    timestamp_ns: i128 = 0,
    event_type: EventType = .unspecified,
    slug: []const u8 = "",
    instance_uid: []const u8 = "",
    session_id: []const u8 = "",
    payload: []Field = &.{},
    chain: []Hop = &.{},

    pub fn clone(allocator: std.mem.Allocator, event: Event) !Event {
        return .{
            .sequence = event.sequence,
            .timestamp_ns = event.timestamp_ns,
            .event_type = event.event_type,
            .slug = try allocator.dupe(u8, event.slug),
            .instance_uid = try allocator.dupe(u8, event.instance_uid),
            .session_id = try allocator.dupe(u8, event.session_id),
            .payload = try cloneFields(allocator, event.payload),
            .chain = try cloneHops(allocator, event.chain),
        };
    }

    pub fn deinit(self: *Event, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.instance_uid);
        allocator.free(self.session_id);
        freeFields(allocator, self.payload);
        freeHops(allocator, self.chain);
        self.* = .{};
    }
};

pub const EventBus = struct {
    allocator: std.mem.Allocator,
    capacity: usize,
    inner: std.ArrayList(Event),
    next_sequence: u64 = 1,
    closed: bool = false,
    mutex: std.Io.Mutex = .init,

    pub fn init(allocator: std.mem.Allocator, capacity: usize) EventBus {
        return .{ .allocator = allocator, .capacity = @max(capacity, 1), .inner = .empty };
    }

    pub fn deinit(self: *EventBus) void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        for (self.inner.items) |*event| event.deinit(self.allocator);
        self.inner.deinit(self.allocator);
    }

    pub fn emit(self: *EventBus, event: Event) !void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        if (self.closed) return;
        if (self.inner.items.len == self.capacity) {
            var old = self.inner.orderedRemove(0);
            old.deinit(self.allocator);
        }
        var cloned = try Event.clone(self.allocator, event);
        cloned.sequence = self.next_sequence;
        self.next_sequence += 1;
        try self.inner.append(self.allocator, cloned);
    }

    pub fn drain(self: *EventBus, allocator: std.mem.Allocator) ![]Event {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        var out = try allocator.alloc(Event, self.inner.items.len);
        errdefer allocator.free(out);
        for (self.inner.items, 0..) |event, index| out[index] = try Event.clone(allocator, event);
        return out;
    }

    pub fn drainSince(self: *EventBus, allocator: std.mem.Allocator, cutoff_ns: i128) ![]Event {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        var out: std.ArrayList(Event) = .empty;
        errdefer {
            for (out.items) |*event| event.deinit(allocator);
            out.deinit(allocator);
        }
        for (self.inner.items) |event| {
            if (event.timestamp_ns >= cutoff_ns) try out.append(allocator, try Event.clone(allocator, event));
        }
        return out.toOwnedSlice(allocator);
    }

    pub fn close(self: *EventBus) void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        self.closed = true;
    }
};

pub const Counter = struct {
    name: []const u8,
    help: []const u8,
    labels: []Label,
    value: i64 = 0,
    mutex: std.Io.Mutex = .init,

    pub fn inc(self: *Counter) void {
        self.add(1);
    }

    pub fn add(self: *Counter, n: i64) void {
        if (n < 0) return;
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        self.value += n;
    }

    pub fn read(self: *Counter) i64 {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        return self.value;
    }

    fn deinit(self: *Counter, allocator: std.mem.Allocator) void {
        allocator.free(self.name);
        allocator.free(self.help);
        freeLabels(allocator, self.labels);
        self.* = undefined;
    }
};

pub const Gauge = struct {
    name: []const u8,
    help: []const u8,
    labels: []Label,
    value: f64 = 0,
    mutex: std.Io.Mutex = .init,

    pub fn set(self: *Gauge, value: f64) void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        self.value = value;
    }

    pub fn add(self: *Gauge, delta: f64) void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        self.value += delta;
    }

    pub fn read(self: *Gauge) f64 {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        return self.value;
    }

    fn deinit(self: *Gauge, allocator: std.mem.Allocator) void {
        allocator.free(self.name);
        allocator.free(self.help);
        freeLabels(allocator, self.labels);
        self.* = undefined;
    }
};

pub const DEFAULT_BUCKETS = [_]f64{
    50e-6,  100e-6, 250e-6, 500e-6, 1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3,
    100e-3, 250e-3, 500e-3, 1.0,    2.5,  5.0,    10.0, 30.0,  60.0,
};

pub const HistogramSnapshot = struct {
    bounds: []f64,
    counts: []i64,
    total: i64,
    sum: f64,

    pub fn deinit(self: *HistogramSnapshot, allocator: std.mem.Allocator) void {
        allocator.free(self.bounds);
        allocator.free(self.counts);
        self.* = .{ .bounds = &.{}, .counts = &.{}, .total = 0, .sum = 0 };
    }

    pub fn quantile(self: HistogramSnapshot, q: f64) f64 {
        if (self.total == 0) return std.math.nan(f64);
        const target = @as(f64, @floatFromInt(self.total)) * q;
        for (self.counts, 0..) |count, index| {
            if (@as(f64, @floatFromInt(count)) >= target) return self.bounds[index];
        }
        return std.math.inf(f64);
    }
};

pub const Histogram = struct {
    name: []const u8,
    help: []const u8,
    labels: []Label,
    bounds: []f64,
    counts: []i64,
    total: i64 = 0,
    sum: f64 = 0,
    mutex: std.Io.Mutex = .init,

    pub fn observe(self: *Histogram, value: f64) void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        self.total += 1;
        self.sum += value;
        for (self.bounds, 0..) |bound, index| {
            if (value <= bound) self.counts[index] += 1;
        }
    }

    pub fn snapshot(self: *Histogram, allocator: std.mem.Allocator) !HistogramSnapshot {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        return .{
            .bounds = try allocator.dupe(f64, self.bounds),
            .counts = try allocator.dupe(i64, self.counts),
            .total = self.total,
            .sum = self.sum,
        };
    }

    fn deinit(self: *Histogram, allocator: std.mem.Allocator) void {
        allocator.free(self.name);
        allocator.free(self.help);
        freeLabels(allocator, self.labels);
        allocator.free(self.bounds);
        allocator.free(self.counts);
        self.* = undefined;
    }
};

pub const Registry = struct {
    allocator: std.mem.Allocator,
    counters: std.ArrayList(Counter) = .empty,
    gauges: std.ArrayList(Gauge) = .empty,
    histograms: std.ArrayList(Histogram) = .empty,
    mutex: std.Io.Mutex = .init,

    pub fn init(allocator: std.mem.Allocator) Registry {
        return .{ .allocator = allocator };
    }

    pub fn deinit(self: *Registry) void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        for (self.counters.items) |*counter_item| counter_item.deinit(self.allocator);
        self.counters.deinit(self.allocator);
        for (self.gauges.items) |*gauge_item| gauge_item.deinit(self.allocator);
        self.gauges.deinit(self.allocator);
        for (self.histograms.items) |*histogram_item| histogram_item.deinit(self.allocator);
        self.histograms.deinit(self.allocator);
    }

    pub fn counter(self: *Registry, name: []const u8, help: []const u8, labels: []const Label) !*Counter {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        for (self.counters.items) |*existing| if (metricMatches(existing.name, existing.labels, name, labels)) return existing;
        try self.counters.append(self.allocator, .{
            .name = try self.allocator.dupe(u8, name),
            .help = try self.allocator.dupe(u8, help),
            .labels = try cloneLabels(self.allocator, labels),
        });
        return &self.counters.items[self.counters.items.len - 1];
    }

    pub fn gauge(self: *Registry, name: []const u8, help: []const u8, labels: []const Label) !*Gauge {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        for (self.gauges.items) |*existing| if (metricMatches(existing.name, existing.labels, name, labels)) return existing;
        try self.gauges.append(self.allocator, .{
            .name = try self.allocator.dupe(u8, name),
            .help = try self.allocator.dupe(u8, help),
            .labels = try cloneLabels(self.allocator, labels),
        });
        return &self.gauges.items[self.gauges.items.len - 1];
    }

    pub fn histogram(self: *Registry, name: []const u8, help: []const u8, labels: []const Label, bounds: ?[]const f64) !*Histogram {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        for (self.histograms.items) |*existing| if (metricMatches(existing.name, existing.labels, name, labels)) return existing;
        const source_bounds = bounds orelse DEFAULT_BUCKETS[0..];
        const owned_bounds = try self.allocator.dupe(f64, source_bounds);
        std.mem.sort(f64, owned_bounds, {}, struct {
            fn less(_: void, a: f64, b: f64) bool {
                return a < b;
            }
        }.less);
        try self.histograms.append(self.allocator, .{
            .name = try self.allocator.dupe(u8, name),
            .help = try self.allocator.dupe(u8, help),
            .labels = try cloneLabels(self.allocator, labels),
            .bounds = owned_bounds,
            .counts = try self.allocator.alloc(i64, owned_bounds.len),
        });
        @memset(self.histograms.items[self.histograms.items.len - 1].counts, 0);
        return &self.histograms.items[self.histograms.items.len - 1];
    }
};

pub const Config = struct {
    slug: []const u8 = "",
    default_log_level: ?Level = null,
    prom_addr: []const u8 = "",
    redacted_fields: []const []const u8 = &.{},
    logs_ring_size: usize = 0,
    events_ring_size: usize = 0,
    run_dir: []const u8 = "",
    instance_uid: []const u8 = "",
    organism_uid: []const u8 = "",
    organism_slug: []const u8 = "",
};

const OwnedConfig = struct {
    slug: []const u8 = "",
    default_log_level: ?Level = null,
    prom_addr: []const u8 = "",
    redacted_fields: []const []const u8 = &.{},
    logs_ring_size: usize = 0,
    events_ring_size: usize = 0,
    run_dir: []const u8 = "",
    instance_uid: []const u8 = "",
    organism_uid: []const u8 = "",
    organism_slug: []const u8 = "",

    fn fromConfig(allocator: std.mem.Allocator, cfg: Config) !OwnedConfig {
        return .{
            .slug = try allocator.dupe(u8, cfg.slug),
            .default_log_level = cfg.default_log_level,
            .prom_addr = try allocator.dupe(u8, cfg.prom_addr),
            .redacted_fields = try dupeStringSlice(allocator, cfg.redacted_fields),
            .logs_ring_size = cfg.logs_ring_size,
            .events_ring_size = cfg.events_ring_size,
            .run_dir = try allocator.dupe(u8, cfg.run_dir),
            .instance_uid = try allocator.dupe(u8, cfg.instance_uid),
            .organism_uid = try allocator.dupe(u8, cfg.organism_uid),
            .organism_slug = try allocator.dupe(u8, cfg.organism_slug),
        };
    }

    fn deinit(self: *OwnedConfig, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.prom_addr);
        freeStringSlice(allocator, self.redacted_fields);
        allocator.free(self.run_dir);
        allocator.free(self.instance_uid);
        allocator.free(self.organism_uid);
        allocator.free(self.organism_slug);
        self.* = .{};
    }
};

pub const Logger = struct {
    obs: *Observability,
    name: []const u8,
    level: Level,

    pub fn enabled(self: Logger, level: Level) bool {
        return @intFromEnum(level) >= @intFromEnum(self.level);
    }

    pub fn trace(self: Logger, message: []const u8, fields: []const Field) !void {
        try self.log(.trace, message, fields);
    }

    pub fn debug(self: Logger, message: []const u8, fields: []const Field) !void {
        try self.log(.debug, message, fields);
    }

    pub fn info(self: Logger, message: []const u8, fields: []const Field) !void {
        try self.log(.info, message, fields);
    }

    pub fn warn(self: Logger, message: []const u8, fields: []const Field) !void {
        try self.log(.warn, message, fields);
    }

    pub fn err(self: Logger, message: []const u8, fields: []const Field) !void {
        try self.log(.err, message, fields);
    }

    pub fn fatal(self: Logger, message: []const u8, fields: []const Field) !void {
        try self.log(.fatal, message, fields);
    }

    fn log(self: Logger, level: Level, message: []const u8, fields: []const Field) !void {
        if (!self.enabled(level) or !self.obs.enabled(.logs)) return;
        const redacted = try redactFields(self.obs.allocator, fields, self.obs.cfg.redacted_fields);
        defer freeFields(self.obs.allocator, redacted);
        const entry = LogRecord{
            .timestamp_ns = nowNs(),
            .level = level,
            .slug = self.obs.cfg.slug,
            .instance_uid = self.obs.cfg.instance_uid,
            .message = message,
            .fields = redacted,
        };
        try self.obs.logRecord(entry);
    }
};

pub const Observability = struct {
    allocator: std.mem.Allocator,
    cfg: OwnedConfig,
    families: FamilySet,
    log_ring: ?LogRing = null,
    event_bus: ?EventBus = null,
    registry: ?Registry = null,
    disk_writers_enabled: bool = false,

    pub fn deinit(self: *Observability) void {
        if (self.log_ring) |*ring| ring.deinit();
        if (self.event_bus) |*bus| bus.deinit();
        if (self.registry) |*reg| reg.deinit();
        self.cfg.deinit(self.allocator);
        self.* = undefined;
    }

    pub fn enabled(self: *const Observability, family: Family) bool {
        return self.families.contains(family);
    }

    pub fn isOrganismRoot(self: *const Observability) bool {
        return self.cfg.organism_uid.len != 0 and std.mem.eql(u8, self.cfg.organism_uid, self.cfg.instance_uid);
    }

    pub fn logger(self: *Observability, name: []const u8) Logger {
        return .{
            .obs = self,
            .name = name,
            .level = if (self.enabled(.logs)) self.cfg.default_log_level orelse .info else .fatal,
        };
    }

    pub fn counter(self: *Observability, name: []const u8, help: []const u8, labels: []const Label) !?*Counter {
        if (!self.enabled(.metrics)) return null;
        return try self.registry.?.counter(name, help, labels);
    }

    pub fn gauge(self: *Observability, name: []const u8, help: []const u8, labels: []const Label) !?*Gauge {
        if (!self.enabled(.metrics)) return null;
        return try self.registry.?.gauge(name, help, labels);
    }

    pub fn histogram(self: *Observability, name: []const u8, help: []const u8, labels: []const Label, bounds: ?[]const f64) !?*Histogram {
        if (!self.enabled(.metrics)) return null;
        return try self.registry.?.histogram(name, help, labels, bounds);
    }

    pub fn emit(self: *Observability, event_type: EventType, payload: []const Field) !void {
        if (!self.enabled(.events)) return;
        const redacted = try redactFields(self.allocator, payload, self.cfg.redacted_fields);
        defer freeFields(self.allocator, redacted);
        const event = Event{
            .timestamp_ns = nowNs(),
            .event_type = event_type,
            .slug = self.cfg.slug,
            .instance_uid = self.cfg.instance_uid,
            .payload = redacted,
        };
        try self.event_bus.?.emit(event);
        if (self.disk_writers_enabled and self.cfg.run_dir.len != 0) {
            try appendJsonLinePath(self.allocator, self.cfg.run_dir, "events.jsonl", try eventToJson(self.allocator, event));
        }
    }

    fn logRecord(self: *Observability, entry: LogRecord) !void {
        if (self.log_ring) |*ring| try ring.push(entry);
        if (self.disk_writers_enabled and self.cfg.run_dir.len != 0) {
            try appendJsonLinePath(self.allocator, self.cfg.run_dir, "stdout.log", try logRecordToJson(self.allocator, entry));
        }
    }

    pub fn close(self: *Observability) void {
        if (self.event_bus) |*bus| bus.close();
    }
};

const service_methods = [_]grpc_server.Method{
    .{ .path = "/holons.v1.HolonObservability/Logs", .stream_handler = logsHandler },
    .{ .path = "/holons.v1.HolonObservability/Metrics", .stream_handler = metricsHandler },
    .{ .path = "/holons.v1.HolonObservability/Events", .stream_handler = eventsHandler },
};

pub fn serviceMethods() []const grpc_server.Method {
    return service_methods[0..];
}

fn logsHandler(allocator: std.mem.Allocator, request_bytes: []const u8, stream: *grpc_server.ServerStream) !void {
    const obs = current() orelse return error.ObservabilityNotConfigured;
    if (!obs.enabled(.logs)) return error.LogsFamilyDisabled;
    const ring = if (obs.log_ring) |*value| value else return error.LogsFamilyDisabled;
    const request = c.holons__v1__logs_request__unpack(null, request_bytes.len, request_bytes.ptr) orelse
        return error.DecodeLogsRequestFailed;
    defer c.holons__v1__logs_request__free_unpacked(request, null);

    const min_level = if (request.*.min_severity_number == c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_UNSPECIFIED)
        @intFromEnum(Level.info)
    else
        @as(i32, @intCast(request.*.min_severity_number));
    var last_seen_sequence: u64 = 0;
    const cutoff_ns = cutoffFromDuration(request.*.since);
    try sendLogRecords(allocator, ring, request, min_level, stream, &last_seen_sequence, cutoff_ns);

    while (request.*.follow != 0 and !stream.cancelled()) {
        sleepMillis(25);
        try sendLogRecords(allocator, ring, request, min_level, stream, &last_seen_sequence, cutoff_ns);
    }
}

fn metricsHandler(allocator: std.mem.Allocator, request_bytes: []const u8, stream: *grpc_server.ServerStream) !void {
    const obs = current() orelse return error.ObservabilityNotConfigured;
    if (!obs.enabled(.metrics)) return error.MetricsFamilyDisabled;
    const registry = if (obs.registry) |*value| value else return error.MetricsFamilyDisabled;
    const request = c.holons__v1__metrics_request__unpack(null, request_bytes.len, request_bytes.ptr) orelse
        return error.DecodeMetricsRequestFailed;
    defer c.holons__v1__metrics_request__free_unpacked(request, null);
    try sendMetrics(allocator, obs, registry, request, stream);
}

fn eventsHandler(allocator: std.mem.Allocator, request_bytes: []const u8, stream: *grpc_server.ServerStream) !void {
    const obs = current() orelse return error.ObservabilityNotConfigured;
    if (!obs.enabled(.events)) return error.EventsFamilyDisabled;
    const bus = if (obs.event_bus) |*value| value else return error.EventsFamilyDisabled;
    const request = c.holons__v1__events_request__unpack(null, request_bytes.len, request_bytes.ptr) orelse
        return error.DecodeEventsRequestFailed;
    defer c.holons__v1__events_request__free_unpacked(request, null);

    var last_seen_sequence: u64 = 0;
    const cutoff_ns = cutoffFromDuration(request.*.since);
    try sendEvents(allocator, bus, request, stream, &last_seen_sequence, cutoff_ns);

    while (request.*.follow != 0 and !stream.cancelled()) {
        sleepMillis(25);
        try sendEvents(allocator, bus, request, stream, &last_seen_sequence, cutoff_ns);
    }
}

fn sendLogRecords(
    allocator: std.mem.Allocator,
    ring: *LogRing,
    request: *c.Holons__V1__LogsRequest,
    min_level: i32,
    stream: *grpc_server.ServerStream,
    last_seen_sequence: *u64,
    cutoff_ns: ?i128,
) !void {
    const entries = if (cutoff_ns) |cutoff| try ring.drainSince(allocator, cutoff) else try ring.drain(allocator);
    defer freeLogRecords(allocator, entries);
    for (entries) |entry| {
        if (entry.sequence <= last_seen_sequence.*) continue;
        if (!matchesLogRequest(entry, request, min_level)) continue;
        const encoded = try packLogRecord(allocator, entry);
        defer allocator.free(encoded);
        try stream.send(encoded);
        last_seen_sequence.* = entry.sequence;
    }
}

fn sendEvents(
    allocator: std.mem.Allocator,
    bus: *EventBus,
    request: *c.Holons__V1__EventsRequest,
    stream: *grpc_server.ServerStream,
    last_seen_sequence: *u64,
    cutoff_ns: ?i128,
) !void {
    const events = if (cutoff_ns) |cutoff| try bus.drainSince(allocator, cutoff) else try bus.drain(allocator);
    defer freeEventsSlice(allocator, events);
    for (events) |event| {
        if (event.sequence <= last_seen_sequence.*) continue;
        if (!matchesEventRequest(event, request)) continue;
        const encoded = try packEventRecord(allocator, event);
        defer allocator.free(encoded);
        try stream.send(encoded);
        last_seen_sequence.* = event.sequence;
    }
}

fn matchesLogRequest(entry: LogRecord, request: *c.Holons__V1__LogsRequest, min_level: i32) bool {
    if (@intFromEnum(entry.level) < min_level) return false;
    if (request.*.n_session_ids != 0 and !containsCStr(request.*.session_ids, request.*.n_session_ids, entry.session_id)) return false;
    if (request.*.n_rpc_methods != 0 and !containsCStr(request.*.rpc_methods, request.*.n_rpc_methods, entry.rpc_method)) return false;
    return true;
}

fn matchesEventRequest(event: Event, request: *c.Holons__V1__EventsRequest) bool {
    if (request.*.n_event_names == 0) return true;
    for (request.*.event_names[0..request.*.n_event_names]) |wanted| {
        if (std.mem.eql(u8, cstr(wanted), event.event_type.eventName())) return true;
    }
    return false;
}

fn containsCStr(values: [*c][*c]u8, len: usize, needle: []const u8) bool {
    for (values[0..len]) |value| {
        if (std.mem.eql(u8, cstr(value), needle)) return true;
    }
    return false;
}

fn cutoffFromDuration(duration: ?*c.Google__Protobuf__Duration) ?i128 {
    const value = duration orelse return null;
    const seconds = @max(value.*.seconds, 0);
    const nanos = @max(value.*.nanos, 0);
    return nowNs() - (@as(i128, @intCast(seconds)) * std.time.ns_per_s + @as(i128, @intCast(nanos)));
}

fn packLogRecord(allocator: std.mem.Allocator, entry: LogRecord) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var msg: c.Holons__V1__LogRecord = undefined;
    c.holons__v1__log_record__init(&msg);
    msg.time_unix_nano = unixNanoU64(entry.timestamp_ns);
    msg.observed_time_unix_nano = msg.time_unix_nano;
    msg.severity_number = severityToProto(entry.level);
    msg.severity_text = try z(a, entry.level.name());
    msg.body = try anyValueProto(a, .{ .string_value = entry.message });
    try fillLogAttributes(a, &msg, entry.slug, entry.instance_uid, entry.session_id, entry.rpc_method, entry.caller, entry.fields);
    try fillLogChain(a, &msg, entry.chain);
    return packProto(allocator, &msg.base, c.holons__v1__log_record__get_packed_size(&msg));
}

fn packEventRecord(allocator: std.mem.Allocator, event: Event) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var msg: c.Holons__V1__LogRecord = undefined;
    c.holons__v1__log_record__init(&msg);
    const event_name = event.event_type.eventName();
    msg.time_unix_nano = unixNanoU64(event.timestamp_ns);
    msg.observed_time_unix_nano = msg.time_unix_nano;
    msg.severity_number = severityToProto(.info);
    msg.severity_text = try z(a, Level.info.name());
    msg.body = try anyValueProto(a, .{ .string_value = event_name });
    msg.event_name = try z(a, event_name);
    try fillLogAttributes(a, &msg, event.slug, event.instance_uid, event.session_id, "", "", event.payload);
    try fillLogChain(a, &msg, event.chain);
    return packProto(allocator, &msg.base, c.holons__v1__log_record__get_packed_size(&msg));
}

pub fn logRecordFromBytes(allocator: std.mem.Allocator, bytes: []const u8) !LogRecord {
    const raw = c.holons__v1__log_record__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeLogRecordFailed;
    defer c.holons__v1__log_record__free_unpacked(raw, null);
    return logRecordFromProto(allocator, raw);
}

pub fn eventFromBytes(allocator: std.mem.Allocator, bytes: []const u8) !Event {
    const raw = c.holons__v1__log_record__unpack(null, bytes.len, bytes.ptr) orelse
        return error.DecodeEventRecordFailed;
    defer c.holons__v1__log_record__free_unpacked(raw, null);
    return eventFromProto(allocator, raw);
}

fn logRecordFromProto(allocator: std.mem.Allocator, raw: *c.Holons__V1__LogRecord) !LogRecord {
    const rpc_method = attributeString(raw, "rpc.method") orelse "";
    const caller = attributeString(raw, "code.caller") orelse "";
    return .{
        .timestamp_ns = raw.*.time_unix_nano,
        .level = levelFromProto(raw.*.severity_number),
        .slug = try allocator.dupe(u8, attributeString(raw, "holons.slug") orelse ""),
        .instance_uid = try allocator.dupe(u8, attributeString(raw, "holons.instance_uid") orelse ""),
        .session_id = try allocator.dupe(u8, attributeString(raw, "holons.session_id") orelse ""),
        .rpc_method = try allocator.dupe(u8, rpc_method),
        .message = try allocator.dupe(u8, anyValueString(raw.*.body) orelse ""),
        .fields = try fieldsFromAttributes(allocator, raw),
        .caller = try allocator.dupe(u8, caller),
        .chain = try chainFromStrings(allocator, raw.*.chain, raw.*.n_chain),
    };
}

fn eventFromProto(allocator: std.mem.Allocator, raw: *c.Holons__V1__LogRecord) !Event {
    const event_name = if (cstr(raw.*.event_name).len == 0) anyValueString(raw.*.body) orelse "" else cstr(raw.*.event_name);
    return .{
        .timestamp_ns = raw.*.time_unix_nano,
        .event_type = eventTypeFromName(event_name),
        .slug = try allocator.dupe(u8, attributeString(raw, "holons.slug") orelse ""),
        .instance_uid = try allocator.dupe(u8, attributeString(raw, "holons.instance_uid") orelse ""),
        .session_id = try allocator.dupe(u8, attributeString(raw, "holons.session_id") orelse ""),
        .payload = try fieldsFromAttributes(allocator, raw),
        .chain = try chainFromStrings(allocator, raw.*.chain, raw.*.n_chain),
    };
}

fn sendMetrics(
    allocator: std.mem.Allocator,
    obs: *Observability,
    registry: *Registry,
    request: *c.Holons__V1__MetricsRequest,
    stream: *grpc_server.ServerStream,
) !void {
    registry.mutex.lockUncancelable(std.Options.debug_io);
    defer registry.mutex.unlock(std.Options.debug_io);

    var sent: usize = 0;
    for (registry.counters.items) |*counter| {
        if (!metricAllowed(request, counter.name)) continue;
        const encoded = try packCounterMetric(allocator, obs, counter);
        defer allocator.free(encoded);
        try stream.send(encoded);
        sent += 1;
    }
    for (registry.gauges.items) |*gauge| {
        if (!metricAllowed(request, gauge.name)) continue;
        const encoded = try packGaugeMetric(allocator, obs, gauge);
        defer allocator.free(encoded);
        try stream.send(encoded);
        sent += 1;
    }
    for (registry.histograms.items) |*histogram| {
        if (!metricAllowed(request, histogram.name)) continue;
        var snapshot = try histogram.snapshot(allocator);
        defer snapshot.deinit(allocator);
        const encoded = try packHistogramMetric(allocator, obs, histogram, snapshot);
        defer allocator.free(encoded);
        try stream.send(encoded);
        sent += 1;
    }
    if (sent == 0 and metricAllowed(request, "holons_instance_info")) {
        const encoded = try packIdentityMetric(allocator, obs);
        defer allocator.free(encoded);
        try stream.send(encoded);
    }
}

fn packIdentityMetric(allocator: std.mem.Allocator, obs: *Observability) ![]u8 {
    const labels = [_]Label{};
    return packGaugeMetricValue(allocator, obs, "holons_instance_info", "Holon instance identity", labels[0..], 1);
}

fn packCounterMetric(allocator: std.mem.Allocator, obs: *Observability, counter: *Counter) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();
    var msg: c.Holons__V1__Metric = undefined;
    c.holons__v1__metric__init(&msg);
    msg.name = try z(a, counter.name);
    msg.description = try z(a, counter.help);
    msg.data_case = c.HOLONS__V1__METRIC__DATA_SUM;
    const sum = try a.create(c.Holons__V1__Sum);
    c.holons__v1__sum__init(sum);
    const point = try numberDataPoint(a, obs, counter.labels, .{ .int_value = counter.read() });
    var ptrs = try a.alloc([*c]c.Holons__V1__NumberDataPoint, 1);
    ptrs[0] = point;
    sum.n_data_points = 1;
    sum.data_points = ptrs.ptr;
    sum.aggregation_temporality = c.HOLONS__V1__AGGREGATION_TEMPORALITY__AGGREGATION_TEMPORALITY_CUMULATIVE;
    sum.is_monotonic = 1;
    msg.unnamed_0.sum = sum;
    return packProto(allocator, &msg.base, c.holons__v1__metric__get_packed_size(&msg));
}

fn packGaugeMetric(allocator: std.mem.Allocator, obs: *Observability, gauge: *Gauge) ![]u8 {
    return packGaugeMetricValue(allocator, obs, gauge.name, gauge.help, gauge.labels, gauge.read());
}

fn packGaugeMetricValue(
    allocator: std.mem.Allocator,
    obs: *Observability,
    name: []const u8,
    help: []const u8,
    labels: []const Label,
    value: f64,
) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();
    var msg: c.Holons__V1__Metric = undefined;
    c.holons__v1__metric__init(&msg);
    msg.name = try z(a, name);
    msg.description = try z(a, help);
    msg.data_case = c.HOLONS__V1__METRIC__DATA_GAUGE;
    const gauge = try a.create(c.Holons__V1__Gauge);
    c.holons__v1__gauge__init(gauge);
    const point = try numberDataPoint(a, obs, labels, .{ .double_value = value });
    var ptrs = try a.alloc([*c]c.Holons__V1__NumberDataPoint, 1);
    ptrs[0] = point;
    gauge.n_data_points = 1;
    gauge.data_points = ptrs.ptr;
    msg.unnamed_0.gauge = gauge;
    return packProto(allocator, &msg.base, c.holons__v1__metric__get_packed_size(&msg));
}

fn packHistogramMetric(allocator: std.mem.Allocator, obs: *Observability, histogram: *Histogram, snapshot: HistogramSnapshot) ![]u8 {
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();
    var msg: c.Holons__V1__Metric = undefined;
    c.holons__v1__metric__init(&msg);
    msg.name = try z(a, histogram.name);
    msg.description = try z(a, histogram.help);
    msg.data_case = c.HOLONS__V1__METRIC__DATA_HISTOGRAM;
    const hist = try a.create(c.Holons__V1__Histogram);
    c.holons__v1__histogram__init(hist);
    hist.aggregation_temporality = c.HOLONS__V1__AGGREGATION_TEMPORALITY__AGGREGATION_TEMPORALITY_CUMULATIVE;
    const point = try a.create(c.Holons__V1__HistogramDataPoint);
    c.holons__v1__histogram_data_point__init(point);
    const now = unixNanoU64(nowNs());
    point.start_time_unix_nano = now;
    point.time_unix_nano = now;
    point.count = @intCast(@max(snapshot.total, 0));
    point.sum = snapshot.sum;
    point.n_explicit_bounds = snapshot.bounds.len;
    point.explicit_bounds = (try a.dupe(f64, snapshot.bounds)).ptr;
    const bucket_counts = try histogramBucketCounts(a, snapshot);
    point.n_bucket_counts = bucket_counts.len;
    point.bucket_counts = bucket_counts.ptr;
    try fillDataPointAttributes(a, point, obs, histogram.labels);
    var ptrs = try a.alloc([*c]c.Holons__V1__HistogramDataPoint, 1);
    ptrs[0] = point;
    hist.n_data_points = 1;
    hist.data_points = ptrs.ptr;
    msg.unnamed_0.histogram = hist;
    return packProto(allocator, &msg.base, c.holons__v1__metric__get_packed_size(&msg));
}

fn numberDataPoint(a: std.mem.Allocator, obs: *Observability, labels: []const Label, value: AnyValue) !*c.Holons__V1__NumberDataPoint {
    const point = try a.create(c.Holons__V1__NumberDataPoint);
    c.holons__v1__number_data_point__init(point);
    const now = unixNanoU64(nowNs());
    point.start_time_unix_nano = now;
    point.time_unix_nano = now;
    try fillNumberDataPointAttributes(a, point, obs, labels);
    switch (value) {
        .int_value => |v| {
            point.value_case = c.HOLONS__V1__NUMBER_DATA_POINT__VALUE_AS_INT;
            point.unnamed_0.as_int = v;
        },
        .double_value => |v| {
            point.value_case = c.HOLONS__V1__NUMBER_DATA_POINT__VALUE_AS_DOUBLE;
            point.unnamed_0.as_double = v;
        },
        else => unreachable,
    }
    return point;
}

fn histogramBucketCounts(a: std.mem.Allocator, snapshot: HistogramSnapshot) ![]u64 {
    var out = try a.alloc(u64, snapshot.bounds.len + 1);
    var previous: i64 = 0;
    for (snapshot.counts, 0..) |cumulative, index| {
        const delta = @max(cumulative - previous, 0);
        out[index] = @intCast(delta);
        previous = cumulative;
    }
    out[snapshot.bounds.len] = @intCast(@max(snapshot.total - previous, 0));
    return out;
}

fn metricAllowed(request: *c.Holons__V1__MetricsRequest, name: []const u8) bool {
    if (request.*.n_name_prefixes == 0) return true;
    for (request.*.name_prefixes[0..request.*.n_name_prefixes]) |prefix| {
        if (std.mem.startsWith(u8, name, cstr(prefix))) return true;
    }
    return false;
}

pub fn prometheusText(allocator: std.mem.Allocator, registry: *Registry) ![]u8 {
    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);

    registry.mutex.lockUncancelable(std.Options.debug_io);
    defer registry.mutex.unlock(std.Options.debug_io);

    for (registry.counters.items) |*counter| {
        if (counter.help.len != 0) try out.print(allocator, "# HELP {s} {s}\n", .{ counter.name, counter.help });
        try out.print(allocator, "# TYPE {s} counter\n", .{counter.name});
        try appendPromSample(&out, allocator, counter.name, counter.labels, @as(f64, @floatFromInt(counter.read())));
    }
    for (registry.gauges.items) |*gauge| {
        if (gauge.help.len != 0) try out.print(allocator, "# HELP {s} {s}\n", .{ gauge.name, gauge.help });
        try out.print(allocator, "# TYPE {s} gauge\n", .{gauge.name});
        try appendPromSample(&out, allocator, gauge.name, gauge.labels, gauge.read());
    }
    for (registry.histograms.items) |*histogram| {
        var snapshot = try histogram.snapshot(allocator);
        defer snapshot.deinit(allocator);
        if (histogram.help.len != 0) try out.print(allocator, "# HELP {s} {s}\n", .{ histogram.name, histogram.help });
        try out.print(allocator, "# TYPE {s} histogram\n", .{histogram.name});
        for (snapshot.bounds, 0..) |bound, index| {
            const labels = try labelsWithLe(allocator, histogram.labels, bound);
            defer freeLabels(allocator, labels);
            const bucket_name = try std.fmt.allocPrint(allocator, "{s}_bucket", .{histogram.name});
            defer allocator.free(bucket_name);
            try appendPromSample(&out, allocator, bucket_name, labels, @as(f64, @floatFromInt(snapshot.counts[index])));
        }
        const count_name = try std.fmt.allocPrint(allocator, "{s}_count", .{histogram.name});
        defer allocator.free(count_name);
        try appendPromSample(&out, allocator, count_name, histogram.labels, @as(f64, @floatFromInt(snapshot.total)));
        const sum_name = try std.fmt.allocPrint(allocator, "{s}_sum", .{histogram.name});
        defer allocator.free(sum_name);
        try appendPromSample(&out, allocator, sum_name, histogram.labels, snapshot.sum);
    }
    return out.toOwnedSlice(allocator);
}

pub fn startPrometheusEndpoint(obs: *Observability) !?[]u8 {
    if (!obs.enabled(.prom)) return null;
    const registry = &(obs.registry orelse return null);
    const fd = net_c.socket(net_c.AF_INET, net_c.SOCK_STREAM, 0);
    if (fd < 0) return error.SocketFailed;
    errdefer _ = net_c.close(fd);

    var yes: c_int = 1;
    _ = net_c.setsockopt(fd, net_c.SOL_SOCKET, net_c.SO_REUSEADDR, &yes, @sizeOf(c_int));

    var addr: net_c.struct_sockaddr_in = std.mem.zeroes(net_c.struct_sockaddr_in);
    if (@hasField(net_c.struct_sockaddr_in, "sin_len")) addr.sin_len = @sizeOf(net_c.struct_sockaddr_in);
    addr.sin_family = net_c.AF_INET;
    addr.sin_port = 0;
    addr.sin_addr.s_addr = net_c.htonl(0x7f000001);
    if (obs.cfg.prom_addr.len != 0) {
        const colon = std.mem.lastIndexOfScalar(u8, obs.cfg.prom_addr, ':') orelse return error.InvalidPrometheusAddress;
        const port = try std.fmt.parseInt(u16, obs.cfg.prom_addr[colon + 1 ..], 10);
        addr.sin_port = net_c.htons(port);
    }
    if (net_c.bind(fd, @ptrCast(&addr), @sizeOf(net_c.struct_sockaddr_in)) != 0) return error.BindFailed;
    if (net_c.listen(fd, 16) != 0) return error.ListenFailed;

    var len: net_c.socklen_t = @sizeOf(net_c.struct_sockaddr_in);
    if (net_c.getsockname(fd, @ptrCast(&addr), &len) != 0) return error.GetSockNameFailed;
    const port = net_c.ntohs(addr.sin_port);

    const ctx = try obs.allocator.create(PrometheusContext);
    ctx.* = .{ .obs = obs, .registry = registry, .fd = fd };
    const worker = try std.Thread.spawn(.{}, prometheusWorker, .{ctx});
    worker.detach();

    return try std.fmt.allocPrint(obs.allocator, "http://127.0.0.1:{}/metrics", .{port});
}

const PrometheusContext = struct {
    obs: *Observability,
    registry: *Registry,
    fd: c_int,
};

fn prometheusWorker(ctx: *PrometheusContext) void {
    while (true) {
        const client = net_c.accept(ctx.fd, null, null);
        if (client < 0) break;
        handlePrometheusClient(ctx, client) catch {};
        _ = net_c.close(client);
    }
    _ = net_c.close(ctx.fd);
    ctx.obs.allocator.destroy(ctx);
}

fn handlePrometheusClient(ctx: *PrometheusContext, client: c_int) !void {
    var buf: [1024]u8 = undefined;
    _ = net_c.read(client, &buf, buf.len);
    const body = try prometheusText(ctx.obs.allocator, ctx.registry);
    defer ctx.obs.allocator.free(body);
    const response = try std.fmt.allocPrint(
        ctx.obs.allocator,
        "HTTP/1.1 200 OK\r\ncontent-type: text/plain; version=0.0.4\r\ncontent-length: {}\r\nconnection: close\r\n\r\n{s}",
        .{ body.len, body },
    );
    defer ctx.obs.allocator.free(response);
    _ = net_c.write(client, response.ptr, response.len);
}

fn appendPromSample(out: *std.ArrayList(u8), allocator: std.mem.Allocator, name: []const u8, labels: []const Label, value: f64) !void {
    try out.appendSlice(allocator, name);
    try appendPromLabels(out, allocator, labels);
    try out.print(allocator, " {d}\n", .{value});
}

fn appendPromLabels(out: *std.ArrayList(u8), allocator: std.mem.Allocator, labels: []const Label) !void {
    if (labels.len == 0) return;
    try out.append(allocator, '{');
    for (labels, 0..) |label, index| {
        if (index != 0) try out.append(allocator, ',');
        const escaped = try promEscape(allocator, label.value);
        defer allocator.free(escaped);
        try out.print(allocator, "{s}=\"{s}\"", .{ label.key, escaped });
    }
    try out.append(allocator, '}');
}

fn promEscape(allocator: std.mem.Allocator, value: []const u8) ![]u8 {
    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);
    for (value) |ch| switch (ch) {
        '\\' => try out.appendSlice(allocator, "\\\\"),
        '\n' => try out.appendSlice(allocator, "\\n"),
        '"' => try out.appendSlice(allocator, "\\\""),
        else => try out.append(allocator, ch),
    };
    return out.toOwnedSlice(allocator);
}

fn labelsWithLe(allocator: std.mem.Allocator, labels: []const Label, bound: f64) ![]Label {
    var out = try allocator.alloc(Label, labels.len + 1);
    errdefer freeLabels(allocator, out);
    for (labels, 0..) |label, index| {
        out[index] = .{
            .key = try allocator.dupe(u8, label.key),
            .value = try allocator.dupe(u8, label.value),
        };
    }
    out[labels.len] = .{
        .key = try allocator.dupe(u8, "le"),
        .value = try std.fmt.allocPrint(allocator, "{d}", .{bound}),
    };
    return out;
}

fn fillLogAttributes(
    a: std.mem.Allocator,
    msg: *c.Holons__V1__LogRecord,
    slug: []const u8,
    instance_uid: []const u8,
    session_id: []const u8,
    rpc_method: []const u8,
    caller: []const u8,
    fields: []const Field,
) !void {
    const extra_count = fields.len + @intFromBool(rpc_method.len != 0) + @intFromBool(caller.len != 0);
    var entries = try a.alloc(c.Holons__V1__KeyValue, 5 + extra_count);
    var ptrs = try a.alloc([*c]c.Holons__V1__KeyValue, entries.len);
    var index: usize = 0;
    try fillKeyValue(a, &entries[index], "holons.slug", .{ .string_value = slug });
    ptrs[index] = &entries[index];
    index += 1;
    try fillKeyValue(a, &entries[index], "service.name", .{ .string_value = slug });
    ptrs[index] = &entries[index];
    index += 1;
    try fillKeyValue(a, &entries[index], "holons.instance_uid", .{ .string_value = instance_uid });
    ptrs[index] = &entries[index];
    index += 1;
    try fillKeyValue(a, &entries[index], "service.instance.id", .{ .string_value = instance_uid });
    ptrs[index] = &entries[index];
    index += 1;
    try fillKeyValue(a, &entries[index], "holons.session_id", .{ .string_value = session_id });
    ptrs[index] = &entries[index];
    index += 1;
    if (rpc_method.len != 0) {
        try fillKeyValue(a, &entries[index], "rpc.method", .{ .string_value = rpc_method });
        ptrs[index] = &entries[index];
        index += 1;
    }
    if (caller.len != 0) {
        try fillKeyValue(a, &entries[index], "code.caller", .{ .string_value = caller });
        ptrs[index] = &entries[index];
        index += 1;
    }
    for (fields) |field| {
        try fillKeyValue(a, &entries[index], field.key, field.value);
        ptrs[index] = &entries[index];
        index += 1;
    }
    msg.n_attributes = index;
    msg.attributes = ptrs.ptr;
}

fn fillNumberDataPointAttributes(a: std.mem.Allocator, point: *c.Holons__V1__NumberDataPoint, obs: *Observability, labels: []const Label) !void {
    const attrs = try metricAttributes(a, obs, labels);
    point.n_attributes = attrs.ptrs.len;
    point.attributes = attrs.ptrs.ptr;
}

fn fillDataPointAttributes(a: std.mem.Allocator, point: *c.Holons__V1__HistogramDataPoint, obs: *Observability, labels: []const Label) !void {
    const attrs = try metricAttributes(a, obs, labels);
    point.n_attributes = attrs.ptrs.len;
    point.attributes = attrs.ptrs.ptr;
}

const MetricAttributes = struct {
    entries: []c.Holons__V1__KeyValue,
    ptrs: [][*c]c.Holons__V1__KeyValue,
};

fn metricAttributes(a: std.mem.Allocator, obs: *Observability, labels: []const Label) !MetricAttributes {
    var entries = try a.alloc(c.Holons__V1__KeyValue, labels.len + 4);
    var ptrs = try a.alloc([*c]c.Holons__V1__KeyValue, labels.len + 4);
    var index: usize = 0;
    try fillKeyValue(a, &entries[index], "holons.slug", .{ .string_value = obs.cfg.slug });
    ptrs[index] = &entries[index];
    index += 1;
    try fillKeyValue(a, &entries[index], "service.name", .{ .string_value = obs.cfg.slug });
    ptrs[index] = &entries[index];
    index += 1;
    try fillKeyValue(a, &entries[index], "holons.instance_uid", .{ .string_value = obs.cfg.instance_uid });
    ptrs[index] = &entries[index];
    index += 1;
    try fillKeyValue(a, &entries[index], "service.instance.id", .{ .string_value = obs.cfg.instance_uid });
    ptrs[index] = &entries[index];
    index += 1;
    for (labels) |label| {
        try fillKeyValue(a, &entries[index], label.key, .{ .string_value = label.value });
        ptrs[index] = &entries[index];
        index += 1;
    }
    return .{ .entries = entries, .ptrs = ptrs[0..index] };
}

fn fillLogChain(a: std.mem.Allocator, msg: *c.Holons__V1__LogRecord, chain: []const Hop) !void {
    if (chain.len == 0) return;
    var entries = try a.alloc([*c]u8, chain.len);
    for (chain, 0..) |hop, index| {
        const encoded = if (hop.instance_uid.len == 0)
            hop.slug
        else
            try std.fmt.allocPrint(a, "{s}/{s}", .{ hop.slug, hop.instance_uid });
        entries[index] = try z(a, encoded);
    }
    msg.n_chain = chain.len;
    msg.chain = entries.ptr;
}

fn fillKeyValue(a: std.mem.Allocator, kv: *c.Holons__V1__KeyValue, key: []const u8, value: AnyValue) !void {
    c.holons__v1__key_value__init(kv);
    kv.key = try z(a, key);
    kv.value = try anyValueProto(a, value);
}

fn anyValueProto(a: std.mem.Allocator, value: AnyValue) !*c.Holons__V1__AnyValue {
    const out = try a.create(c.Holons__V1__AnyValue);
    c.holons__v1__any_value__init(out);
    switch (value) {
        .string_value => |v| {
            out.value_case = c.HOLONS__V1__ANY_VALUE__VALUE_STRING_VALUE;
            out.unnamed_0.string_value = try z(a, v);
        },
        .bool_value => |v| {
            out.value_case = c.HOLONS__V1__ANY_VALUE__VALUE_BOOL_VALUE;
            out.unnamed_0.bool_value = @intFromBool(v);
        },
        .int_value => |v| {
            out.value_case = c.HOLONS__V1__ANY_VALUE__VALUE_INT_VALUE;
            out.unnamed_0.int_value = v;
        },
        .double_value => |v| {
            out.value_case = c.HOLONS__V1__ANY_VALUE__VALUE_DOUBLE_VALUE;
            out.unnamed_0.double_value = v;
        },
    }
    return out;
}

fn severityToProto(level: Level) c.Holons__V1__SeverityNumber {
    return switch (level) {
        .trace => c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_TRACE,
        .debug => c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_DEBUG,
        .info => c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_INFO,
        .warn => c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_WARN,
        .err => c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_ERROR,
        .fatal => c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_FATAL,
        .unset => c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_UNSPECIFIED,
    };
}

fn levelFromProto(level: c.Holons__V1__SeverityNumber) Level {
    return switch (level) {
        c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_TRACE => .trace,
        c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_DEBUG => .debug,
        c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_INFO => .info,
        c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_WARN => .warn,
        c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_ERROR => .err,
        c.HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_FATAL => .fatal,
        else => .unset,
    };
}

fn eventTypeFromName(name: []const u8) EventType {
    inline for ([_]EventType{ .instance_spawned, .instance_ready, .instance_exited, .instance_crashed, .session_started, .session_ended, .handler_panic, .config_reloaded }) |event_type| {
        if (std.mem.eql(u8, name, event_type.eventName())) return event_type;
    }
    return .unspecified;
}

fn chainFromStrings(allocator: std.mem.Allocator, raw_chain: [*c][*c]u8, len: usize) ![]Hop {
    var out = try allocator.alloc(Hop, len);
    errdefer freeHops(allocator, out);
    if (len == 0) return out;
    const chain = raw_chain orelse return error.InvalidChain;
    for (chain[0..len], 0..) |hop, index| {
        const raw_hop = cstr(hop);
        const slash = std.mem.indexOfScalar(u8, raw_hop, '/');
        out[index] = .{
            .slug = try allocator.dupe(u8, if (slash) |pos| raw_hop[0..pos] else raw_hop),
            .instance_uid = try allocator.dupe(u8, if (slash) |pos| raw_hop[pos + 1 ..] else ""),
        };
    }
    return out;
}

fn attributeString(raw: *c.Holons__V1__LogRecord, key: []const u8) ?[]const u8 {
    for (raw.*.attributes[0..raw.*.n_attributes]) |attr| {
        if (std.mem.eql(u8, cstr(attr.*.key), key)) return anyValueString(attr.*.value);
    }
    return null;
}

fn anyValueString(value: ?*c.Holons__V1__AnyValue) ?[]const u8 {
    const raw = value orelse return null;
    return switch (raw.*.value_case) {
        c.HOLONS__V1__ANY_VALUE__VALUE_STRING_VALUE => cstr(raw.*.unnamed_0.string_value),
        else => null,
    };
}

fn anyValueFromProto(value: ?*c.Holons__V1__AnyValue) AnyValue {
    const raw = value orelse return .{ .string_value = "" };
    return switch (raw.*.value_case) {
        c.HOLONS__V1__ANY_VALUE__VALUE_STRING_VALUE => .{ .string_value = cstr(raw.*.unnamed_0.string_value) },
        c.HOLONS__V1__ANY_VALUE__VALUE_BOOL_VALUE => .{ .bool_value = raw.*.unnamed_0.bool_value != 0 },
        c.HOLONS__V1__ANY_VALUE__VALUE_INT_VALUE => .{ .int_value = raw.*.unnamed_0.int_value },
        c.HOLONS__V1__ANY_VALUE__VALUE_DOUBLE_VALUE => .{ .double_value = raw.*.unnamed_0.double_value },
        else => .{ .string_value = "" },
    };
}

fn fieldsFromAttributes(allocator: std.mem.Allocator, raw: *c.Holons__V1__LogRecord) ![]Field {
    var count: usize = 0;
    for (raw.*.attributes[0..raw.*.n_attributes]) |attr| {
        if (!isResourceAttribute(cstr(attr.*.key))) count += 1;
    }
    var out = try allocator.alloc(Field, count);
    errdefer freeFields(allocator, out);
    var index: usize = 0;
    for (raw.*.attributes[0..raw.*.n_attributes]) |attr| {
        const key = cstr(attr.*.key);
        if (isResourceAttribute(key)) continue;
        out[index] = .{
            .key = try allocator.dupe(u8, key),
            .value = try AnyValue.clone(allocator, anyValueFromProto(attr.*.value)),
        };
        index += 1;
    }
    return out;
}

fn isResourceAttribute(key: []const u8) bool {
    return std.mem.eql(u8, key, "holons.slug") or
        std.mem.eql(u8, key, "service.name") or
        std.mem.eql(u8, key, "holons.instance_uid") or
        std.mem.eql(u8, key, "service.instance.id") or
        std.mem.eql(u8, key, "holons.session_id") or
        std.mem.eql(u8, key, "rpc.method") or
        std.mem.eql(u8, key, "code.caller");
}

fn packProto(allocator: std.mem.Allocator, base: *c.ProtobufCMessage, len: usize) ![]u8 {
    const buf = try allocator.alloc(u8, len);
    errdefer allocator.free(buf);
    const encoded_len = c.protobuf_c_message_pack(base, buf.ptr);
    if (encoded_len != len) return error.EncodeSizeMismatch;
    return buf;
}

fn z(allocator: std.mem.Allocator, value: []const u8) ![*c]u8 {
    const out = try allocator.dupeZ(u8, value);
    return out.ptr;
}

fn cstr(ptr: [*c]const u8) []const u8 {
    if (ptr == null) return "";
    return std.mem.span(ptr);
}

fn freeLogRecords(allocator: std.mem.Allocator, entries: []LogRecord) void {
    for (entries) |*entry| entry.deinit(allocator);
    allocator.free(entries);
}

fn freeEventsSlice(allocator: std.mem.Allocator, events: []Event) void {
    for (events) |*event| event.deinit(allocator);
    allocator.free(events);
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(
        std.Io.Threaded.global_single_threaded.io(),
        std.Io.Duration.fromMilliseconds(ms),
        .awake,
    ) catch {};
}

var current_mutex: std.Io.Mutex = .init;
var current_obs: ?*Observability = null;

pub fn configure(allocator: std.mem.Allocator, cfg: Config) !*Observability {
    var merged = cfg;
    const instance_uid = try processEnvOwned(allocator, "OP_INSTANCE_UID");
    defer if (instance_uid) |value| allocator.free(value);
    const organism_uid = try processEnvOwned(allocator, "OP_ORGANISM_UID");
    defer if (organism_uid) |value| allocator.free(value);
    const organism_slug = try processEnvOwned(allocator, "OP_ORGANISM_SLUG");
    defer if (organism_slug) |value| allocator.free(value);
    const prom_addr = try processEnvOwned(allocator, "OP_PROM_ADDR");
    defer if (prom_addr) |value| allocator.free(value);
    const run_dir = try processEnvOwned(allocator, "OP_RUN_DIR");
    defer if (run_dir) |value| allocator.free(value);
    const op_obs = try processEnvOwned(allocator, "OP_OBS");
    defer if (op_obs) |value| allocator.free(value);

    if (merged.instance_uid.len == 0) merged.instance_uid = instance_uid orelse "";
    if (merged.organism_uid.len == 0) merged.organism_uid = organism_uid orelse "";
    if (merged.organism_slug.len == 0) merged.organism_slug = organism_slug orelse "";
    if (merged.prom_addr.len == 0) merged.prom_addr = prom_addr orelse "";
    if (merged.run_dir.len == 0) merged.run_dir = run_dir orelse "";
    return configureFromRaw(allocator, merged, op_obs orelse "");
}

pub fn configureFromEnv(allocator: std.mem.Allocator, cfg: Config, env: []const EnvEntry) !*Observability {
    var merged = cfg;
    if (merged.instance_uid.len == 0) merged.instance_uid = envValue(env, "OP_INSTANCE_UID") orelse "";
    if (merged.organism_uid.len == 0) merged.organism_uid = envValue(env, "OP_ORGANISM_UID") orelse "";
    if (merged.organism_slug.len == 0) merged.organism_slug = envValue(env, "OP_ORGANISM_SLUG") orelse "";
    if (merged.prom_addr.len == 0) merged.prom_addr = envValue(env, "OP_PROM_ADDR") orelse "";
    if (merged.run_dir.len == 0) merged.run_dir = envValue(env, "OP_RUN_DIR") orelse "";
    return configureFromRaw(allocator, merged, envValue(env, "OP_OBS") orelse "");
}

pub fn createFromEnv(allocator: std.mem.Allocator, cfg: Config, env: []const EnvEntry) !*Observability {
    var merged = cfg;
    if (merged.instance_uid.len == 0) merged.instance_uid = envValue(env, "OP_INSTANCE_UID") orelse "";
    if (merged.organism_uid.len == 0) merged.organism_uid = envValue(env, "OP_ORGANISM_UID") orelse "";
    if (merged.organism_slug.len == 0) merged.organism_slug = envValue(env, "OP_ORGANISM_SLUG") orelse "";
    if (merged.prom_addr.len == 0) merged.prom_addr = envValue(env, "OP_PROM_ADDR") orelse "";
    if (merged.run_dir.len == 0) merged.run_dir = envValue(env, "OP_RUN_DIR") orelse "";
    return createFromRaw(allocator, merged, envValue(env, "OP_OBS") orelse "");
}

fn configureFromRaw(allocator: std.mem.Allocator, cfg: Config, op_obs: []const u8) !*Observability {
    const obs = try createFromRaw(allocator, cfg, op_obs);
    errdefer destroy(obs);
    reset();
    current_mutex.lockUncancelable(std.Options.debug_io);
    current_obs = obs;
    current_mutex.unlock(std.Options.debug_io);
    return obs;
}

fn createFromRaw(allocator: std.mem.Allocator, cfg: Config, op_obs: []const u8) !*Observability {
    const families = try parseOpObs(op_obs);
    var owned = try OwnedConfig.fromConfig(allocator, cfg);
    errdefer owned.deinit(allocator);
    if (owned.slug.len == 0) {
        allocator.free(owned.slug);
        owned.slug = try processSlug(allocator);
    }
    if (owned.instance_uid.len == 0) {
        allocator.free(owned.instance_uid);
        owned.instance_uid = try newInstanceUid(allocator);
    }
    if (owned.run_dir.len != 0) {
        const derived = try deriveRunDir(allocator, owned.run_dir, owned.slug, owned.instance_uid);
        allocator.free(owned.run_dir);
        owned.run_dir = derived;
    }
    const obs = try allocator.create(Observability);
    errdefer allocator.destroy(obs);
    obs.* = .{
        .allocator = allocator,
        .cfg = owned,
        .families = families,
        .log_ring = if (families.logs) LogRing.init(allocator, if (cfg.logs_ring_size == 0) 1024 else cfg.logs_ring_size) else null,
        .event_bus = if (families.events) EventBus.init(allocator, if (cfg.events_ring_size == 0) 256 else cfg.events_ring_size) else null,
        .registry = if (families.metrics) Registry.init(allocator) else null,
    };
    return obs;
}

pub fn current() ?*Observability {
    current_mutex.lockUncancelable(std.Options.debug_io);
    defer current_mutex.unlock(std.Options.debug_io);
    return current_obs;
}

pub fn reset() void {
    current_mutex.lockUncancelable(std.Options.debug_io);
    const obs = current_obs;
    current_obs = null;
    current_mutex.unlock(std.Options.debug_io);
    if (obs) |ptr| {
        destroy(ptr);
    }
}

pub fn destroy(obs: *Observability) void {
    const allocator = obs.allocator;
    obs.deinit();
    allocator.destroy(obs);
}

pub fn deriveRunDir(allocator: std.mem.Allocator, root: []const u8, slug: []const u8, uid: []const u8) ![]u8 {
    if (root.len == 0 or slug.len == 0 or uid.len == 0) return allocator.dupe(u8, root);
    return std.fs.path.join(allocator, &.{ root, slug, uid });
}

pub fn enableDiskWriters(obs: *Observability, run_dir: []const u8) !void {
    if (run_dir.len == 0) return;
    try std.Io.Dir.cwd().createDirPath(std.Options.debug_io, run_dir);
    obs.disk_writers_enabled = true;
}

pub const MetaJson = struct {
    slug: []const u8,
    uid: []const u8,
    pid: i32,
    started_at_ns: i128,
    mode: []const u8,
    transport: []const u8,
    address: []const u8,
    metrics_addr: []const u8 = "",
    log_path: []const u8 = "",
    log_bytes_rotated: i64 = 0,
    organism_uid: []const u8 = "",
    organism_slug: []const u8 = "",
    is_default: bool = false,
};

pub fn writeMetaJson(allocator: std.mem.Allocator, run_dir: []const u8, meta: MetaJson) !void {
    const cwd = std.Io.Dir.cwd();
    try cwd.createDirPath(std.Options.debug_io, run_dir);
    var body: std.ArrayList(u8) = .empty;
    defer body.deinit(allocator);
    const started_at = try rfc3339Alloc(allocator, meta.started_at_ns);
    defer allocator.free(started_at);
    try body.append(allocator, '{');
    try appendJsonField(&body, allocator, "slug", meta.slug, false);
    try appendJsonField(&body, allocator, "uid", meta.uid, true);
    try appendRawField(&body, allocator, "pid", try std.fmt.allocPrint(allocator, "{d}", .{meta.pid}), true);
    try appendJsonField(&body, allocator, "started_at", started_at, true);
    try appendJsonField(&body, allocator, "mode", meta.mode, true);
    try appendJsonField(&body, allocator, "transport", meta.transport, true);
    try appendJsonField(&body, allocator, "address", meta.address, true);
    if (meta.metrics_addr.len != 0) try appendJsonField(&body, allocator, "metrics_addr", meta.metrics_addr, true);
    if (meta.log_path.len != 0) try appendJsonField(&body, allocator, "log_path", meta.log_path, true);
    if (meta.log_bytes_rotated > 0) try appendRawField(&body, allocator, "log_bytes_rotated", try std.fmt.allocPrint(allocator, "{d}", .{meta.log_bytes_rotated}), true);
    if (meta.organism_uid.len != 0) try appendJsonField(&body, allocator, "organism_uid", meta.organism_uid, true);
    if (meta.organism_slug.len != 0) try appendJsonField(&body, allocator, "organism_slug", meta.organism_slug, true);
    if (meta.is_default) try appendRawField(&body, allocator, "default", try allocator.dupe(u8, "true"), true);
    try body.append(allocator, '}');

    const tmp = try std.fs.path.join(allocator, &.{ run_dir, "meta.json.tmp" });
    defer allocator.free(tmp);
    const path = try std.fs.path.join(allocator, &.{ run_dir, "meta.json" });
    defer allocator.free(path);
    try cwd.writeFile(std.Options.debug_io, .{ .sub_path = tmp, .data = body.items });
    try cwd.rename(tmp, cwd, path, std.Options.debug_io);
}

fn logRecordToJson(allocator: std.mem.Allocator, entry: LogRecord) ![]u8 {
    var body: std.ArrayList(u8) = .empty;
    errdefer body.deinit(allocator);
    const timestamp = try rfc3339Alloc(allocator, entry.timestamp_ns);
    defer allocator.free(timestamp);
    try body.append(allocator, '{');
    try appendJsonField(&body, allocator, "kind", "log", false);
    try appendJsonField(&body, allocator, "ts", timestamp, true);
    try appendJsonField(&body, allocator, "level", entry.level.name(), true);
    try appendJsonField(&body, allocator, "slug", entry.slug, true);
    try appendJsonField(&body, allocator, "instance_uid", entry.instance_uid, true);
    try appendJsonField(&body, allocator, "message", entry.message, true);
    if (entry.session_id.len != 0) try appendJsonField(&body, allocator, "session_id", entry.session_id, true);
    if (entry.rpc_method.len != 0) try appendJsonField(&body, allocator, "rpc_method", entry.rpc_method, true);
    if (entry.fields.len != 0) try appendRawField(&body, allocator, "fields", try fieldsToJson(allocator, entry.fields), true);
    if (entry.caller.len != 0) try appendJsonField(&body, allocator, "caller", entry.caller, true);
    if (entry.chain.len != 0) try appendRawField(&body, allocator, "chain", try chainToJson(allocator, entry.chain), true);
    try body.append(allocator, '}');
    return body.toOwnedSlice(allocator);
}

fn eventToJson(allocator: std.mem.Allocator, event: Event) ![]u8 {
    var body: std.ArrayList(u8) = .empty;
    errdefer body.deinit(allocator);
    const timestamp = try rfc3339Alloc(allocator, event.timestamp_ns);
    defer allocator.free(timestamp);
    try body.append(allocator, '{');
    try appendJsonField(&body, allocator, "kind", "event", false);
    try appendJsonField(&body, allocator, "ts", timestamp, true);
    try appendJsonField(&body, allocator, "type", event.event_type.name(), true);
    try appendJsonField(&body, allocator, "slug", event.slug, true);
    try appendJsonField(&body, allocator, "instance_uid", event.instance_uid, true);
    if (event.session_id.len != 0) try appendJsonField(&body, allocator, "session_id", event.session_id, true);
    if (event.payload.len != 0) try appendRawField(&body, allocator, "payload", try fieldsToJson(allocator, event.payload), true);
    if (event.chain.len != 0) try appendRawField(&body, allocator, "chain", try chainToJson(allocator, event.chain), true);
    try body.append(allocator, '}');
    return body.toOwnedSlice(allocator);
}

fn appendJsonLinePath(allocator: std.mem.Allocator, run_dir: []const u8, basename: []const u8, body: []u8) !void {
    defer allocator.free(body);
    const path = try std.fs.path.join(allocator, &.{ run_dir, basename });
    defer allocator.free(path);
    const cwd = std.Io.Dir.cwd();
    const existing = cwd.readFileAlloc(std.Options.debug_io, path, allocator, .limited(64 * 1024 * 1024)) catch |err| switch (err) {
        error.FileNotFound => try allocator.alloc(u8, 0),
        else => return err,
    };
    defer allocator.free(existing);
    var combined = try std.ArrayList(u8).initCapacity(allocator, existing.len + body.len + 1);
    defer combined.deinit(allocator);
    try combined.appendSlice(allocator, existing);
    try combined.appendSlice(allocator, body);
    try combined.append(allocator, '\n');
    try cwd.writeFile(std.Options.debug_io, .{ .sub_path = path, .data = combined.items });
}

fn appendJsonField(out: *std.ArrayList(u8), allocator: std.mem.Allocator, key: []const u8, value: []const u8, comma: bool) !void {
    const escaped = try jsonString(allocator, value);
    defer allocator.free(escaped);
    try appendRawField(out, allocator, key, try allocator.dupe(u8, escaped), comma);
}

fn appendRawField(out: *std.ArrayList(u8), allocator: std.mem.Allocator, key: []const u8, raw_value: []u8, comma: bool) !void {
    defer allocator.free(raw_value);
    if (comma) try out.append(allocator, ',');
    const escaped_key = try jsonString(allocator, key);
    defer allocator.free(escaped_key);
    try out.appendSlice(allocator, escaped_key);
    try out.append(allocator, ':');
    try out.appendSlice(allocator, raw_value);
}

fn labelsToJson(allocator: std.mem.Allocator, labels: []const Label) ![]u8 {
    var body: std.ArrayList(u8) = .empty;
    errdefer body.deinit(allocator);
    try body.append(allocator, '{');
    for (labels, 0..) |label, index| {
        try appendJsonField(&body, allocator, label.key, label.value, index != 0);
    }
    try body.append(allocator, '}');
    return body.toOwnedSlice(allocator);
}

fn fieldsToJson(allocator: std.mem.Allocator, fields: []const Field) ![]u8 {
    var body: std.ArrayList(u8) = .empty;
    errdefer body.deinit(allocator);
    try body.append(allocator, '{');
    for (fields, 0..) |field, index| {
        const raw = switch (field.value) {
            .string_value => |value| try jsonString(allocator, value),
            .bool_value => |value| try allocator.dupe(u8, if (value) "true" else "false"),
            .int_value => |value| try std.fmt.allocPrint(allocator, "{d}", .{value}),
            .double_value => |value| try std.fmt.allocPrint(allocator, "{d}", .{value}),
        };
        try appendRawField(&body, allocator, field.key, raw, index != 0);
    }
    try body.append(allocator, '}');
    return body.toOwnedSlice(allocator);
}

fn chainToJson(allocator: std.mem.Allocator, chain: []const Hop) ![]u8 {
    var body: std.ArrayList(u8) = .empty;
    errdefer body.deinit(allocator);
    try body.append(allocator, '[');
    for (chain, 0..) |hop, index| {
        if (index != 0) try body.append(allocator, ',');
        try body.append(allocator, '{');
        try appendJsonField(&body, allocator, "slug", hop.slug, false);
        try appendJsonField(&body, allocator, "instance_uid", hop.instance_uid, true);
        try body.append(allocator, '}');
    }
    try body.append(allocator, ']');
    return body.toOwnedSlice(allocator);
}

fn jsonString(allocator: std.mem.Allocator, value: []const u8) ![]u8 {
    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);
    try out.append(allocator, '"');
    for (value) |ch| {
        switch (ch) {
            '\\' => try out.appendSlice(allocator, "\\\\"),
            '"' => try out.appendSlice(allocator, "\\\""),
            '\n' => try out.appendSlice(allocator, "\\n"),
            '\r' => try out.appendSlice(allocator, "\\r"),
            '\t' => try out.appendSlice(allocator, "\\t"),
            else => if (ch < 0x20) {
                try out.print(allocator, "\\u{x:0>4}", .{ch});
            } else {
                try out.append(allocator, ch);
            },
        }
    }
    try out.append(allocator, '"');
    return out.toOwnedSlice(allocator);
}

fn rfc3339Alloc(allocator: std.mem.Allocator, ns: i128) ![]u8 {
    const seconds = @as(i64, @intCast(@divFloor(ns, std.time.ns_per_s)));
    const nanos = @as(u32, @intCast(@mod(ns, std.time.ns_per_s)));
    const epoch_seconds = std.time.epoch.EpochSeconds{ .secs = @intCast(seconds) };
    const epoch_day = epoch_seconds.getEpochDay();
    const year_day = epoch_day.calculateYearDay();
    const month_day = year_day.calculateMonthDay();
    const day_seconds = epoch_seconds.getDaySeconds();
    return std.fmt.allocPrint(
        allocator,
        "{d:0>4}-{d:0>2}-{d:0>2}T{d:0>2}:{d:0>2}:{d:0>2}.{d:0>9}Z",
        .{
            year_day.year,
            @intFromEnum(month_day.month),
            month_day.day_index + 1,
            day_seconds.getHoursIntoDay(),
            day_seconds.getMinutesIntoHour(),
            day_seconds.getSecondsIntoMinute(),
            nanos,
        },
    );
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}

fn unixNanoU64(ns: i128) u64 {
    return @intCast(@max(ns, 0));
}

fn newInstanceUid(allocator: std.mem.Allocator) ![]u8 {
    return std.fmt.allocPrint(allocator, "{d}-{d}", .{ std.c.getpid(), nowNs() });
}

fn processEnvOwned(allocator: std.mem.Allocator, key: []const u8) !?[]u8 {
    const key_z = try allocator.dupeZ(u8, key);
    defer allocator.free(key_z);
    const value = std.c.getenv(key_z.ptr) orelse return null;
    return try allocator.dupe(u8, std.mem.span(value));
}

fn processSlug(allocator: std.mem.Allocator) ![]u8 {
    if (std.process.executablePathAlloc(std.Options.debug_io, allocator)) |path| {
        defer allocator.free(path);
        return allocator.dupe(u8, std.fs.path.basename(path));
    } else |_| {
        return allocator.dupe(u8, "");
    }
}

fn knownToken(token: []const u8) bool {
    for (V1_TOKENS) |known| if (std.mem.eql(u8, token, known)) return true;
    return false;
}

fn familyFromString(token: []const u8) ?Family {
    if (std.mem.eql(u8, token, "logs")) return .logs;
    if (std.mem.eql(u8, token, "metrics")) return .metrics;
    if (std.mem.eql(u8, token, "events")) return .events;
    if (std.mem.eql(u8, token, "prom")) return .prom;
    if (std.mem.eql(u8, token, "otel")) return .otel;
    return null;
}

fn envValue(env: []const EnvEntry, key: []const u8) ?[]const u8 {
    for (env) |entry| {
        if (std.mem.eql(u8, entry.key, key)) return entry.value;
    }
    return null;
}

fn metricMatches(name: []const u8, existing_labels: []const Label, requested_name: []const u8, requested_labels: []const Label) bool {
    if (!std.mem.eql(u8, name, requested_name) or existing_labels.len != requested_labels.len) return false;
    for (existing_labels, requested_labels) |left, right| {
        if (!std.mem.eql(u8, left.key, right.key) or !std.mem.eql(u8, left.value, right.value)) return false;
    }
    return true;
}

fn redactLabels(allocator: std.mem.Allocator, labels: []const Label, redacted_fields: []const []const u8) ![]Label {
    var out = try allocator.alloc(Label, labels.len);
    errdefer freeLabels(allocator, out);
    for (labels, 0..) |label, index| {
        out[index] = .{
            .key = try allocator.dupe(u8, label.key),
            .value = try allocator.dupe(u8, if (containsString(redacted_fields, label.key)) "<redacted>" else label.value),
        };
    }
    return out;
}

fn redactFields(allocator: std.mem.Allocator, fields: []const Field, redacted_fields: []const []const u8) ![]Field {
    var out = try allocator.alloc(Field, fields.len);
    errdefer freeFields(allocator, out);
    for (fields, 0..) |field, index| {
        out[index] = .{
            .key = try allocator.dupe(u8, field.key),
            .value = if (containsString(redacted_fields, field.key))
                .{ .string_value = try allocator.dupe(u8, "<redacted>") }
            else
                try AnyValue.clone(allocator, field.value),
        };
    }
    return out;
}

fn containsString(values: []const []const u8, needle: []const u8) bool {
    for (values) |value| if (std.mem.eql(u8, value, needle)) return true;
    return false;
}

fn cloneLabels(allocator: std.mem.Allocator, labels: []const Label) ![]Label {
    var out = try allocator.alloc(Label, labels.len);
    errdefer freeLabels(allocator, out);
    for (labels, 0..) |label, index| {
        out[index] = .{
            .key = try allocator.dupe(u8, label.key),
            .value = try allocator.dupe(u8, label.value),
        };
    }
    return out;
}

fn cloneFields(allocator: std.mem.Allocator, fields: []const Field) ![]Field {
    var out = try allocator.alloc(Field, fields.len);
    errdefer freeFields(allocator, out);
    for (fields, 0..) |field, index| {
        out[index] = .{
            .key = try allocator.dupe(u8, field.key),
            .value = try AnyValue.clone(allocator, field.value),
        };
    }
    return out;
}

fn freeLabels(allocator: std.mem.Allocator, labels: []Label) void {
    for (labels) |label| {
        allocator.free(label.key);
        allocator.free(label.value);
    }
    allocator.free(labels);
}

fn freeFields(allocator: std.mem.Allocator, fields: []Field) void {
    for (fields) |*field| {
        allocator.free(field.key);
        field.value.deinit(allocator);
    }
    allocator.free(fields);
}

fn cloneHops(allocator: std.mem.Allocator, hops: []const Hop) ![]Hop {
    var out = try allocator.alloc(Hop, hops.len);
    errdefer freeHops(allocator, out);
    for (hops, 0..) |hop, index| {
        out[index] = .{
            .slug = try allocator.dupe(u8, hop.slug),
            .instance_uid = try allocator.dupe(u8, hop.instance_uid),
        };
    }
    return out;
}

fn freeHops(allocator: std.mem.Allocator, hops: []Hop) void {
    for (hops) |*hop| hop.deinit(allocator);
    allocator.free(hops);
}

fn dupeStringSlice(allocator: std.mem.Allocator, values: []const []const u8) ![]const []const u8 {
    var out = try allocator.alloc([]const u8, values.len);
    errdefer allocator.free(out);
    for (values, 0..) |value, index| out[index] = try allocator.dupe(u8, value);
    return out;
}

fn freeStringSlice(allocator: std.mem.Allocator, values: []const []const u8) void {
    for (values) |value| allocator.free(value);
    allocator.free(values);
}

test "parse op obs rejects reserved and unknown tokens" {
    try std.testing.expectEqual(@as(usize, 0), (try parseOpObs("")).count());
    try std.testing.expect((try parseOpObs("logs,metrics")).contains(.logs));
    try std.testing.expect((try parseOpObs("all")).contains(.prom));
    try std.testing.expectError(error.ReservedObservabilityToken, parseOpObs("logs,otel"));
    try std.testing.expectError(error.ReservedObservabilityToken, parseOpObs("sessions"));
    try std.testing.expectError(error.UnknownObservabilityToken, parseOpObs("bogus"));
}
