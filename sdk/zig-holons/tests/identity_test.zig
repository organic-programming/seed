const std = @import("std");
const holons = @import("zig_holons");

test "identity resolves holon.proto manifest metadata" {
    const allocator = std.testing.allocator;
    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    try tmp.dir.writeFile(std.testing.io, .{
        .sub_path = "holon.proto",
        .data =
        \\syntax = "proto3";
        \\package example.v1;
        \\option (holons.v1.manifest) = {
        \\  identity: {
        \\    uuid: "identity-uuid"
        \\    given_name: "Zig"
        \\    family_name: "Identity?"
        \\    aliases: ["zig-id", "zid"]
        \\  }
        \\  lang: "zig"
        \\  kind: "native"
        \\  build: { runner: "zig" main: "src/main.zig" }
        \\  artifacts: { binary: "zig-identity" primary: "zig-identity.holon" }
        \\};
        ,
    });

    const path = try std.fs.path.join(allocator, &.{ ".zig-cache", "tmp", tmp.sub_path[0..], "holon.proto" });
    defer allocator.free(path);

    var manifest = try holons.identity.resolveProtoFile(allocator, path);
    defer manifest.deinit(allocator);

    try std.testing.expectEqualStrings("identity-uuid", manifest.identity.uuid);
    try std.testing.expectEqualStrings("zig", manifest.identity.lang);
    try std.testing.expectEqualStrings("zig", manifest.build_runner);
    try std.testing.expectEqualStrings("zig-id", manifest.identity.aliases[0]);

    const slug = try manifest.identity.slugAlloc(allocator);
    defer allocator.free(slug);
    try std.testing.expectEqualStrings("zig-identity", slug);
}
