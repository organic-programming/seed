const std = @import("std");

pub const Error = error{
    NoDescriptionRegistered,
};

pub const StaticDescribe = struct {
    json: []const u8 = "",
    proto_builder: ?*const fn (std.mem.Allocator) anyerror![]u8 = null,
};

var registered: ?StaticDescribe = null;

pub fn useStaticResponse(response: StaticDescribe) void {
    registered = response;
}

pub fn clearStaticResponse() void {
    registered = null;
}

pub fn current() Error!StaticDescribe {
    return registered orelse error.NoDescriptionRegistered;
}

pub fn currentJson() Error![]const u8 {
    return (try current()).json;
}

pub fn currentProtoAlloc(allocator: std.mem.Allocator) ![]u8 {
    const response = try current();
    const builder = response.proto_builder orelse return error.NoDescriptionRegistered;
    return builder(allocator);
}

test "register static describe response" {
    defer clearStaticResponse();
    useStaticResponse(.{ .json = "{\"holon\":\"test\"}" });
    try std.testing.expectEqualStrings("{\"holon\":\"test\"}", try currentJson());
}
