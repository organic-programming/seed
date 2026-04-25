const std = @import("std");

pub const Scope = enum {
    siblings,
    cwd,
    source,
    built,
    installed,
    cached,
};

pub const HolonRef = struct {
    slug: []const u8,
    path: []const u8,
    scope: Scope,
};

pub const Result = struct {
    matches: []HolonRef,
};

pub const Error = error{
    NotFound,
};

pub fn resolveSourcePath(allocator: std.mem.Allocator, root: []const u8, slug: []const u8) ![]u8 {
    return std.fs.path.join(allocator, &.{ root, "examples", "hello-world", slug });
}

pub fn findBySlug(allocator: std.mem.Allocator, root: []const u8, slug: []const u8) !HolonRef {
    const path = try resolveSourcePath(allocator, root, slug);
    errdefer allocator.free(path);
    std.fs.cwd().access(path, .{}) catch |err| switch (err) {
        error.FileNotFound => return error.NotFound,
        else => return err,
    };
    return .{ .slug = slug, .path = path, .scope = .source };
}

test "source path follows hello-world layout" {
    const path = try resolveSourcePath(std.testing.allocator, "/repo", "gabriel-greeting-zig");
    defer std.testing.allocator.free(path);
    try std.testing.expect(std.mem.endsWith(u8, path, "examples/hello-world/gabriel-greeting-zig"));
}
