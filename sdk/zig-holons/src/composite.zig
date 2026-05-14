const std = @import("std");

pub const Error = error{
    ExecutablePathRequired,
    MemberDirectoryNotFound,
    MemberIdRequired,
    NoExecutableFound,
};

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
