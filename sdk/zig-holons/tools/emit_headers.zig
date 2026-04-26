const std = @import("std");
const holons = @import("zig_holons");

pub fn main() !void {
    try holons.cabi.writeHeaderPath("include/holons_sdk.h");
}
