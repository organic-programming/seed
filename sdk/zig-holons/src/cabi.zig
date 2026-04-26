const std = @import("std");
const connect_mod = @import("connect.zig");
const describe_mod = @import("describe.zig");
const discover_mod = @import("discover.zig");
const grpc_client = @import("grpc/client.zig");
const grpc_server = @import("grpc/server.zig");
const holonrpc = @import("holonrpc.zig");
const hub_mod = @import("hub.zig");
const serve_mod = @import("serve.zig");

const c = @cImport({
    @cInclude("stdlib.h");
});

pub const ABI_VERSION_MAJOR: c_uint = 0;
pub const ABI_VERSION_MINOR: c_uint = 1;
pub const ABI_VERSION_PATCH: c_uint = 0;

pub const HolonsStatus = enum(c_int) {
    ok = 0,
    invalid_argument = 1,
    runtime_error = 2,
    not_found = 3,
    unsupported = 4,
    out_of_memory = 5,
};

pub const StringResult = extern struct {
    status: HolonsStatus = .ok,
    data: ?[*:0]u8 = null,
    error_message: ?[*:0]u8 = null,
};

const Context = struct {
    allocator: std.mem.Allocator = std.heap.c_allocator,
};

const Connection = struct {
    channel: grpc_client.Channel,
};

const Server = struct {
    server: grpc_server.Server,
};

const DiscoveryResult = struct {
    result: discover_mod.DiscoverResult,
};

const HubClient = struct {
    client: hub_mod.Client,
};

const allocator = std.heap.c_allocator;
var describe_mutex: std.Io.Mutex = .init;
var registered_describe_json: ?[:0]u8 = null;
var registered_describe_proto: ?[]u8 = null;

pub const header =
    \\#ifndef HOLONS_SDK_H
    \\#define HOLONS_SDK_H
    \\
    \\#include <stddef.h>
    \\#include <stdint.h>
    \\
    \\#ifdef __cplusplus
    \\extern "C" {
    \\#endif
    \\
    \\#define HOLONS_SDK_ABI_VERSION_MAJOR 0u
    \\#define HOLONS_SDK_ABI_VERSION_MINOR 1u
    \\#define HOLONS_SDK_ABI_VERSION_PATCH 0u
    \\
    \\typedef enum holons_status {
    \\  HOLONS_STATUS_OK = 0,
    \\  HOLONS_STATUS_INVALID_ARGUMENT = 1,
    \\  HOLONS_STATUS_RUNTIME_ERROR = 2,
    \\  HOLONS_STATUS_NOT_FOUND = 3,
    \\  HOLONS_STATUS_UNSUPPORTED = 4,
    \\  HOLONS_STATUS_OUT_OF_MEMORY = 5
    \\} holons_status;
    \\
    \\typedef struct holons_sdk_context holons_sdk_context;
    \\typedef struct holons_connection holons_connection;
    \\typedef struct holons_server holons_server;
    \\typedef struct holons_discovery_result holons_discovery_result;
    \\typedef struct holons_hub_client holons_hub_client;
    \\
    \\typedef struct holons_string_result {
    \\  holons_status status;
    \\  char *data;
    \\  char *error_message;
    \\} holons_string_result;
    \\
    \\enum {
    \\  HOLONS_DISCOVER_SCOPE_LOCAL = 0,
    \\  HOLONS_DISCOVER_SCOPE_PROXY = 1,
    \\  HOLONS_DISCOVER_SCOPE_DELEGATED = 2,
    \\  HOLONS_DISCOVER_SIBLINGS = 0x01,
    \\  HOLONS_DISCOVER_CWD = 0x02,
    \\  HOLONS_DISCOVER_SOURCE = 0x04,
    \\  HOLONS_DISCOVER_BUILT = 0x08,
    \\  HOLONS_DISCOVER_INSTALLED = 0x10,
    \\  HOLONS_DISCOVER_CACHED = 0x20,
    \\  HOLONS_DISCOVER_ALL = 0x3f,
    \\  HOLONS_DISCOVER_NO_LIMIT = 0,
    \\  HOLONS_DISCOVER_NO_TIMEOUT = 0
    \\};
    \\
    \\unsigned int holons_sdk_abi_version_major(void);
    \\unsigned int holons_sdk_abi_version_minor(void);
    \\unsigned int holons_sdk_abi_version_patch(void);
    \\const char *holons_sdk_version(void);
    \\const char *holons_status_message(holons_status status);
    \\
    \\holons_status holons_sdk_init(holons_sdk_context **out);
    \\void holons_sdk_shutdown(holons_sdk_context *context);
    \\void holons_sdk_free(void *ptr);
    \\void holons_string_result_free(holons_string_result *result);
    \\
    \\holons_status holons_connect(holons_sdk_context *context, const char *uri, holons_connection **out);
    \\holons_string_result holons_connection_describe_json(holons_connection *connection);
    \\void holons_connection_close(holons_connection *connection);
    \\
    \\holons_status holons_describe_register_static_json(holons_sdk_context *context, const char *json);
    \\holons_status holons_describe_register_static_proto(holons_sdk_context *context, const uint8_t *bytes, size_t len);
    \\void holons_describe_clear_static(void);
    \\holons_string_result holons_describe_static_json(void);
    \\
    \\holons_status holons_serve_blocking(holons_sdk_context *context, const char *listen_uri);
    \\holons_status holons_server_start(holons_sdk_context *context, const char *listen_uri, holons_server **out);
    \\void holons_server_shutdown(holons_server *server);
    \\void holons_server_wait(holons_server *server);
    \\void holons_server_free(holons_server *server);
    \\
    \\holons_status holons_discover(holons_sdk_context *context, const char *expression, const char *root, int specifiers, int limit, uint32_t timeout_ms, holons_discovery_result **out);
    \\size_t holons_discovery_result_len(const holons_discovery_result *result);
    \\holons_string_result holons_discovery_result_json(const holons_discovery_result *result);
    \\void holons_discovery_result_free(holons_discovery_result *result);
    \\
    \\holons_status holons_hub_client_connect(holons_sdk_context *context, const char *uri, holons_hub_client **out);
    \\holons_string_result holons_hub_client_invoke_json(holons_hub_client *client, const char *method, const char *params_json);
    \\void holons_hub_client_close(holons_hub_client *client);
    \\
    \\#ifdef __cplusplus
    \\}
    \\#endif
    \\
    \\#endif
    \\
;

pub fn writeHeaderPath(path: []const u8) !void {
    if (std.fs.path.dirname(path)) |dir| {
        try std.Io.Dir.cwd().createDirPath(std.Options.debug_io, dir);
    }
    try std.Io.Dir.cwd().writeFile(std.Options.debug_io, .{ .sub_path = path, .data = header });
}

pub const exported_symbols = struct {
    pub const abi_version_major = holons_sdk_abi_version_major;
    pub const abi_version_minor = holons_sdk_abi_version_minor;
    pub const abi_version_patch = holons_sdk_abi_version_patch;
    pub const sdk_version = holons_sdk_version;
    pub const status_message = holons_status_message;
    pub const init = holons_sdk_init;
    pub const shutdown = holons_sdk_shutdown;
    pub const free = holons_sdk_free;
    pub const string_result_free = holons_string_result_free;
    pub const connect = holons_connect;
    pub const connection_describe_json = holons_connection_describe_json;
    pub const connection_close = holons_connection_close;
    pub const describe_register_static_json = holons_describe_register_static_json;
    pub const describe_register_static_proto = holons_describe_register_static_proto;
    pub const describe_clear_static = holons_describe_clear_static;
    pub const describe_static_json = holons_describe_static_json;
    pub const serve_blocking = holons_serve_blocking;
    pub const server_start = holons_server_start;
    pub const server_shutdown = holons_server_shutdown;
    pub const server_wait = holons_server_wait;
    pub const server_free = holons_server_free;
    pub const discover = holons_discover;
    pub const discovery_result_len = holons_discovery_result_len;
    pub const discovery_result_json = holons_discovery_result_json;
    pub const discovery_result_free = holons_discovery_result_free;
    pub const hub_client_connect = holons_hub_client_connect;
    pub const hub_client_invoke_json = holons_hub_client_invoke_json;
    pub const hub_client_close = holons_hub_client_close;
};

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

export fn holons_status_message(status: HolonsStatus) callconv(.c) [*:0]const u8 {
    return switch (status) {
        .ok => "ok",
        .invalid_argument => "invalid argument",
        .runtime_error => "runtime error",
        .not_found => "not found",
        .unsupported => "unsupported",
        .out_of_memory => "out of memory",
    };
}

export fn holons_sdk_init(out: ?*?*Context) callconv(.c) HolonsStatus {
    const slot = out orelse return .invalid_argument;
    const context = allocator.create(Context) catch return .out_of_memory;
    context.* = .{};
    slot.* = context;
    grpc_client.init();
    return .ok;
}

export fn holons_sdk_shutdown(context: ?*Context) callconv(.c) void {
    const ctx = context orelse return;
    grpc_client.shutdown();
    allocator.destroy(ctx);
}

export fn holons_sdk_free(ptr: ?*anyopaque) callconv(.c) void {
    if (ptr) |p| c.free(p);
}

export fn holons_string_result_free(result: ?*StringResult) callconv(.c) void {
    const res = result orelse return;
    if (res.data) |data| c.free(data);
    if (res.error_message) |message| c.free(message);
    res.* = .{};
}

export fn holons_connect(context: ?*Context, uri: ?[*:0]const u8, out: ?*?*Connection) callconv(.c) HolonsStatus {
    _ = context orelse return .invalid_argument;
    const target_uri = cString(uri) catch return .invalid_argument;
    const slot = out orelse return .invalid_argument;
    slot.* = null;

    const connection = allocator.create(Connection) catch return .out_of_memory;
    errdefer allocator.destroy(connection);
    connection.* = .{
        .channel = grpc_client.connect(allocator, target_uri) catch |err| return statusFromError(err),
    };
    slot.* = connection;
    return .ok;
}

export fn holons_connection_describe_json(connection: ?*Connection) callconv(.c) StringResult {
    const conn = connection orelse return errorResult(.invalid_argument, "connection is required");
    var response = conn.channel.describe(allocator) catch |err| return errorNameResult(err);
    defer response.deinit();
    const json = describeResponseJson(response) catch |err| return errorNameResult(err);
    return okOwnedResult(json);
}

export fn holons_connection_close(connection: ?*Connection) callconv(.c) void {
    const conn = connection orelse return;
    conn.channel.deinit();
    allocator.destroy(conn);
}

export fn holons_describe_register_static_json(context: ?*Context, json: ?[*:0]const u8) callconv(.c) HolonsStatus {
    _ = context orelse return .invalid_argument;
    const value = cString(json) catch return .invalid_argument;
    const copy = allocator.dupeZ(u8, value) catch return .out_of_memory;
    describe_mutex.lockUncancelable(std.Options.debug_io);
    defer describe_mutex.unlock(std.Options.debug_io);
    if (registered_describe_json) |old| c.free(old.ptr);
    registered_describe_json = copy;
    refreshStaticDescribeLocked();
    return .ok;
}

export fn holons_describe_register_static_proto(
    context: ?*Context,
    bytes: ?[*]const u8,
    len: usize,
) callconv(.c) HolonsStatus {
    _ = context orelse return .invalid_argument;
    if (bytes == null and len != 0) return .invalid_argument;
    const copy = allocator.alloc(u8, len) catch return .out_of_memory;
    if (len > 0) @memcpy(copy, bytes.?[0..len]);
    describe_mutex.lockUncancelable(std.Options.debug_io);
    defer describe_mutex.unlock(std.Options.debug_io);
    if (registered_describe_proto) |old| allocator.free(old);
    registered_describe_proto = copy;
    refreshStaticDescribeLocked();
    return .ok;
}

export fn holons_describe_clear_static() callconv(.c) void {
    describe_mutex.lockUncancelable(std.Options.debug_io);
    defer describe_mutex.unlock(std.Options.debug_io);
    if (registered_describe_json) |json| c.free(json.ptr);
    if (registered_describe_proto) |proto| allocator.free(proto);
    registered_describe_json = null;
    registered_describe_proto = null;
    describe_mod.clearStaticResponse();
}

export fn holons_describe_static_json() callconv(.c) StringResult {
    const json = describe_mod.currentJson() catch |err| return errorNameResult(err);
    return dupResult(json);
}

export fn holons_serve_blocking(context: ?*Context, listen_uri: ?[*:0]const u8) callconv(.c) HolonsStatus {
    _ = context orelse return .invalid_argument;
    const uri = cString(listen_uri) catch return .invalid_argument;
    serve_mod.runSingle(.{ .listen_uri = uri, .methods = &.{} }) catch |err| return statusFromError(err);
    return .ok;
}

export fn holons_server_start(context: ?*Context, listen_uri: ?[*:0]const u8, out: ?*?*Server) callconv(.c) HolonsStatus {
    _ = context orelse return .invalid_argument;
    const uri = cString(listen_uri) catch return .invalid_argument;
    const slot = out orelse return .invalid_argument;
    slot.* = null;
    const handle = allocator.create(Server) catch return .out_of_memory;
    errdefer allocator.destroy(handle);
    handle.* = .{ .server = grpc_server.bind(allocator, uri, &.{}) catch |err| return statusFromError(err) };
    handle.server.start() catch |err| {
        handle.server.deinit();
        return statusFromError(err);
    };
    slot.* = handle;
    return .ok;
}

export fn holons_server_shutdown(server: ?*Server) callconv(.c) void {
    if (server) |handle| handle.server.shutdown();
}

export fn holons_server_wait(server: ?*Server) callconv(.c) void {
    if (server) |handle| handle.server.wait();
}

export fn holons_server_free(server: ?*Server) callconv(.c) void {
    const handle = server orelse return;
    handle.server.deinit();
    allocator.destroy(handle);
}

export fn holons_discover(
    context: ?*Context,
    expression: ?[*:0]const u8,
    root: ?[*:0]const u8,
    specifiers: c_int,
    limit: c_int,
    timeout_ms: u32,
    out: ?*?*DiscoveryResult,
) callconv(.c) HolonsStatus {
    _ = context orelse return .invalid_argument;
    const slot = out orelse return .invalid_argument;
    slot.* = null;
    const expression_slice = optionalCString(expression) catch return .invalid_argument;
    const root_slice = optionalCString(root) catch return .invalid_argument;
    const handle = allocator.create(DiscoveryResult) catch return .out_of_memory;
    errdefer allocator.destroy(handle);
    handle.* = .{
        .result = discover_mod.discover(
            allocator,
            discover_mod.LOCAL,
            expression_slice,
            root_slice,
            specifiers,
            limit,
            timeout_ms,
        ) catch |err| return statusFromError(err),
    };
    slot.* = handle;
    return .ok;
}

export fn holons_discovery_result_len(result: ?*const DiscoveryResult) callconv(.c) usize {
    const handle = result orelse return 0;
    return handle.result.found.len;
}

export fn holons_discovery_result_json(result: ?*const DiscoveryResult) callconv(.c) StringResult {
    const handle = result orelse return errorResult(.invalid_argument, "discovery result is required");
    const json = discoveryResultJson(&handle.result) catch |err| return errorNameResult(err);
    return okOwnedResult(json);
}

export fn holons_discovery_result_free(result: ?*DiscoveryResult) callconv(.c) void {
    const handle = result orelse return;
    handle.result.deinit(allocator);
    allocator.destroy(handle);
}

export fn holons_hub_client_connect(context: ?*Context, uri: ?[*:0]const u8, out: ?*?*HubClient) callconv(.c) HolonsStatus {
    _ = context orelse return .invalid_argument;
    const target_uri = cString(uri) catch return .invalid_argument;
    const slot = out orelse return .invalid_argument;
    slot.* = null;
    const handle = allocator.create(HubClient) catch return .out_of_memory;
    errdefer allocator.destroy(handle);
    handle.* = .{ .client = hub_mod.Client.connect(allocator, target_uri) catch |err| return statusFromError(err) };
    slot.* = handle;
    return .ok;
}

export fn holons_hub_client_invoke_json(
    client: ?*HubClient,
    method: ?[*:0]const u8,
    params_json: ?[*:0]const u8,
) callconv(.c) StringResult {
    const handle = client orelse return errorResult(.invalid_argument, "hub client is required");
    const method_slice = cString(method) catch return errorResult(.invalid_argument, "method is required");
    const params_slice = cString(params_json) catch return errorResult(.invalid_argument, "params_json is required");
    var result = handle.client.invokeAlloc(allocator, method_slice, params_slice) catch |err| return errorNameResult(err);
    defer result.deinit(allocator);
    return dupResult(result.result_json);
}

export fn holons_hub_client_close(client: ?*HubClient) callconv(.c) void {
    const handle = client orelse return;
    handle.client.deinit();
    allocator.destroy(handle);
}

fn abiProtoBuilder(out_allocator: std.mem.Allocator) anyerror![]u8 {
    describe_mutex.lockUncancelable(std.Options.debug_io);
    defer describe_mutex.unlock(std.Options.debug_io);
    const proto = registered_describe_proto orelse return error.DescribeProtoUnavailable;
    return out_allocator.dupe(u8, proto);
}

fn refreshStaticDescribeLocked() void {
    describe_mod.useStaticResponse(.{
        .json = if (registered_describe_json) |json| json else "",
        .proto_builder = if (registered_describe_proto == null) null else abiProtoBuilder,
    });
}

fn cString(ptr: ?[*:0]const u8) ![]const u8 {
    const value = ptr orelse return error.InvalidArgument;
    return std.mem.span(value);
}

fn optionalCString(ptr: ?[*:0]const u8) !?[]const u8 {
    if (ptr) |value| {
        const slice = std.mem.span(value);
        return if (slice.len == 0) null else slice;
    }
    return null;
}

fn statusFromError(err: anyerror) HolonsStatus {
    return switch (err) {
        error.InvalidArgument, error.EmptyUri, error.MissingScheme => .invalid_argument,
        error.UnsupportedScheme, error.UnsupportedListenTransport, error.StdioCommandRequired => .unsupported,
        error.NotFound, error.EnvironmentVariableNotFound, error.GoGreetingExampleNotFound => .not_found,
        error.OutOfMemory => .out_of_memory,
        else => .runtime_error,
    };
}

fn errorNameResult(err: anyerror) StringResult {
    return errorResult(statusFromError(err), @errorName(err));
}

fn errorResult(status: HolonsStatus, message: []const u8) StringResult {
    return .{
        .status = status,
        .error_message = dupCString(message) catch null,
    };
}

fn dupResult(bytes: []const u8) StringResult {
    return okOwnedResult(allocator.dupeZ(u8, bytes) catch return errorResult(.out_of_memory, "out of memory"));
}

fn okOwnedResult(data: [:0]u8) StringResult {
    return .{
        .status = .ok,
        .data = data.ptr,
    };
}

fn dupCString(bytes: []const u8) ![:0]u8 {
    return allocator.dupeZ(u8, bytes);
}

fn describeResponseJson(response: anytype) ![:0]u8 {
    var out: std.Io.Writer.Allocating = .init(allocator);
    defer out.deinit();
    try out.writer.writeAll("{\"manifest\":{\"identity\":{");
    try writeJsonField(&out.writer, "family_name", response.familyName(), false);
    try writeJsonField(&out.writer, "uuid", response.uuid(), true);
    try out.writer.print("}},\"service_count\":{d}}}", .{response.serviceCount()});
    return try allocator.dupeZ(u8, out.written());
}

fn discoveryResultJson(result: *const discover_mod.DiscoverResult) ![:0]u8 {
    var out: std.Io.Writer.Allocating = .init(allocator);
    defer out.deinit();
    try out.writer.writeAll("{\"found\":[");
    for (result.found, 0..) |item, index| {
        if (index != 0) try out.writer.writeByte(',');
        try holonRefJson(&out.writer, item);
    }
    try out.writer.writeByte(']');
    if (result.error_message) |message| {
        try out.writer.writeAll(",\"error_message\":");
        try jsonString(&out.writer, message);
    }
    try out.writer.writeByte('}');
    return try allocator.dupeZ(u8, out.written());
}

fn holonRefJson(writer: *std.Io.Writer, item: discover_mod.HolonRef) !void {
    try writer.writeByte('{');
    try writeJsonField(writer, "url", item.url, false);
    if (item.info) |info| {
        try writer.writeAll(",\"info\":{");
        try writeJsonField(writer, "slug", info.slug, false);
        try writeJsonField(writer, "uuid", info.uuid, true);
        try writeJsonField(writer, "lang", info.lang, true);
        try writeJsonField(writer, "runner", info.runner, true);
        try writeJsonField(writer, "status", info.status, true);
        try writeJsonField(writer, "kind", info.kind, true);
        try writeJsonField(writer, "transport", info.transport, true);
        try writer.writeAll(",\"has_dist\":");
        try writer.writeAll(if (info.has_dist) "true" else "false");
        try writer.writeAll(",\"has_source\":");
        try writer.writeAll(if (info.has_source) "true" else "false");
        try writer.writeAll(",\"identity\":{");
        try writeJsonField(writer, "given_name", info.identity.given_name, false);
        try writeJsonField(writer, "family_name", info.identity.family_name, true);
        try writeJsonField(writer, "motto", info.identity.motto, true);
        try writer.writeAll("}}");
    }
    if (item.error_message) |message| {
        try writer.writeAll(",\"error_message\":");
        try jsonString(writer, message);
    }
    try writer.writeByte('}');
}

fn writeJsonField(writer: *std.Io.Writer, name: []const u8, value: []const u8, comma: bool) !void {
    if (comma) try writer.writeByte(',');
    try jsonString(writer, name);
    try writer.writeByte(':');
    try jsonString(writer, value);
}

fn jsonString(writer: *std.Io.Writer, value: []const u8) !void {
    try writer.writeByte('"');
    for (value) |ch| switch (ch) {
        '"' => try writer.writeAll("\\\""),
        '\\' => try writer.writeAll("\\\\"),
        '\n' => try writer.writeAll("\\n"),
        '\r' => try writer.writeAll("\\r"),
        '\t' => try writer.writeAll("\\t"),
        else => if (ch < 0x20) {
            try writer.print("\\u{x:0>4}", .{ch});
        } else {
            try writer.writeByte(ch);
        },
    };
    try writer.writeByte('"');
}

test "ABI version is exposed" {
    try std.testing.expectEqual(@as(c_uint, 0), holons_sdk_abi_version_major());
    try std.testing.expectEqual(@as(c_uint, 1), holons_sdk_abi_version_minor());
    try std.testing.expectEqual(@as(c_uint, 0), holons_sdk_abi_version_patch());
}

test "generated header avoids internal dependency types" {
    try std.testing.expect(std.mem.indexOf(u8, header, "ProtobufC") == null);
    try std.testing.expect(std.mem.indexOf(u8, header, "grpc_") == null);
}
