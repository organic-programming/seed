//! Holon-RPC client support.
//!
//! Rust-parity transport coverage is intentionally dial-only for WebSocket,
//! WebSocket-over-TLS, and HTTP+SSE. The wire format is JSON-RPC 2.0 with the
//! `holon-rpc` WebSocket subprotocol.

const std = @import("std");
const transport = @import("transport.zig");

const json_rpc_version = "2.0";
const websocket_protocol = "holon-rpc";
const max_frame_payload = 1 << 20;

pub const TransportMode = enum {
    websocket,
    http,
};

pub const NormalizedUrl = struct {
    mode: TransportMode,
    url: []u8,

    pub fn deinit(self: NormalizedUrl, allocator: std.mem.Allocator) void {
        allocator.free(self.url);
    }
};

pub const ConnectOptions = struct {
    /// PEM bundle containing extra CA certificates trusted for wss://.
    tls_ca_file: ?[]const u8 = null,
};

pub const Request = struct {
    id: []const u8,
    method: []const u8,
    payload_json: []const u8,
};

pub const Response = struct {
    id: []const u8,
    payload_json: []const u8,
};

pub const ResponseError = struct {
    code: i64,
    message: []u8,
    data_json: ?[]u8 = null,

    pub fn deinit(self: *ResponseError, allocator: std.mem.Allocator) void {
        allocator.free(self.message);
        if (self.data_json) |data| allocator.free(data);
        self.* = undefined;
    }
};

pub const InvokeResult = struct {
    result_json: []u8,

    pub fn deinit(self: *InvokeResult, allocator: std.mem.Allocator) void {
        allocator.free(self.result_json);
        self.* = undefined;
    }
};

pub const SSEEvent = struct {
    event: []u8,
    id: []u8,
    result_json: []u8,
    error_json: ?[]u8 = null,

    pub fn deinit(self: *SSEEvent, allocator: std.mem.Allocator) void {
        allocator.free(self.event);
        allocator.free(self.id);
        allocator.free(self.result_json);
        if (self.error_json) |error_json| allocator.free(error_json);
        self.* = undefined;
    }
};

pub const SSEEvents = struct {
    items: []SSEEvent,

    pub fn deinit(self: *SSEEvents, allocator: std.mem.Allocator) void {
        for (self.items) |*event| event.deinit(allocator);
        allocator.free(self.items);
        self.* = undefined;
    }
};

pub const Client = struct {
    allocator: std.mem.Allocator,
    http_client: *std.http.Client,
    connection: *std.http.Client.Connection,
    next_client_id: u64 = 0,

    pub fn connect(allocator: std.mem.Allocator, raw_uri: []const u8) !Client {
        return connectWithOptions(allocator, raw_uri, .{});
    }

    pub fn connectWithOptions(
        allocator: std.mem.Allocator,
        raw_uri: []const u8,
        options: ConnectOptions,
    ) !Client {
        const normalized = try normalizeWebSocketUrl(allocator, raw_uri);
        defer normalized.deinit(allocator);

        const http_client = try allocator.create(std.http.Client);
        errdefer allocator.destroy(http_client);
        http_client.* = .{
            .allocator = allocator,
            .io = std.Io.Threaded.global_single_threaded.io(),
        };
        errdefer http_client.deinit();

        if (options.tls_ca_file) |ca_file| {
            const now = std.Io.Clock.real.now(http_client.io);
            http_client.now = now;
            try http_client.ca_bundle.addCertsFromFilePathAbsolute(
                allocator,
                http_client.io,
                now,
                ca_file,
            );
        }

        const uri = try std.Uri.parse(normalized.url);
        var key: [24]u8 = undefined;
        try makeWebSocketKey(&key, http_client.io);

        const headers = [_]std.http.Header{
            .{ .name = "Upgrade", .value = "websocket" },
            .{ .name = "Sec-WebSocket-Version", .value = "13" },
            .{ .name = "Sec-WebSocket-Key", .value = &key },
            .{ .name = "Sec-WebSocket-Protocol", .value = websocket_protocol },
        };

        var request = try http_client.request(.GET, uri, .{
            .headers = .{ .connection = .{ .override = "Upgrade" } },
            .extra_headers = &headers,
            .keep_alive = false,
            .redirect_behavior = .unhandled,
        });
        errdefer request.deinit();

        try request.sendBodiless();
        var redirect_buffer: [0]u8 = .{};
        const response = try request.receiveHead(&redirect_buffer);
        if (response.head.status != .switching_protocols) return error.WebSocketUpgradeFailed;
        if (!headerContains(response.head, "upgrade", "websocket")) return error.WebSocketUpgradeFailed;
        if (!headerContains(response.head, "sec-websocket-protocol", websocket_protocol)) return error.WebSocketSubprotocolRejected;

        const connection = request.connection.?;
        request.connection = null;
        http_client.connection_pool.used.remove(&connection.pool_node);
        request.deinit();

        return .{
            .allocator = allocator,
            .http_client = http_client,
            .connection = connection,
        };
    }

    pub fn deinit(self: *Client) void {
        _ = self.writeClose() catch {};
        self.connection.destroy(self.http_client.io);
        self.http_client.deinit();
        self.allocator.destroy(self.http_client);
        self.* = undefined;
    }

    pub fn invokeAlloc(
        self: *Client,
        allocator: std.mem.Allocator,
        method: []const u8,
        params_json: []const u8,
    ) !InvokeResult {
        self.next_client_id += 1;
        const request_id = try std.fmt.allocPrint(allocator, "c{}", .{self.next_client_id});
        defer allocator.free(request_id);

        const frame = try encodeJsonRpcRequest(allocator, .{
            .id = request_id,
            .method = method,
            .payload_json = params_json,
        });
        defer allocator.free(frame);
        try self.writeText(frame);

        while (true) {
            const payload = try self.readMessageAlloc(allocator);
            defer allocator.free(payload);
            var parsed = try std.json.parseFromSlice(std.json.Value, allocator, payload, .{});
            defer parsed.deinit();
            const root = parsed.value.object;

            if (root.get("method") != null) {
                try self.replyMethodNotFound(allocator, root);
                continue;
            }

            const id_value = root.get("id") orelse continue;
            if (id_value != .string) continue;
            if (!std.mem.eql(u8, id_value.string, request_id)) continue;
            if (!hasJsonRpcVersion(root)) return error.InvalidResponse;

            if (root.get("error")) |error_value| {
                try decodeResponseError(allocator, error_value);
                unreachable;
            }

            const result_value = root.get("result") orelse return .{
                .result_json = try allocator.dupe(u8, "{}"),
            };
            return .{ .result_json = try stringifyJsonValue(allocator, result_value) };
        }
    }

    fn replyMethodNotFound(
        self: *Client,
        allocator: std.mem.Allocator,
        request: std.json.ObjectMap,
    ) !void {
        const id_json = if (request.get("id")) |id| try stringifyJsonValue(allocator, id) else "null";
        defer if (request.get("id") != null) allocator.free(id_json);
        const response = try std.fmt.allocPrint(
            allocator,
            "{{\"jsonrpc\":\"2.0\",\"id\":{s},\"error\":{{\"code\":-32601,\"message\":\"method not found\"}}}}",
            .{id_json},
        );
        defer allocator.free(response);
        try self.writeText(response);
    }

    fn writeText(self: *Client, payload: []const u8) !void {
        try self.writeFrame(.text, payload);
    }

    fn writeClose(self: *Client) !void {
        try self.writeFrame(.close, "");
    }

    fn writePong(self: *Client, payload: []const u8) !void {
        try self.writeFrame(.pong, payload);
    }

    fn writeFrame(self: *Client, opcode: Opcode, payload: []const u8) !void {
        if (payload.len > max_frame_payload) return error.MessageTooLarge;
        var header: [14]u8 = undefined;
        var index: usize = 0;
        header[index] = @as(u8, 0x80) | @as(u8, @intCast(@intFromEnum(opcode)));
        index += 1;
        if (payload.len <= 125) {
            header[index] = 0x80 | @as(u8, @intCast(payload.len));
            index += 1;
        } else if (payload.len <= std.math.maxInt(u16)) {
            header[index] = 0x80 | 126;
            index += 1;
            std.mem.writeInt(u16, header[index..][0..2], @intCast(payload.len), .big);
            index += 2;
        } else {
            header[index] = 0x80 | 127;
            index += 1;
            std.mem.writeInt(u64, header[index..][0..8], @intCast(payload.len), .big);
            index += 8;
        }

        var mask: [4]u8 = undefined;
        self.http_client.io.random(&mask);
        @memcpy(header[index..][0..4], &mask);
        index += 4;

        const masked = try self.allocator.alloc(u8, payload.len);
        defer self.allocator.free(masked);
        for (payload, 0..) |byte, i| masked[i] = byte ^ mask[i % 4];

        const writer = self.connection.writer();
        try writer.writeAll(header[0..index]);
        try writer.writeAll(masked);
        try self.connection.flush();
    }

    fn readMessageAlloc(self: *Client, allocator: std.mem.Allocator) ![]u8 {
        while (true) {
            const frame = try self.readFrameAlloc(allocator);
            switch (frame.opcode) {
                .text, .binary => return frame.payload,
                .ping => {
                    defer allocator.free(frame.payload);
                    try self.writePong(frame.payload);
                },
                .pong => allocator.free(frame.payload),
                .close => {
                    allocator.free(frame.payload);
                    return error.ConnectionClosed;
                },
                .continuation => {
                    allocator.free(frame.payload);
                    return error.FragmentedFramesUnsupported;
                },
                _ => {
                    allocator.free(frame.payload);
                    return error.UnsupportedOpcode;
                },
            }
        }
    }

    fn readFrameAlloc(self: *Client, allocator: std.mem.Allocator) !Frame {
        var header: [2]u8 = undefined;
        try self.connection.reader().readSliceAll(&header);
        const opcode: Opcode = @enumFromInt(header[0] & 0x0f);
        const masked = (header[1] & 0x80) != 0;
        var length: u64 = header[1] & 0x7f;
        if (length == 126) {
            var ext: [2]u8 = undefined;
            try self.connection.reader().readSliceAll(&ext);
            length = std.mem.readInt(u16, &ext, .big);
        } else if (length == 127) {
            var ext: [8]u8 = undefined;
            try self.connection.reader().readSliceAll(&ext);
            length = std.mem.readInt(u64, &ext, .big);
        }
        if (length > max_frame_payload) return error.MessageTooLarge;

        var mask: [4]u8 = .{ 0, 0, 0, 0 };
        if (masked) try self.connection.reader().readSliceAll(&mask);

        const payload = try allocator.alloc(u8, @intCast(length));
        errdefer allocator.free(payload);
        try self.connection.reader().readSliceAll(payload);
        if (masked) {
            for (payload, 0..) |*byte, i| byte.* ^= mask[i % 4];
        }
        return .{ .opcode = opcode, .payload = payload };
    }
};

pub const HTTPClient = struct {
    allocator: std.mem.Allocator,
    http_client: std.http.Client,
    base_url: []u8,

    pub fn init(allocator: std.mem.Allocator, raw_uri: []const u8) !HTTPClient {
        return initWithOptions(allocator, raw_uri, .{});
    }

    pub fn initWithOptions(
        allocator: std.mem.Allocator,
        raw_uri: []const u8,
        options: ConnectOptions,
    ) !HTTPClient {
        const normalized = try normalizeHttpUrl(allocator, raw_uri);
        errdefer normalized.deinit(allocator);

        var http_client: std.http.Client = .{
            .allocator = allocator,
            .io = std.Io.Threaded.global_single_threaded.io(),
        };
        errdefer http_client.deinit();

        if (options.tls_ca_file) |ca_file| {
            const now = std.Io.Clock.real.now(http_client.io);
            http_client.now = now;
            try http_client.ca_bundle.addCertsFromFilePathAbsolute(
                allocator,
                http_client.io,
                now,
                ca_file,
            );
        }

        return .{
            .allocator = allocator,
            .http_client = http_client,
            .base_url = normalized.url,
        };
    }

    pub fn deinit(self: *HTTPClient) void {
        self.allocator.free(self.base_url);
        self.http_client.deinit();
        self.* = undefined;
    }

    pub fn invokeAlloc(
        self: *HTTPClient,
        allocator: std.mem.Allocator,
        method: []const u8,
        params_json: []const u8,
    ) !InvokeResult {
        const url = try self.methodUrl(allocator, method, null);
        defer allocator.free(url);

        const body = try self.fetchAlloc(allocator, .POST, url, params_json, "application/json");
        defer allocator.free(body);
        return .{ .result_json = try decodeHttpResult(allocator, body) };
    }

    pub fn streamAlloc(
        self: *HTTPClient,
        allocator: std.mem.Allocator,
        method: []const u8,
        params_json: []const u8,
    ) !SSEEvents {
        const url = try self.methodUrl(allocator, method, null);
        defer allocator.free(url);
        const body = try self.fetchAlloc(allocator, .POST, url, params_json, "text/event-stream");
        defer allocator.free(body);
        return parseSSEEvents(allocator, body);
    }

    pub fn streamQueryAlloc(
        self: *HTTPClient,
        allocator: std.mem.Allocator,
        method: []const u8,
        query: []const u8,
    ) !SSEEvents {
        const url = try self.methodUrl(allocator, method, query);
        defer allocator.free(url);
        const body = try self.fetchAlloc(allocator, .GET, url, null, "text/event-stream");
        defer allocator.free(body);
        return parseSSEEvents(allocator, body);
    }

    fn methodUrl(
        self: *HTTPClient,
        allocator: std.mem.Allocator,
        method: []const u8,
        query: ?[]const u8,
    ) ![]u8 {
        const trimmed = std.mem.trim(u8, method, "/ \t\r\n");
        if (trimmed.len == 0) return error.MethodRequired;
        if (query) |q| {
            if (q.len > 0) return std.fmt.allocPrint(allocator, "{s}/{s}?{s}", .{ self.base_url, trimmed, q });
        }
        return std.fmt.allocPrint(allocator, "{s}/{s}", .{ self.base_url, trimmed });
    }

    fn fetchAlloc(
        self: *HTTPClient,
        allocator: std.mem.Allocator,
        method: std.http.Method,
        url: []const u8,
        payload: ?[]const u8,
        accept: []const u8,
    ) ![]u8 {
        var response_body: std.Io.Writer.Allocating = .init(allocator);
        defer response_body.deinit();

        const extra_headers = [_]std.http.Header{
            .{ .name = "Accept", .value = accept },
        };
        const result = try self.http_client.fetch(.{
            .location = .{ .url = url },
            .method = method,
            .payload = payload,
            .response_writer = &response_body.writer,
            .keep_alive = false,
            .headers = .{ .content_type = .{ .override = "application/json" } },
            .extra_headers = &extra_headers,
            .redirect_behavior = .unhandled,
        });
        if (result.status.class() == .client_error or result.status.class() == .server_error) {
            return error.HttpStatusError;
        }
        return response_body.toOwnedSlice();
    }
};

const Opcode = enum(u4) {
    continuation = 0x0,
    text = 0x1,
    binary = 0x2,
    close = 0x8,
    ping = 0x9,
    pong = 0xa,
    _,
};

const Frame = struct {
    opcode: Opcode,
    payload: []u8,
};

pub fn normalizeTransportUrl(allocator: std.mem.Allocator, raw_uri: []const u8) !NormalizedUrl {
    const trimmed = std.mem.trim(u8, raw_uri, " \t\r\n");
    if (trimmed.len == 0) return error.EmptyUri;
    if (std.mem.startsWith(u8, trimmed, "ws://") or std.mem.startsWith(u8, trimmed, "wss://")) {
        return .{ .mode = .websocket, .url = try allocator.dupe(u8, trimmed) };
    }
    if (std.mem.startsWith(u8, trimmed, "http://") or std.mem.startsWith(u8, trimmed, "https://")) {
        return .{ .mode = .http, .url = try trimTrailingSlash(allocator, trimmed) };
    }
    if (std.mem.startsWith(u8, trimmed, "rest+sse://")) {
        const rest = trimmed["rest+sse://".len..];
        const no_slash = std.mem.trimEnd(u8, rest, "/");
        return .{ .mode = .http, .url = try std.fmt.allocPrint(allocator, "http://{s}", .{no_slash}) };
    }
    return error.UnsupportedScheme;
}

pub fn normalizeWebSocketUrl(allocator: std.mem.Allocator, raw_uri: []const u8) !NormalizedUrl {
    const normalized = try normalizeTransportUrl(allocator, raw_uri);
    errdefer normalized.deinit(allocator);
    if (normalized.mode != .websocket) return error.UnsupportedScheme;
    return normalized;
}

pub fn normalizeHttpUrl(allocator: std.mem.Allocator, raw_uri: []const u8) !NormalizedUrl {
    const normalized = try normalizeTransportUrl(allocator, raw_uri);
    errdefer normalized.deinit(allocator);
    if (normalized.mode != .http) return error.UnsupportedScheme;
    return normalized;
}

pub fn encodeRequest(allocator: std.mem.Allocator, request: Request) ![]u8 {
    return encodeJsonRpcRequest(allocator, request);
}

pub fn encodeJsonRpcRequest(allocator: std.mem.Allocator, request: Request) ![]u8 {
    return std.fmt.allocPrint(
        allocator,
        "{{\"jsonrpc\":\"2.0\",\"id\":\"{s}\",\"method\":\"{s}\",\"params\":{s}}}",
        .{ request.id, request.method, request.payload_json },
    );
}

fn decodeHttpResult(allocator: std.mem.Allocator, body: []const u8) ![]u8 {
    var parsed = try std.json.parseFromSlice(std.json.Value, allocator, body, .{});
    defer parsed.deinit();
    if (parsed.value != .object) return try allocator.dupe(u8, body);
    const root = parsed.value.object;
    if (root.get("error")) |error_value| {
        try decodeResponseError(allocator, error_value);
        unreachable;
    }
    if (root.get("result")) |result_value| return stringifyJsonValue(allocator, result_value);
    return try allocator.dupe(u8, body);
}

fn decodeResponseError(allocator: std.mem.Allocator, error_value: std.json.Value) !void {
    _ = allocator;
    _ = error_value;
    return error.RpcError;
}

fn parseSSEEvents(allocator: std.mem.Allocator, body: []const u8) !SSEEvents {
    var events: std.ArrayList(SSEEvent) = .empty;
    errdefer {
        for (events.items) |*event| event.deinit(allocator);
        events.deinit(allocator);
    }

    var event_name: []const u8 = "";
    var event_id: []const u8 = "";
    var data: std.Io.Writer.Allocating = .init(allocator);
    defer data.deinit();

    var line_it = std.mem.splitScalar(u8, body, '\n');
    while (line_it.next()) |raw_line| {
        const line = std.mem.trimEnd(u8, raw_line, "\r");
        if (line.len == 0) {
            try flushSSEEvent(allocator, &events, &event_name, &event_id, &data);
            continue;
        }
        if (std.mem.startsWith(u8, line, "event:")) {
            event_name = std.mem.trim(u8, line["event:".len..], " \t");
        } else if (std.mem.startsWith(u8, line, "id:")) {
            event_id = std.mem.trim(u8, line["id:".len..], " \t");
        } else if (std.mem.startsWith(u8, line, "data:")) {
            if (data.written().len > 0) try data.writer.writeByte('\n');
            try data.writer.writeAll(std.mem.trim(u8, line["data:".len..], " \t"));
        }
    }
    try flushSSEEvent(allocator, &events, &event_name, &event_id, &data);

    return .{ .items = try events.toOwnedSlice(allocator) };
}

fn flushSSEEvent(
    allocator: std.mem.Allocator,
    events: *std.ArrayList(SSEEvent),
    event_name: *[]const u8,
    event_id: *[]const u8,
    data: *std.Io.Writer.Allocating,
) !void {
    if (event_name.*.len == 0 and event_id.*.len == 0 and data.written().len == 0) return;

    const name = if (event_name.*.len == 0) "message" else event_name.*;
    var result_json = try allocator.dupe(u8, "{}");
    var error_json: ?[]u8 = null;
    errdefer {
        allocator.free(result_json);
        if (error_json) |err_json| allocator.free(err_json);
    }

    const payload = data.written();
    if ((std.mem.eql(u8, name, "message") or std.mem.eql(u8, name, "error")) and payload.len > 0) {
        var parsed = try std.json.parseFromSlice(std.json.Value, allocator, payload, .{});
        defer parsed.deinit();
        if (parsed.value == .object) {
            const root = parsed.value.object;
            if (root.get("error")) |error_value| {
                allocator.free(result_json);
                result_json = try allocator.dupe(u8, "{}");
                error_json = try stringifyJsonValue(allocator, error_value);
            } else if (root.get("result")) |result_value| {
                allocator.free(result_json);
                result_json = try stringifyJsonValue(allocator, result_value);
            }
        }
    }

    try events.append(allocator, .{
        .event = try allocator.dupe(u8, name),
        .id = try allocator.dupe(u8, event_id.*),
        .result_json = result_json,
        .error_json = error_json,
    });
    event_name.* = "";
    event_id.* = "";
    data.clearRetainingCapacity();
}

fn stringifyJsonValue(allocator: std.mem.Allocator, value: std.json.Value) ![]u8 {
    var writer: std.Io.Writer.Allocating = .init(allocator);
    defer writer.deinit();
    try std.json.Stringify.value(value, .{}, &writer.writer);
    return writer.toOwnedSlice();
}

fn hasJsonRpcVersion(root: std.json.ObjectMap) bool {
    const jsonrpc = root.get("jsonrpc") orelse return false;
    return jsonrpc == .string and std.mem.eql(u8, jsonrpc.string, json_rpc_version);
}

fn headerContains(head: std.http.Client.Response.Head, name: []const u8, value: []const u8) bool {
    var headers = head.iterateHeaders();
    while (headers.next()) |header| {
        if (!std.ascii.eqlIgnoreCase(header.name, name)) continue;
        var values = std.mem.splitScalar(u8, header.value, ',');
        while (values.next()) |candidate| {
            if (std.ascii.eqlIgnoreCase(std.mem.trim(u8, candidate, " \t"), value)) return true;
        }
    }
    return false;
}

fn makeWebSocketKey(out: *[24]u8, io: std.Io) !void {
    var nonce: [16]u8 = undefined;
    io.random(&nonce);
    const encoded = std.base64.standard.Encoder.encode(out, &nonce);
    if (encoded.len != out.len) return error.InvalidWebSocketKey;
}

fn trimTrailingSlash(allocator: std.mem.Allocator, input: []const u8) ![]u8 {
    return allocator.dupe(u8, std.mem.trimEnd(u8, input, "/"));
}

test "encode JSON-RPC request frame" {
    const encoded = try encodeRequest(std.testing.allocator, .{
        .id = "1",
        .method = "HolonMeta.Describe",
        .payload_json = "{}",
    });
    defer std.testing.allocator.free(encoded);
    try std.testing.expectEqualStrings(
        "{\"jsonrpc\":\"2.0\",\"id\":\"1\",\"method\":\"HolonMeta.Describe\",\"params\":{}}",
        encoded,
    );
}

test "normalize Holon-RPC transport aliases" {
    const normalized = try normalizeTransportUrl(std.testing.allocator, "rest+sse://127.0.0.1:8080/api/v1/rpc/");
    defer normalized.deinit(std.testing.allocator);
    try std.testing.expectEqual(TransportMode.http, normalized.mode);
    try std.testing.expectEqualStrings("http://127.0.0.1:8080/api/v1/rpc", normalized.url);
}
