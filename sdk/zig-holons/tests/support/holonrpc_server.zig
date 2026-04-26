const std = @import("std");

const c = @cImport({
    @cInclude("signal.h");
    @cInclude("stdlib.h");
    @cInclude("sys/wait.h");
    @cInclude("unistd.h");
});

pub const Ready = struct {
    url: []u8,
    ca_file: ?[]u8 = null,

    pub fn deinit(self: *Ready, allocator: std.mem.Allocator) void {
        allocator.free(self.url);
        if (self.ca_file) |ca_file| allocator.free(ca_file);
        self.* = undefined;
    }
};

pub const Process = struct {
    pid: c.pid_t,
    ready: Ready,

    pub fn stop(self: *Process, allocator: std.mem.Allocator) void {
        self.ready.deinit(allocator);
        if (self.pid <= 0) return;
        var status: c_int = 0;
        _ = c.kill(-self.pid, c.SIGTERM);
        var attempt: u32 = 0;
        while (attempt < 20) : (attempt += 1) {
            const waited = c.waitpid(self.pid, &status, c.WNOHANG);
            if (waited == self.pid) {
                self.pid = 0;
                return;
            }
            if (waited < 0) break;
            _ = c.usleep(100_000);
        }
        _ = c.kill(-self.pid, c.SIGKILL);
        _ = c.waitpid(self.pid, &status, 0);
        self.pid = 0;
    }
};

pub fn start(allocator: std.mem.Allocator, mode: []const u8) !Process {
    const dir = try helperDir(allocator);
    defer allocator.free(dir);

    const mode_z = try allocator.dupeZ(u8, mode);
    defer allocator.free(mode_z);

    const binary_name = try buildHelper(allocator, dir, mode);
    defer allocator.free(binary_name);

    const exec_name_bytes = try std.fmt.allocPrint(allocator, "./{s}", .{binary_name});
    defer allocator.free(exec_name_bytes);
    const exec_name = try allocator.dupeZ(u8, exec_name_bytes);
    defer allocator.free(exec_name);

    var pipe_fds: [2]c_int = undefined;
    if (c.pipe(&pipe_fds) != 0) return error.PipeFailed;
    errdefer {
        _ = c.close(pipe_fds[0]);
        _ = c.close(pipe_fds[1]);
    }

    const argv = [_:null]?[*:0]const u8{
        exec_name.ptr,
        "--mode",
        mode_z.ptr,
    };

    const pid = c.fork();
    if (pid < 0) return error.ForkFailed;
    if (pid == 0) {
        _ = c.setpgid(0, 0);
        _ = c.close(pipe_fds[0]);
        if (c.dup2(pipe_fds[1], c.STDOUT_FILENO) < 0) c._exit(126);
        _ = c.close(pipe_fds[1]);
        if (c.chdir(dir.ptr) != 0) c._exit(126);
        _ = c.execvp(argv[0].?, @ptrCast(&argv));
        c._exit(127);
    }
    _ = c.setpgid(pid, pid);
    _ = c.close(pipe_fds[1]);

    const line = readLineAlloc(allocator, pipe_fds[0]) catch |err| {
        _ = c.close(pipe_fds[0]);
        var process: Process = .{
            .pid = pid,
            .ready = .{ .url = try allocator.dupe(u8, "") },
        };
        process.stop(allocator);
        return err;
    };
    _ = c.close(pipe_fds[0]);
    defer allocator.free(line);

    var parsed = try std.json.parseFromSlice(std.json.Value, allocator, line, .{});
    defer parsed.deinit();
    const root = parsed.value.object;
    const url = root.get("url") orelse return error.ReadyUrlMissing;
    if (url != .string) return error.ReadyUrlInvalid;

    var ca_file: ?[]u8 = null;
    if (root.get("ca_file")) |ca| {
        if (ca == .string and ca.string.len > 0) {
            ca_file = try allocator.dupe(u8, ca.string);
        }
    }

    return .{
        .pid = pid,
        .ready = .{
            .url = try allocator.dupe(u8, url.string),
            .ca_file = ca_file,
        },
    };
}

fn buildHelper(allocator: std.mem.Allocator, dir: [:0]const u8, mode: []const u8) ![:0]u8 {
    const binary_name_bytes = try std.fmt.allocPrint(
        allocator,
        ".zig-holonrpc-helper-{d}-{s}",
        .{ c.getpid(), mode },
    );
    defer allocator.free(binary_name_bytes);

    const binary_name = try allocator.dupeZ(u8, binary_name_bytes);
    errdefer allocator.free(binary_name);

    const argv = [_:null]?[*:0]const u8{
        "env",
        "GOWORK=off",
        "GOFLAGS=-mod=mod",
        "go",
        "build",
        "-o",
        binary_name.ptr,
        ".",
    };

    const pid = c.fork();
    if (pid < 0) return error.ForkFailed;
    if (pid == 0) {
        if (c.chdir(dir.ptr) != 0) c._exit(126);
        _ = c.execvp(argv[0].?, @ptrCast(&argv));
        c._exit(127);
    }

    var status: c_int = 0;
    if (c.waitpid(pid, &status, 0) != pid) return error.WaitFailed;
    if (status != 0) return error.GoBuildFailed;
    return binary_name;
}

fn helperDir(allocator: std.mem.Allocator) ![:0]u8 {
    const candidates = [_][]const u8{
        "tests/support/holonrpc_server",
        "sdk/zig-holons/tests/support/holonrpc_server",
    };
    for (candidates) |candidate| {
        const marker = try std.fs.path.join(allocator, &.{ candidate, "go.mod" });
        defer allocator.free(marker);
        std.Io.Dir.cwd().access(std.testing.io, marker, .{}) catch continue;
        return allocator.dupeZ(u8, candidate);
    }
    return error.HolonRPCHelperNotFound;
}

fn readLineAlloc(allocator: std.mem.Allocator, fd: c_int) ![]u8 {
    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);
    var byte: [1]u8 = undefined;
    while (true) {
        const n = c.read(fd, &byte, 1);
        if (n < 0) return error.ReadFailed;
        if (n == 0) return error.UnexpectedEof;
        if (byte[0] == '\n') break;
        try out.append(allocator, byte[0]);
        if (out.items.len > 8192) return error.LineTooLong;
    }
    return out.toOwnedSlice(allocator);
}
