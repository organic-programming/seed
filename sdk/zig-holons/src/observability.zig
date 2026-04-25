const std = @import("std");

pub const Level = enum { debug, info, warn, err };

pub const Event = struct {
    name: []const u8,
    chain_id: []const u8 = "",
    message: []const u8 = "",
};

pub const Counter = struct {
    name: []const u8,
    value: u64 = 0,

    pub fn inc(self: *Counter, amount: u64) void {
        self.value += amount;
    }
};

pub const Histogram = struct {
    name: []const u8,
    count: u64 = 0,
    sum: f64 = 0,

    pub fn observe(self: *Histogram, value: f64) void {
        self.count += 1;
        self.sum += value;
    }
};

pub const HolonObservability = struct {
    allocator: std.mem.Allocator,
    events: std.ArrayList(Event),

    pub fn init(allocator: std.mem.Allocator) HolonObservability {
        return .{ .allocator = allocator, .events = .empty };
    }

    pub fn deinit(self: *HolonObservability) void {
        self.events.deinit(self.allocator);
    }

    pub fn emit(self: *HolonObservability, event: Event) !void {
        try self.events.append(self.allocator, event);
    }
};

test "counter increments" {
    var counter = Counter{ .name = "requests" };
    counter.inc(2);
    try std.testing.expectEqual(@as(u64, 2), counter.value);
}
