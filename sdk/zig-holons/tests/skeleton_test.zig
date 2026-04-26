const std = @import("std");
const holons = @import("zig_holons");

test "skeleton exposes rust-parity modules" {
    try std.testing.expect(holons.transport.supportsServe(.stdio));
    try std.testing.expect(holons.transport.supportsServe(.tcp));
    try std.testing.expect(holons.transport.supportsServe(.unix));
    try std.testing.expect(holons.transport.supportsDial(.ws));
    try std.testing.expect(holons.transport.supportsDial(.wss));
    try std.testing.expect(holons.transport.supportsDial(.rest_sse));
}
