const std = @import("std");
const builtin = @import("builtin");
const uri = @import("uri.zig");

const is_windows = builtin.os.tag == .windows;

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
    pumps: ?*PumpGroup = null,

    pub fn close(self: *Child) void {
        if (is_windows) return;
        closeFd(&self.socket_fd);
        if (self.pumps) |pumps| {
            pumps.close();
            self.pumps = null;
        }
        if (self.pid > 0) {
            terminateChild(&self.pid);
            self.pid = 0;
        }
    }
};

pub const ServerBridge = struct {
    socket_fd: c_int,
    pumps: ?*PumpGroup = null,

    pub fn close(self: *ServerBridge) void {
        if (is_windows) return;
        closeFd(&self.socket_fd);
        if (self.pumps) |pumps| {
            pumps.close();
            self.pumps = null;
        }
    }
};

const c = if (is_windows) struct {
    const pid_t = c_int;
} else @cImport({
    @cInclude("fcntl.h");
    @cInclude("poll.h");
    @cInclude("signal.h");
    @cInclude("stdlib.h");
    @cInclude("sys/socket.h");
    @cInclude("sys/wait.h");
    @cInclude("unistd.h");
});

const PUMP_POLL_MS = 50;
const CHILD_TERM_TIMEOUT_MS = 10_000;

const PumpControl = struct {
    stop_requested: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),
};

const PumpArgs = struct {
    control: *PumpControl,
    reader_fd: c_int,
    writer_fd: c_int,
    close_writer_as_socket: bool,
};

const PumpGroup = struct {
    allocator: std.mem.Allocator,
    control: *PumpControl,
    input_args: *PumpArgs,
    output_args: *PumpArgs,
    input_thread: ?std.Thread = null,
    output_thread: ?std.Thread = null,

    fn init(
        allocator: std.mem.Allocator,
        input_args: PumpArgs,
        output_args: PumpArgs,
    ) !*PumpGroup {
        const control = try allocator.create(PumpControl);
        errdefer allocator.destroy(control);
        control.* = .{};

        const group = try allocator.create(PumpGroup);
        errdefer allocator.destroy(group);
        const input = try allocator.create(PumpArgs);
        errdefer allocator.destroy(input);
        const output = try allocator.create(PumpArgs);
        errdefer allocator.destroy(output);

        input.* = input_args;
        input.control = control;
        output.* = output_args;
        output.control = control;
        group.* = .{
            .allocator = allocator,
            .control = control,
            .input_args = input,
            .output_args = output,
        };
        errdefer group.close();

        group.input_thread = try std.Thread.spawn(.{}, pump, .{input});
        group.output_thread = try std.Thread.spawn(.{}, pump, .{output});
        return group;
    }

    fn close(self: *PumpGroup) void {
        self.control.stop_requested.store(true, .release);
        if (self.input_thread) |thread| {
            thread.join();
            self.input_thread = null;
        } else {
            closeFd(&self.input_args.reader_fd);
            closeFd(&self.input_args.writer_fd);
        }
        if (self.output_thread) |thread| {
            thread.join();
            self.output_thread = null;
        } else {
            closeFd(&self.output_args.reader_fd);
            closeFd(&self.output_args.writer_fd);
        }
        const allocator = self.allocator;
        allocator.destroy(self.input_args);
        allocator.destroy(self.output_args);
        allocator.destroy(self.control);
        allocator.destroy(self);
    }
};

pub fn validate(raw: []const u8) uri.ParseError!uri.Endpoint {
    const endpoint = try uri.parse(raw);
    if (endpoint.scheme != .stdio) return error.UnsupportedScheme;
    return endpoint;
}

pub fn connect(raw: []const u8) uri.ParseError!Connection {
    return .{ .endpoint = try validate(raw), .owns_child = false };
}

pub fn spawnCommand(allocator: std.mem.Allocator, command: Command) !Child {
    if (is_windows) return error.UnsupportedStdioTransport;
    if (command.argv.len == 0) return error.EmptyCommand;
    ignoreSigpipe();

    var child_stdin: [2]c_int = undefined;
    var child_stdout: [2]c_int = undefined;
    var sockets: [2]c_int = undefined;
    if (c.pipe(&child_stdin) != 0) return error.PipeFailed;
    errdefer {
        closeFd(&child_stdin[0]);
        closeFd(&child_stdin[1]);
    }
    if (c.pipe(&child_stdout) != 0) return error.PipeFailed;
    errdefer {
        closeFd(&child_stdout[0]);
        closeFd(&child_stdout[1]);
    }
    if (c.socketpair(c.AF_UNIX, c.SOCK_STREAM, 0, &sockets) != 0) return error.SocketpairFailed;
    errdefer {
        closeFd(&sockets[0]);
        closeFd(&sockets[1]);
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
        closeInheritedFds();
        _ = c.execvp(argv[0].?, @ptrCast(argv.ptr));
        c._exit(127);
    }

    closeFd(&child_stdin[0]);
    closeFd(&child_stdout[1]);

    const socket_reader = c.dup(sockets[1]);
    if (socket_reader < 0) return error.DupFailed;
    errdefer _ = c.close(socket_reader);
    const socket_writer = c.dup(sockets[1]);
    if (socket_writer < 0) return error.DupFailed;
    errdefer _ = c.close(socket_writer);
    closeFd(&sockets[1]);

    const pumps = try PumpGroup.init(
        allocator,
        .{
            .control = undefined,
            .reader_fd = socket_reader,
            .writer_fd = child_stdin[1],
            .close_writer_as_socket = false,
        },
        .{
            .control = undefined,
            .reader_fd = child_stdout[0],
            .writer_fd = socket_writer,
            .close_writer_as_socket = true,
        },
    );

    return .{ .pid = pid, .socket_fd = sockets[0], .pumps = pumps };
}

pub fn openServerBridge() !ServerBridge {
    if (is_windows) return error.UnsupportedStdioTransport;
    ignoreSigpipe();
    var sockets: [2]c_int = undefined;
    if (c.socketpair(c.AF_UNIX, c.SOCK_STREAM, 0, &sockets) != 0) return error.SocketpairFailed;
    errdefer {
        closeFd(&sockets[0]);
        closeFd(&sockets[1]);
    }
    try setNonBlock(sockets[0]);

    const socket_reader = c.dup(sockets[1]);
    if (socket_reader < 0) return error.DupFailed;
    errdefer _ = c.close(socket_reader);
    const socket_writer = c.dup(sockets[1]);
    if (socket_writer < 0) return error.DupFailed;
    errdefer _ = c.close(socket_writer);
    closeFd(&sockets[1]);

    const pumps = try PumpGroup.init(
        std.heap.c_allocator,
        .{
            .control = undefined,
            .reader_fd = 0,
            .writer_fd = socket_writer,
            .close_writer_as_socket = true,
        },
        .{
            .control = undefined,
            .reader_fd = socket_reader,
            .writer_fd = 1,
            .close_writer_as_socket = false,
        },
    );

    return .{ .socket_fd = sockets[0], .pumps = pumps };
}

fn setNonBlock(fd: c_int) !void {
    if (is_windows) return error.UnsupportedStdioTransport;
    const flags = c.fcntl(fd, c.F_GETFL, @as(c_int, 0));
    if (flags < 0) return error.FcntlGetFailed;
    if (c.fcntl(fd, c.F_SETFL, flags | c.O_NONBLOCK) != 0) return error.FcntlSetFailed;
}

fn writeAll(control: *PumpControl, fd: c_int, bytes: []const u8) !void {
    if (is_windows) return error.UnsupportedStdioTransport;
    var offset: usize = 0;
    while (offset < bytes.len) {
        if (control.stop_requested.load(.acquire) and !waitWritable(fd, 0)) return error.WriteStopped;
        if (!waitWritable(fd, PUMP_POLL_MS)) continue;
        const chunk = @min(bytes.len - offset, 4096);
        const n = c.write(fd, bytes.ptr + offset, chunk);
        if (n < 0) return error.WriteFailed;
        if (n == 0) return error.WriteZero;
        offset += @intCast(n);
    }
}

fn pump(args: *PumpArgs) void {
    if (is_windows) return;
    var buf: [8192]u8 = undefined;
    while (true) {
        if (args.control.stop_requested.load(.acquire) and !waitReadable(args.reader_fd, 0)) break;
        if (!waitReadable(args.reader_fd, PUMP_POLL_MS)) continue;
        const n = c.read(args.reader_fd, &buf, buf.len);
        if (n <= 0) break;
        writeAll(args.control, args.writer_fd, buf[0..@intCast(n)]) catch break;
    }
    if (args.close_writer_as_socket and args.writer_fd >= 0) {
        _ = c.shutdown(args.writer_fd, c.SHUT_WR);
    }
    closeFd(&args.writer_fd);
    closeFd(&args.reader_fd);
}

fn waitReadable(fd: c_int, timeout_ms: c_int) bool {
    return waitFd(fd, @intCast(c.POLLIN | c.POLLHUP | c.POLLERR), timeout_ms);
}

fn waitWritable(fd: c_int, timeout_ms: c_int) bool {
    return waitFd(fd, @intCast(c.POLLOUT | c.POLLHUP | c.POLLERR), timeout_ms);
}

fn waitFd(fd: c_int, events: c_short, timeout_ms: c_int) bool {
    if (fd < 0) return false;
    var pfd = c.pollfd{
        .fd = fd,
        .events = events,
        .revents = 0,
    };
    const rc = c.poll(&pfd, 1, timeout_ms);
    return rc > 0 and (pfd.revents & events) != 0;
}

fn closeFd(fd: *c_int) void {
    if (fd.* >= 0) {
        _ = c.close(fd.*);
        fd.* = -1;
    }
}

fn terminateChild(pid: *c.pid_t) void {
    var status: c_int = 0;
    if (c.waitpid(pid.*, &status, c.WNOHANG) == pid.*) return;
    _ = c.kill(pid.*, c.SIGTERM);
    const deadline_ns = nowNs() + (@as(i128, CHILD_TERM_TIMEOUT_MS) * std.time.ns_per_ms);
    while (nowNs() < deadline_ns) {
        if (c.waitpid(pid.*, &status, c.WNOHANG) == pid.*) return;
        sleepMillis(50);
    }
    _ = c.kill(pid.*, c.SIGKILL);
    _ = c.waitpid(pid.*, &status, 0);
}

fn ignoreSigpipe() void {
    if (is_windows) return;
    _ = c.signal(c.SIGPIPE, handleSigpipe);
}

fn handleSigpipe(_: c_int) callconv(.c) void {
    // Stdio relay pumps can race child shutdown; EPIPE means the stream closed.
}

fn closeInheritedFds() void {
    var fd: c_int = 3;
    while (fd < 4096) : (fd += 1) {
        _ = c.close(fd);
    }
}

fn nowNs() i128 {
    var ts: std.c.timespec = undefined;
    if (std.c.clock_gettime(.REALTIME, &ts) != 0) return 0;
    return @as(i128, @intCast(ts.sec)) * std.time.ns_per_s + @as(i128, @intCast(ts.nsec));
}

fn sleepMillis(ms: i64) void {
    std.Io.sleep(
        std.Io.Threaded.global_single_threaded.io(),
        std.Io.Duration.fromMilliseconds(@intCast(ms)),
        .awake,
    ) catch {};
}
