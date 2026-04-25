const std = @import("std");

pub const Error = error{
    NoDescriptionRegistered,
};

pub const StaticDescribe = struct {
    json: []const u8,
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

test "register static describe response" {
    defer clearStaticResponse();
    useStaticResponse(.{ .json = "{\"holon\":\"test\"}" });
    try std.testing.expectEqualStrings("{\"holon\":\"test\"}", try currentJson());
}
