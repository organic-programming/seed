pub const c = @cImport({
    @cInclude("grpc/grpc.h");
    @cInclude("grpc/byte_buffer.h");
    @cInclude("grpc/byte_buffer_reader.h");
    @cInclude("grpc/credentials.h");
});

pub fn init() void {
    c.grpc_init();
}

pub fn shutdown() void {
    c.grpc_shutdown();
}

pub fn statusOk() c.grpc_status_code {
    return c.GRPC_STATUS_OK;
}

test "gRPC Core headers are available" {
    const std = @import("std");
    try std.testing.expectEqual(c.grpc_status_code.GRPC_STATUS_OK, statusOk());
}
