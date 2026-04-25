const std = @import("std");

pub const ABI_VERSION_MAJOR: c_uint = 0;
pub const ABI_VERSION_MINOR: c_uint = 1;
pub const ABI_VERSION_PATCH: c_uint = 0;

pub const HolonsStatus = enum(c_int) {
    ok = 0,
    invalid_argument = 1,
    runtime_error = 2,
};

pub const HolonsHandle = opaque {};

export fn holons_sdk_abi_version_major() callconv(.c) c_uint {
    return ABI_VERSION_MAJOR;
}

export fn holons_sdk_abi_version_minor() callconv(.c) c_uint {
    return ABI_VERSION_MINOR;
}

export fn holons_sdk_abi_version_patch() callconv(.c) c_uint {
    return ABI_VERSION_PATCH;
}

export fn holons_sdk_version() callconv(.c) [*:0]const u8 {
    return "0.1.0";
}

export fn holons_sdk_free(ptr: ?*anyopaque) callconv(.c) void {
    if (ptr) |_| {}
}

test "ABI version is exposed" {
    try std.testing.expectEqual(@as(c_uint, 0), holons_sdk_abi_version_major());
}
