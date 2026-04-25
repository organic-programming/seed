pub const c = @cImport({
    @cInclude("grpc/grpc.h");
    @cInclude("grpc/byte_buffer.h");
    @cInclude("grpc/byte_buffer_reader.h");
    @cInclude("grpc/credentials.h");
    @cInclude("grpc/grpc_posix.h");
    @cInclude("grpc/slice.h");
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

pub fn deadlineAfterMillis(milliseconds: i64) c.gpr_timespec {
    return c.gpr_time_add(
        c.gpr_now(c.GPR_CLOCK_REALTIME),
        c.gpr_time_from_millis(milliseconds, c.GPR_TIMESPAN),
    );
}

pub fn deadlineAfterSeconds(seconds: i64) c.gpr_timespec {
    return c.gpr_time_add(
        c.gpr_now(c.GPR_CLOCK_REALTIME),
        c.gpr_time_from_seconds(seconds, c.GPR_TIMESPAN),
    );
}

pub fn sliceLen(slice: c.grpc_slice) usize {
    if (slice.refcount != null) return slice.data.refcounted.length;
    return slice.data.inlined.length;
}

pub fn slicePtr(slice: *const c.grpc_slice) [*]const u8 {
    if (slice.refcount != null) return @ptrCast(slice.data.refcounted.bytes);
    return @ptrCast(&slice.data.inlined.bytes);
}

pub fn readByteBuffer(allocator: @import("std").mem.Allocator, bb: *c.grpc_byte_buffer) ![]u8 {
    var reader: c.grpc_byte_buffer_reader = undefined;
    if (c.grpc_byte_buffer_reader_init(&reader, bb) == 0) return error.ByteBufferReaderInitFailed;
    defer c.grpc_byte_buffer_reader_destroy(&reader);

    var all = c.grpc_byte_buffer_reader_readall(&reader);
    defer c.grpc_slice_unref(all);

    const len = sliceLen(all);
    const out = try allocator.alloc(u8, len);
    @memcpy(out, slicePtr(&all)[0..len]);
    return out;
}

test "gRPC Core headers are available" {
    const std = @import("std");
    try std.testing.expectEqual(@as(c.grpc_status_code, @intCast(c.GRPC_STATUS_OK)), statusOk());
}
