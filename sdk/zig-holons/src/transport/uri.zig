const std = @import("std");

pub const Scheme = enum {
    stdio,
    tcp,
    unix,
    ws,
    wss,
    rest_sse,

    pub fn text(self: Scheme) []const u8 {
        return switch (self) {
            .stdio => "stdio",
            .tcp => "tcp",
            .unix => "unix",
            .ws => "ws",
            .wss => "wss",
            .rest_sse => "rest+sse",
        };
    }
};

pub const Endpoint = struct {
    raw: []const u8,
    scheme: Scheme,
    address: []const u8,
};

pub const ParseError = error{
    EmptyUri,
    MissingScheme,
    UnsupportedScheme,
};

pub fn parse(raw: []const u8) ParseError!Endpoint {
    if (raw.len == 0) return error.EmptyUri;
    const sep = std.mem.indexOf(u8, raw, "://") orelse return error.MissingScheme;
    const scheme_text = raw[0..sep];
    const address = raw[sep + 3 ..];
    const scheme = if (std.mem.eql(u8, scheme_text, "stdio"))
        Scheme.stdio
    else if (std.mem.eql(u8, scheme_text, "tcp"))
        Scheme.tcp
    else if (std.mem.eql(u8, scheme_text, "unix"))
        Scheme.unix
    else if (std.mem.eql(u8, scheme_text, "ws"))
        Scheme.ws
    else if (std.mem.eql(u8, scheme_text, "wss"))
        Scheme.wss
    else if (std.mem.eql(u8, scheme_text, "rest+sse"))
        Scheme.rest_sse
    else
        return error.UnsupportedScheme;
    return .{ .raw = raw, .scheme = scheme, .address = address };
}

test "parse rest+sse URI" {
    const ep = try parse("rest+sse://127.0.0.1:8080/rpc");
    try std.testing.expectEqual(Scheme.rest_sse, ep.scheme);
    try std.testing.expectEqualStrings("127.0.0.1:8080/rpc", ep.address);
}
