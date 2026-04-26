const std = @import("std");

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
    debug = 2,
    info = 3,
    warn = 4,
    err = 5,
    fatal = 6,

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
};

pub const Label = struct {
    key: []const u8,
    value: []const u8,
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

pub const LogEntry = struct {
    timestamp_ns: i128 = 0,
    level: Level = .info,
    slug: []const u8 = "",
    instance_uid: []const u8 = "",
    session_id: []const u8 = "",
    rpc_method: []const u8 = "",
    message: []const u8 = "",
    fields: []Label = &.{},
    caller: []const u8 = "",
    chain: []Hop = &.{},

    pub fn clone(allocator: std.mem.Allocator, entry: LogEntry) !LogEntry {
        return .{
            .timestamp_ns = entry.timestamp_ns,
            .level = entry.level,
            .slug = try allocator.dupe(u8, entry.slug),
            .instance_uid = try allocator.dupe(u8, entry.instance_uid),
            .session_id = try allocator.dupe(u8, entry.session_id),
            .rpc_method = try allocator.dupe(u8, entry.rpc_method),
            .message = try allocator.dupe(u8, entry.message),
            .fields = try cloneLabels(allocator, entry.fields),
            .caller = try allocator.dupe(u8, entry.caller),
            .chain = try cloneHops(allocator, entry.chain),
        };
    }

    pub fn deinit(self: *LogEntry, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.instance_uid);
        allocator.free(self.session_id);
        allocator.free(self.rpc_method);
        allocator.free(self.message);
        freeLabels(allocator, self.fields);
        allocator.free(self.caller);
        freeHops(allocator, self.chain);
        self.* = .{};
    }
};

pub const LogRing = struct {
    allocator: std.mem.Allocator,
    capacity: usize,
    inner: std.ArrayList(LogEntry),
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

    pub fn push(self: *LogRing, entry: LogEntry) !void {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        if (self.inner.items.len == self.capacity) {
            var old = self.inner.orderedRemove(0);
            old.deinit(self.allocator);
        }
        try self.inner.append(self.allocator, try LogEntry.clone(self.allocator, entry));
    }

    pub fn drain(self: *LogRing, allocator: std.mem.Allocator) ![]LogEntry {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        var out = try allocator.alloc(LogEntry, self.inner.items.len);
        errdefer allocator.free(out);
        for (self.inner.items, 0..) |entry, index| out[index] = try LogEntry.clone(allocator, entry);
        return out;
    }

    pub fn drainSince(self: *LogRing, allocator: std.mem.Allocator, cutoff_ns: i128) ![]LogEntry {
        self.mutex.lockUncancelable(std.Options.debug_io);
        defer self.mutex.unlock(std.Options.debug_io);
        var out: std.ArrayList(LogEntry) = .empty;
        errdefer {
            for (out.items) |*entry| entry.deinit(allocator);
            out.deinit(allocator);
        }
        for (self.inner.items) |entry| {
            if (entry.timestamp_ns >= cutoff_ns) try out.append(allocator, try LogEntry.clone(allocator, entry));
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
    timestamp_ns: i128 = 0,
    event_type: EventType = .unspecified,
    slug: []const u8 = "",
    instance_uid: []const u8 = "",
    session_id: []const u8 = "",
    payload: []Label = &.{},
    chain: []Hop = &.{},

    pub fn clone(allocator: std.mem.Allocator, event: Event) !Event {
        return .{
            .timestamp_ns = event.timestamp_ns,
            .event_type = event.event_type,
            .slug = try allocator.dupe(u8, event.slug),
            .instance_uid = try allocator.dupe(u8, event.instance_uid),
            .session_id = try allocator.dupe(u8, event.session_id),
            .payload = try cloneLabels(allocator, event.payload),
            .chain = try cloneHops(allocator, event.chain),
        };
    }

    pub fn deinit(self: *Event, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.instance_uid);
        allocator.free(self.session_id);
        freeLabels(allocator, self.payload);
        freeHops(allocator, self.chain);
        self.* = .{};
    }
};

pub const EventBus = struct {
    allocator: std.mem.Allocator,
    capacity: usize,
    inner: std.ArrayList(Event),
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
        try self.inner.append(self.allocator, try Event.clone(self.allocator, event));
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

    pub fn trace(self: Logger, message: []const u8, fields: []const Label) !void {
        try self.log(.trace, message, fields);
    }

    pub fn debug(self: Logger, message: []const u8, fields: []const Label) !void {
        try self.log(.debug, message, fields);
    }

    pub fn info(self: Logger, message: []const u8, fields: []const Label) !void {
        try self.log(.info, message, fields);
    }

    pub fn warn(self: Logger, message: []const u8, fields: []const Label) !void {
        try self.log(.warn, message, fields);
    }

    pub fn err(self: Logger, message: []const u8, fields: []const Label) !void {
        try self.log(.err, message, fields);
    }

    pub fn fatal(self: Logger, message: []const u8, fields: []const Label) !void {
        try self.log(.fatal, message, fields);
    }

    fn log(self: Logger, level: Level, message: []const u8, fields: []const Label) !void {
        if (!self.enabled(level) or !self.obs.enabled(.logs)) return;
        const redacted = try redactLabels(self.obs.allocator, fields, self.obs.cfg.redacted_fields);
        defer freeLabels(self.obs.allocator, redacted);
        const entry = LogEntry{
            .timestamp_ns = nowNs(),
            .level = level,
            .slug = self.obs.cfg.slug,
            .instance_uid = self.obs.cfg.instance_uid,
            .message = message,
            .fields = redacted,
        };
        try self.obs.logEntry(entry);
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

    pub fn emit(self: *Observability, event_type: EventType, payload: []const Label) !void {
        if (!self.enabled(.events)) return;
        const redacted = try redactLabels(self.allocator, payload, self.cfg.redacted_fields);
        defer freeLabels(self.allocator, redacted);
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

    fn logEntry(self: *Observability, entry: LogEntry) !void {
        if (self.log_ring) |*ring| try ring.push(entry);
        if (self.disk_writers_enabled and self.cfg.run_dir.len != 0) {
            try appendJsonLinePath(self.allocator, self.cfg.run_dir, "stdout.log", try logEntryToJson(self.allocator, entry));
        }
    }

    pub fn close(self: *Observability) void {
        if (self.event_bus) |*bus| bus.close();
    }
};

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

fn configureFromRaw(allocator: std.mem.Allocator, cfg: Config, op_obs: []const u8) !*Observability {
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
    reset();
    current_mutex.lockUncancelable(std.Options.debug_io);
    current_obs = obs;
    current_mutex.unlock(std.Options.debug_io);
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
        const allocator = ptr.allocator;
        ptr.deinit();
        allocator.destroy(ptr);
    }
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

fn logEntryToJson(allocator: std.mem.Allocator, entry: LogEntry) ![]u8 {
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
    if (entry.fields.len != 0) try appendRawField(&body, allocator, "fields", try labelsToJson(allocator, entry.fields), true);
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
    if (event.payload.len != 0) try appendRawField(&body, allocator, "payload", try labelsToJson(allocator, event.payload), true);
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

fn newInstanceUid(allocator: std.mem.Allocator) ![]u8 {
    return std.fmt.allocPrint(allocator, "{d}-{d}", .{ std.c.getpid(), nowNs() });
}

fn processEnvOwned(allocator: std.mem.Allocator, key: []const u8) !?[]u8 {
    const key_z = try allocator.dupeZ(u8, key);
    defer allocator.free(key_z);
    const value = std.c.getenv(key_z.ptr) orelse return null;
    return allocator.dupe(u8, std.mem.span(value));
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

fn freeLabels(allocator: std.mem.Allocator, labels: []Label) void {
    for (labels) |label| {
        allocator.free(label.key);
        allocator.free(label.value);
    }
    allocator.free(labels);
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
