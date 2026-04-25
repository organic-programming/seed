const std = @import("std");
const holons = @import("zig_holons");

extern fn setenv(name: [*:0]const u8, value: [*:0]const u8, overwrite: c_int) c_int;

test "discover searches siblings cwd source built installed and cached scopes" {
    const allocator = std.testing.allocator;
    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const root = try std.fs.path.join(allocator, &.{ ".zig-cache", "tmp", tmp.sub_path[0..] });
    defer allocator.free(root);

    try writePackage(allocator, root, "cwd-alpha.holon", "cwd-alpha", "cwd-uuid");
    try writePackage(allocator, root, ".op/build/built-beta.holon", "built-beta", "built-uuid");
    try writeSourceHolon(allocator, root, "source-gamma", "source-uuid");

    const opbin_root = try std.fs.path.join(allocator, &.{ root, "opbin" });
    defer allocator.free(opbin_root);
    try writePackage(allocator, opbin_root, "installed-delta.holon", "installed-delta", "installed-uuid");

    const oppath_root = try std.fs.path.join(allocator, &.{ root, "oppath" });
    defer allocator.free(oppath_root);
    try writePackage(allocator, oppath_root, "cache/cached-epsilon.holon", "cached-epsilon", "cached-uuid");

    const app_bin = try std.fs.path.join(allocator, &.{ root, "Harness.app", "Contents", "MacOS", "harness" });
    defer allocator.free(app_bin);
    try makeParent(allocator, app_bin);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{ .sub_path = app_bin, .data = "" });
    try writePackage(allocator, root, "Harness.app/Contents/Resources/Holons/sibling-zeta.holon", "sibling-zeta", "sibling-uuid");

    try setEnv("OPBIN", opbin_root);
    try setEnv("OPPATH", oppath_root);
    try setEnv("ZIG_HOLONS_TEST_EXECUTABLE", app_bin);

    var result = try holons.discover.discover(
        allocator,
        holons.discover.LOCAL,
        null,
        root,
        holons.discover.ALL,
        holons.discover.NO_LIMIT,
        holons.discover.NO_TIMEOUT,
    );
    defer result.deinit(allocator);

    try expectSlug(result, "cwd-alpha");
    try expectSlug(result, "built-beta");
    try expectSlug(result, "source-gamma");
    try expectSlug(result, "installed-delta");
    try expectSlug(result, "cached-epsilon");
    try expectSlug(result, "sibling-zeta");

    var resolved = try holons.discover.resolve(
        allocator,
        holons.discover.LOCAL,
        "source-gamma",
        root,
        holons.discover.ALL,
        holons.discover.NO_TIMEOUT,
    );
    defer resolved.deinit(allocator);
    try std.testing.expect(resolved.ref != null);
    try std.testing.expectEqualStrings("source-gamma", resolved.ref.?.info.?.slug);
}

fn writePackage(allocator: std.mem.Allocator, root: []const u8, rel: []const u8, slug: []const u8, uuid: []const u8) !void {
    const dir_path = try std.fs.path.join(allocator, &.{ root, rel });
    defer allocator.free(dir_path);
    try std.Io.Dir.cwd().createDirPath(std.testing.io, dir_path);

    const file_path = try std.fs.path.join(allocator, &.{ dir_path, ".holon.json" });
    defer allocator.free(file_path);
    const body = try std.fmt.allocPrint(allocator,
        \\{{
        \\  "schema": "holon-package/v1",
        \\  "slug": "{s}",
        \\  "uuid": "{s}",
        \\  "identity": {{"given_name": "{s}", "family_name": "Holon", "aliases": ["{s}-alias"]}},
        \\  "lang": "zig",
        \\  "runner": "zig",
        \\  "status": "draft",
        \\  "kind": "native",
        \\  "transport": "stdio://",
        \\  "entrypoint": "bin/{s}",
        \\  "architectures": ["darwin_arm64"],
        \\  "has_dist": true,
        \\  "has_source": false
        \\}}
    , .{ slug, uuid, slug, slug, slug });
    defer allocator.free(body);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{ .sub_path = file_path, .data = body });
}

fn writeSourceHolon(allocator: std.mem.Allocator, root: []const u8, slug: []const u8, uuid: []const u8) !void {
    const api_dir = try std.fs.path.join(allocator, &.{ root, slug, "api", "v1" });
    defer allocator.free(api_dir);
    try std.Io.Dir.cwd().createDirPath(std.testing.io, api_dir);

    const proto_path = try std.fs.path.join(allocator, &.{ api_dir, "holon.proto" });
    defer allocator.free(proto_path);
    const body = try std.fmt.allocPrint(allocator,
        \\syntax = "proto3";
        \\package {s}.v1;
        \\option (holons.v1.manifest) = {{
        \\  identity: {{ uuid: "{s}" given_name: "Source" family_name: "Gamma?" aliases: ["source-alias"] }}
        \\  lang: "zig"
        \\  kind: "native"
        \\  build: {{ runner: "zig" main: "src/main.zig" }}
        \\  artifacts: {{ binary: "{s}" primary: "{s}.holon" }}
        \\}};
    , .{ slug, uuid, slug, slug });
    defer allocator.free(body);
    try std.Io.Dir.cwd().writeFile(std.testing.io, .{ .sub_path = proto_path, .data = body });
}

fn makeParent(allocator: std.mem.Allocator, path: []const u8) !void {
    const parent = std.fs.path.dirname(path) orelse return;
    const duped = try allocator.dupe(u8, parent);
    defer allocator.free(duped);
    try std.Io.Dir.cwd().createDirPath(std.testing.io, duped);
}

fn setEnv(name: [:0]const u8, value: []const u8) !void {
    var buffer: [std.Io.Dir.max_path_bytes:0]u8 = undefined;
    if (value.len >= buffer.len) return error.NameTooLong;
    @memcpy(buffer[0..value.len], value);
    buffer[value.len] = 0;
    if (setenv(name, &buffer, 1) != 0) return error.SetEnvFailed;
}

fn expectSlug(result: holons.discover.DiscoverResult, slug: []const u8) !void {
    for (result.found) |ref| {
        if (ref.info) |info| {
            if (std.mem.eql(u8, info.slug, slug)) return;
        }
    }
    std.debug.print("missing slug {s}; found {d} refs\n", .{ slug, result.found.len });
    return error.SlugNotFound;
}
