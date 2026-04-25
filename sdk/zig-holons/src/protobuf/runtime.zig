pub const c = @cImport({
    @cInclude("protobuf-c/protobuf-c.h");
});

pub const RuntimeVersion = struct {
    text: [*:0]const u8,
    number: c_uint,
};

pub fn version() RuntimeVersion {
    return .{
        .text = c.protobuf_c_version(),
        .number = c.protobuf_c_version_number(),
    };
}

test "protobuf-c runtime is available" {
    const std = @import("std");
    try std.testing.expect(version().number >= 1005002);
}
