const std = @import("std");
const holons = @import("zig_holons");

const runtime = holons.protobuf.runtime;
const c = runtime.c;

pub const methods = [_]holons.grpc.server.Method{
    .{ .path = "/greeting.v1.GreetingService/SayHello", .handler = sayHello },
    .{ .path = "/greeting.v1.GreetingService/ListLanguages", .handler = listLanguages },
};

pub fn registerDescribe() void {
    holons.describe.useStaticResponse(.{
        .json = "{\"family_name\":\"Greeting-Zig-Test\"}",
        .proto_builder = buildDescribe,
    });
}

pub fn buildDescribe(allocator: std.mem.Allocator) ![]u8 {
    const schema = try allocator.dupeZ(u8, "holon/v1");
    defer allocator.free(schema);
    const uuid = try allocator.dupeZ(u8, "00000000-0000-4000-8000-000000000004");
    defer allocator.free(uuid);
    const given_name = try allocator.dupeZ(u8, "Gabriel");
    defer allocator.free(given_name);
    const family_name = try allocator.dupeZ(u8, "Greeting-Zig-Test");
    defer allocator.free(family_name);
    const lang = try allocator.dupeZ(u8, "zig");
    defer allocator.free(lang);
    const kind = try allocator.dupeZ(u8, "native");
    defer allocator.free(kind);

    var identity: c.Holons__V1__HolonManifest__Identity = undefined;
    c.holons__v1__holon_manifest__identity__init(&identity);
    identity.schema = schema.ptr;
    identity.uuid = uuid.ptr;
    identity.given_name = given_name.ptr;
    identity.family_name = family_name.ptr;

    var manifest: c.Holons__V1__HolonManifest = undefined;
    c.holons__v1__holon_manifest__init(&manifest);
    manifest.identity = &identity;
    manifest.lang = lang.ptr;
    manifest.kind = kind.ptr;

    var response: c.Holons__V1__DescribeResponse = undefined;
    c.holons__v1__describe_response__init(&response);
    response.manifest = &manifest;

    return runtime.packDescribeResponse(allocator, &response);
}

fn sayHello(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    var request = try runtime.unpackSayHelloRequest(bytes);
    defer request.deinit();

    const raw_name = std.mem.trim(u8, request.name(), " \t\r\n");
    const name = if (raw_name.len == 0) "friend" else raw_name;
    const lang_code = if (request.langCode().len == 0) "en" else request.langCode();
    const language = if (std.mem.eql(u8, lang_code, "fr")) "French" else "English";
    const greeting = if (std.mem.eql(u8, lang_code, "fr"))
        try std.fmt.allocPrint(allocator, "Bonjour {s}", .{name})
    else
        try std.fmt.allocPrint(allocator, "Hello {s}", .{name});
    defer allocator.free(greeting);

    return runtime.packSayHelloResponse(allocator, greeting, language, lang_code);
}

fn listLanguages(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    try runtime.unpackListLanguagesRequest(bytes);
    return runtime.packListLanguagesResponse(allocator, &.{
        .{ .code = "en", .name = "English", .native = "English" },
        .{ .code = "fr", .name = "French", .native = "Francais" },
    });
}
