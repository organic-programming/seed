const std = @import("std");

pub const Identity = struct {
    slug: []const u8,
    name: []const u8,
    version: []const u8,
};

pub const Error = error{
    MissingManifest,
    MissingSlug,
};

pub fn fromManifestFields(slug: []const u8, name: []const u8, version: []const u8) Error!Identity {
    if (slug.len == 0) return error.MissingSlug;
    return .{ .slug = slug, .name = name, .version = version };
}

pub fn inferSlugFromProtoText(proto_text: []const u8) Error![]const u8 {
    const marker = "slug:";
    const idx = std.mem.indexOf(u8, proto_text, marker) orelse return error.MissingManifest;
    var rest = std.mem.trimStart(u8, proto_text[idx + marker.len ..], " \t\r\n\"");
    const end = std.mem.indexOfAny(u8, rest, "\"\r\n") orelse rest.len;
    rest = rest[0..end];
    if (rest.len == 0) return error.MissingSlug;
    return rest;
}

test "infer slug from proto manifest text" {
    try std.testing.expectEqualStrings("gabriel-greeting-zig", try inferSlugFromProtoText("slug: \"gabriel-greeting-zig\""));
}
