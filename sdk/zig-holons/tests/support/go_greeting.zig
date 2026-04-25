const std = @import("std");

const c = @cImport({
    @cInclude("arpa/inet.h");
    @cInclude("netinet/in.h");
    @cInclude("signal.h");
    @cInclude("stdlib.h");
    @cInclude("sys/socket.h");
    @cInclude("sys/wait.h");
    @cInclude("unistd.h");
});

pub const Process = struct {
    pid: c.pid_t,

    pub fn stop(self: *Process) void {
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

pub fn goGreetingDir(allocator: std.mem.Allocator) ![:0]u8 {
    const candidates = [_][]const u8{
        "../../examples/hello-world/gabriel-greeting-go",
        "examples/hello-world/gabriel-greeting-go",
    };
    for (candidates) |candidate| {
        const marker = try std.fs.path.join(allocator, &.{ candidate, "go.mod" });
        defer allocator.free(marker);
        std.Io.Dir.cwd().access(std.testing.io, marker, .{}) catch continue;
        return allocator.dupeZ(u8, candidate);
    }
    return error.GoGreetingExampleNotFound;
}

pub fn reserveLoopbackPort() !u16 {
    const fd = c.socket(c.AF_INET, c.SOCK_STREAM, 0);
    if (fd < 0) return error.SocketFailed;
    defer _ = c.close(fd);

    var addr: c.struct_sockaddr_in = std.mem.zeroes(c.struct_sockaddr_in);
    if (@hasField(c.struct_sockaddr_in, "sin_len")) {
        addr.sin_len = @sizeOf(c.struct_sockaddr_in);
    }
    addr.sin_family = c.AF_INET;
    addr.sin_port = 0;
    addr.sin_addr.s_addr = c.htonl(0x7f000001);

    if (c.bind(fd, @ptrCast(&addr), @sizeOf(c.struct_sockaddr_in)) != 0) return error.BindFailed;
    var len: c.socklen_t = @sizeOf(c.struct_sockaddr_in);
    if (c.getsockname(fd, @ptrCast(&addr), &len) != 0) return error.GetSockNameFailed;
    return c.ntohs(addr.sin_port);
}

pub fn waitTcpPort(port: u16, attempts: u32) !void {
    var remaining = attempts;
    while (remaining > 0) : (remaining -= 1) {
        if (tryConnectLoopback(port)) return;
        try std.Io.sleep(std.testing.io, std.Io.Duration.fromMilliseconds(125), .awake);
    }
    return error.TcpPortNotReady;
}

pub fn startTcp(allocator: std.mem.Allocator, port: u16) !Process {
    const cwd = try goGreetingDir(allocator);
    defer allocator.free(cwd);
    var listen_buf: [64]u8 = undefined;
    const listen = try std.fmt.bufPrintZ(&listen_buf, "tcp://127.0.0.1:{}", .{port});
    const argv = [_:null]?[*:0]const u8{
        "go",
        "run",
        "./cmd",
        "serve",
        "--listen",
        listen.ptr,
    };
    return .{ .pid = try forkExec(cwd.ptr, &argv) };
}

fn tryConnectLoopback(port: u16) bool {
    const fd = c.socket(c.AF_INET, c.SOCK_STREAM, 0);
    if (fd < 0) return false;
    defer _ = c.close(fd);

    var addr: c.struct_sockaddr_in = std.mem.zeroes(c.struct_sockaddr_in);
    if (@hasField(c.struct_sockaddr_in, "sin_len")) {
        addr.sin_len = @sizeOf(c.struct_sockaddr_in);
    }
    addr.sin_family = c.AF_INET;
    addr.sin_port = c.htons(port);
    addr.sin_addr.s_addr = c.htonl(0x7f000001);

    return c.connect(fd, @ptrCast(&addr), @sizeOf(c.struct_sockaddr_in)) == 0;
}

fn forkExec(cwd: [*:0]const u8, argv: [*:null]const ?[*:0]const u8) !c.pid_t {
    const pid = c.fork();
    if (pid < 0) return error.ForkFailed;
    if (pid == 0) {
        _ = c.setpgid(0, 0);
        if (c.chdir(cwd) != 0) c._exit(126);
        _ = c.execvp(argv[0].?, @ptrCast(argv));
        c._exit(127);
    }
    _ = c.setpgid(pid, pid);
    return pid;
}
