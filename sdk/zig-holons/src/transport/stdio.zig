const std = @import("std");
const uri = @import("uri.zig");

pub const Connection = struct {
    endpoint: uri.Endpoint,
    owns_child: bool = false,
};

pub const Command = struct {
    argv: []const [:0]const u8,
    cwd: ?[:0]const u8 = null,
};

pub const Child = struct {
    pid: c.pid_t,
    socket_fd: c_int,

    pub fn close(self: *Child) void {
        _ = c.close(self.socket_fd);
        if (self.pid > 0) {
            _ = c.kill(self.pid, c.SIGTERM);
            var status: c_int = 0;
            _ = c.waitpid(self.pid, &status, 0);
            self.pid = 0;
        }
    }
};

pub const ServerBridge = struct {
    socket_fd: c_int,

    pub fn close(self: *ServerBridge) void {
        _ = c.close(self.socket_fd);
    }
};

const c = @cImport({
    @cInclude("fcntl.h");
    @cInclude("signal.h");
    @cInclude("stdlib.h");
    @cInclude("sys/socket.h");
    @cInclude("sys/wait.h");
    @cInclude("unistd.h");
});

pub fn validate(raw: []const u8) uri.ParseError!uri.Endpoint {
    const endpoint = try uri.parse(raw);
    if (endpoint.scheme != .stdio) return error.UnsupportedScheme;
    return endpoint;
}

pub fn connect(raw: []const u8) uri.ParseError!Connection {
    return .{ .endpoint = try validate(raw), .owns_child = false };
}

pub fn spawnCommand(allocator: std.mem.Allocator, command: Command) !Child {
    if (command.argv.len == 0) return error.EmptyCommand;

    var child_stdin: [2]c_int = undefined;
    var child_stdout: [2]c_int = undefined;
    var sockets: [2]c_int = undefined;
    if (c.pipe(&child_stdin) != 0) return error.PipeFailed;
    errdefer {
        _ = c.close(child_stdin[0]);
        _ = c.close(child_stdin[1]);
    }
    if (c.pipe(&child_stdout) != 0) return error.PipeFailed;
    errdefer {
        _ = c.close(child_stdout[0]);
        _ = c.close(child_stdout[1]);
    }
    if (c.socketpair(c.AF_UNIX, c.SOCK_STREAM, 0, &sockets) != 0) return error.SocketpairFailed;
    errdefer {
        _ = c.close(sockets[0]);
        _ = c.close(sockets[1]);
    }

    const argv = try allocator.alloc(?[*:0]const u8, command.argv.len + 1);
    defer allocator.free(argv);
    for (command.argv, 0..) |arg, index| argv[index] = arg.ptr;
    argv[command.argv.len] = null;

    const pid = c.fork();
    if (pid < 0) return error.ForkFailed;
    if (pid == 0) {
        if (command.cwd) |cwd| {
            if (c.chdir(cwd.ptr) != 0) c._exit(126);
        }
        _ = c.dup2(child_stdin[0], 0);
        _ = c.dup2(child_stdout[1], 1);
        _ = c.close(child_stdin[0]);
        _ = c.close(child_stdin[1]);
        _ = c.close(child_stdout[0]);
        _ = c.close(child_stdout[1]);
        _ = c.close(sockets[0]);
        _ = c.close(sockets[1]);
        _ = c.execvp(argv[0].?, @ptrCast(argv.ptr));
        c._exit(127);
    }

    _ = c.close(child_stdin[0]);
    _ = c.close(child_stdout[1]);

    const input_thread = try std.Thread.spawn(.{}, pump, .{ sockets[1], child_stdin[1], false });
    input_thread.detach();
    const output_thread = try std.Thread.spawn(.{}, pump, .{ child_stdout[0], sockets[1], true });
    output_thread.detach();

    return .{ .pid = pid, .socket_fd = sockets[0] };
}

pub fn openServerBridge() !ServerBridge {
    var sockets: [2]c_int = undefined;
    if (c.socketpair(c.AF_UNIX, c.SOCK_STREAM, 0, &sockets) != 0) return error.SocketpairFailed;
    errdefer {
        _ = c.close(sockets[0]);
        _ = c.close(sockets[1]);
    }
    try setNonBlock(sockets[0]);

    const input_thread = try std.Thread.spawn(.{}, pump, .{ 0, sockets[1], true });
    input_thread.detach();
    const output_thread = try std.Thread.spawn(.{}, pump, .{ sockets[1], 1, false });
    output_thread.detach();

    return .{ .socket_fd = sockets[0] };
}

fn setNonBlock(fd: c_int) !void {
    const flags = c.fcntl(fd, c.F_GETFL, @as(c_int, 0));
    if (flags < 0) return error.FcntlGetFailed;
    if (c.fcntl(fd, c.F_SETFL, flags | c.O_NONBLOCK) != 0) return error.FcntlSetFailed;
}

fn writeAll(fd: c_int, bytes: []const u8) !void {
    var offset: usize = 0;
    while (offset < bytes.len) {
        const n = c.write(fd, bytes.ptr + offset, bytes.len - offset);
        if (n < 0) return error.WriteFailed;
        if (n == 0) return error.WriteZero;
        offset += @intCast(n);
    }
}

fn pump(reader_fd: c_int, writer_fd: c_int, close_writer_as_socket: bool) void {
    var buf: [8192]u8 = undefined;
    while (true) {
        const n = c.read(reader_fd, &buf, buf.len);
        if (n <= 0) break;
        writeAll(writer_fd, buf[0..@intCast(n)]) catch break;
    }
    if (close_writer_as_socket) {
        _ = c.shutdown(writer_fd, c.SHUT_WR);
    }
    _ = c.close(writer_fd);
    _ = c.close(reader_fd);
}
