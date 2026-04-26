pub const c = @cImport({
    @cInclude("grpc/grpc.h");
    @cInclude("grpc/grpc_posix.h");
});

pub const Fd = c_int;

pub fn hasPosixFdApi() bool {
    return @TypeOf(c.grpc_channel_create_from_fd) != void and @TypeOf(c.grpc_server_add_channel_from_fd) != void;
}

test "gRPC POSIX fd APIs are visible" {
    const std = @import("std");
    try std.testing.expect(hasPosixFdApi());
}
