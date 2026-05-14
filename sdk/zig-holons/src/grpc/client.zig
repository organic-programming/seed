const std = @import("std");
const core = @import("core.zig");
const runtime = @import("../protobuf/runtime.zig");
const transport = @import("../transport.zig");

pub const Channel = struct {
    allocator: std.mem.Allocator,
    endpoint: transport.Endpoint,
    raw: *core.c.grpc_channel,
    child: ?transport.stdio.Child = null,
    owned_target: ?[:0]u8 = null,

    pub fn deinit(self: *Channel) void {
        core.c.grpc_channel_destroy(self.raw);
        if (self.owned_target) |target| self.allocator.free(target);
        if (self.child) |*child| child.close();
    }

    pub fn unaryAlloc(
        self: *Channel,
        allocator: std.mem.Allocator,
        method: []const u8,
        request: []const u8,
        timeout_ms: i64,
    ) ![]u8 {
        return unaryAllocRaw(self.raw, allocator, method, request, timeout_ms);
    }

    pub fn serverStream(
        self: *Channel,
        allocator: std.mem.Allocator,
        method: []const u8,
        request: []const u8,
        timeout_ms: i64,
    ) !ServerStream {
        return serverStreamRaw(self.raw, allocator, method, request, timeout_ms);
    }

    pub fn describe(self: *Channel, allocator: std.mem.Allocator) !runtime.DescribeResponse {
        const request = try runtime.packDescribeRequest(allocator);
        defer allocator.free(request);
        const response = try self.unaryAlloc(
            allocator,
            "/holons.v1.HolonMeta/Describe",
            request,
            10_000,
        );
        defer allocator.free(response);
        return runtime.unpackDescribeResponse(response);
    }

    pub fn sayHello(
        self: *Channel,
        allocator: std.mem.Allocator,
        name: []const u8,
        lang_code: []const u8,
    ) !runtime.SayHelloResponse {
        const request = try runtime.packSayHelloRequest(allocator, name, lang_code);
        defer allocator.free(request);
        const response = try self.unaryAlloc(
            allocator,
            "/greeting.v1.GreetingService/SayHello",
            request,
            10_000,
        );
        defer allocator.free(response);
        return runtime.unpackSayHelloResponse(response);
    }

    pub fn listLanguages(self: *Channel, allocator: std.mem.Allocator) !runtime.ListLanguagesResponse {
        const request = try runtime.packListLanguagesRequest(allocator);
        defer allocator.free(request);
        const response = try self.unaryAlloc(
            allocator,
            "/greeting.v1.GreetingService/ListLanguages",
            request,
            10_000,
        );
        defer allocator.free(response);
        return runtime.unpackListLanguagesResponse(response);
    }
};

pub const ServerStream = struct {
    cq: *core.c.grpc_completion_queue,
    call: *core.c.grpc_call,
    initial_metadata: core.c.grpc_metadata_array,
    trailing_metadata: core.c.grpc_metadata_array,
    timeout_ms: i64,
    status_received: bool = false,
    status: core.c.grpc_status_code = core.c.GRPC_STATUS_OK,
    status_details: core.c.grpc_slice = undefined,
    error_string: [*c]const u8 = null,

    pub fn next(self: *ServerStream, allocator: std.mem.Allocator) !?[]u8 {
        if (self.status_received) {
            if (self.status != core.c.GRPC_STATUS_OK) return error.GrpcStatusNotOk;
            return null;
        }

        var response_bb: ?*core.c.grpc_byte_buffer = null;
        var ops: [1]core.c.grpc_op = [_]core.c.grpc_op{std.mem.zeroes(core.c.grpc_op)} ** 1;
        ops[0].op = core.c.GRPC_OP_RECV_MESSAGE;
        ops[0].data.recv_message.recv_message = &response_bb;

        var tag_token: u8 = 2;
        if (core.c.grpc_call_start_batch(self.call, &ops, ops.len, &tag_token, null) != core.c.GRPC_CALL_OK) {
            return error.CallStartBatchFailed;
        }
        var event = core.c.grpc_completion_queue_pluck(
            self.cq,
            &tag_token,
            core.deadlineAfterMillis(self.timeout_ms),
            null,
        );
        if (event.type != core.c.GRPC_OP_COMPLETE) {
            self.cancel();
            event = core.c.grpc_completion_queue_pluck(
                self.cq,
                &tag_token,
                core.deadlineAfterMillis(1_000),
                null,
            );
            if (event.type != core.c.GRPC_OP_COMPLETE) return error.CallCompletionFailed;
        }
        if (event.success == 0) {
            try self.receiveStatus();
            if (self.status != core.c.GRPC_STATUS_OK) return error.GrpcStatusNotOk;
            return null;
        }

        const bb = response_bb orelse {
            try self.receiveStatus();
            if (self.status != core.c.GRPC_STATUS_OK) return error.GrpcStatusNotOk;
            return null;
        };
        defer core.c.grpc_byte_buffer_destroy(bb);
        return @as(?[]u8, try core.readByteBuffer(allocator, bb));
    }

    pub fn cancel(self: *ServerStream) void {
        if (!self.status_received) {
            _ = core.c.grpc_call_cancel(self.call, null);
        }
    }

    pub fn deinit(self: *ServerStream) void {
        if (!self.status_received) {
            self.cancel();
        } else {
            core.c.grpc_slice_unref(self.status_details);
        }
        core.c.grpc_metadata_array_destroy(&self.initial_metadata);
        core.c.grpc_metadata_array_destroy(&self.trailing_metadata);
        core.c.grpc_call_unref(self.call);
        core.c.grpc_completion_queue_shutdown(self.cq);
        _ = core.c.grpc_completion_queue_pluck(self.cq, @ptrFromInt(999), core.deadlineAfterMillis(250), null);
        core.c.grpc_completion_queue_destroy(self.cq);
    }

    fn receiveStatus(self: *ServerStream) !void {
        if (self.status_received) return;
        var ops: [1]core.c.grpc_op = [_]core.c.grpc_op{std.mem.zeroes(core.c.grpc_op)} ** 1;
        ops[0].op = core.c.GRPC_OP_RECV_STATUS_ON_CLIENT;
        ops[0].data.recv_status_on_client.trailing_metadata = &self.trailing_metadata;
        ops[0].data.recv_status_on_client.status = &self.status;
        ops[0].data.recv_status_on_client.status_details = &self.status_details;
        ops[0].data.recv_status_on_client.error_string = &self.error_string;

        var tag_token: u8 = 3;
        if (core.c.grpc_call_start_batch(self.call, &ops, ops.len, &tag_token, null) != core.c.GRPC_CALL_OK) {
            return error.CallStartBatchFailed;
        }
        const event = core.c.grpc_completion_queue_pluck(
            self.cq,
            &tag_token,
            core.deadlineAfterMillis(self.timeout_ms),
            null,
        );
        if (event.type != core.c.GRPC_OP_COMPLETE or event.success == 0) return error.CallCompletionFailed;
        self.status_received = true;
    }
};

pub fn init() void {
    core.init();
}

pub fn shutdown() void {
    core.shutdown();
}

pub fn connect(allocator: std.mem.Allocator, raw_uri: []const u8) !Channel {
    const endpoint = try transport.parse(raw_uri);
    if (!transport.supportsDial(endpoint.scheme)) return error.UnsupportedScheme;
    return switch (endpoint.scheme) {
        .tcp => connectTcpEndpoint(allocator, endpoint),
        .stdio => error.StdioCommandRequired,
        .unix => connectUnixEndpoint(allocator, endpoint),
        .ws, .wss, .rest_sse => error.UnsupportedScheme,
    };
}

pub fn connectTcp(allocator: std.mem.Allocator, raw_uri: []const u8) !Channel {
    const endpoint = try transport.parse(raw_uri);
    if (endpoint.scheme != .tcp) return error.UnsupportedScheme;
    return connectTcpEndpoint(allocator, endpoint);
}

pub fn connectStdioCommand(allocator: std.mem.Allocator, command: transport.stdio.Command) !Channel {
    var child = try transport.stdio.spawnCommand(allocator, command);
    errdefer child.close();

    core.init();
    errdefer core.shutdown();

    const creds = core.c.grpc_insecure_credentials_create() orelse return error.CredentialsCreateFailed;
    defer core.c.grpc_channel_credentials_release(creds);

    const raw = core.c.grpc_channel_create_from_fd("stdio-zig", child.socket_fd, creds, null) orelse
        return error.ChannelCreateFromFdFailed;

    return .{
        .allocator = allocator,
        .endpoint = .{ .raw = "stdio://", .scheme = .stdio, .address = "" },
        .raw = raw,
        .child = child,
    };
}

fn connectTcpEndpoint(allocator: std.mem.Allocator, endpoint: transport.Endpoint) !Channel {
    core.init();
    errdefer core.shutdown();

    const creds = core.c.grpc_insecure_credentials_create() orelse return error.CredentialsCreateFailed;
    defer core.c.grpc_channel_credentials_release(creds);

    const target = try tcpTarget(allocator, endpoint.address);
    errdefer allocator.free(target);
    const raw = core.c.grpc_channel_create(target.ptr, creds, null) orelse return error.ChannelCreateFailed;
    return .{ .allocator = allocator, .endpoint = endpoint, .raw = raw, .owned_target = target };
}

fn tcpTarget(allocator: std.mem.Allocator, address: []const u8) ![:0]u8 {
    const prefix = if (isIpv4Address(tcpHost(address))) "ipv4:" else "dns:///";
    const target = try allocator.allocSentinel(u8, prefix.len + address.len, 0);
    @memcpy(target[0..prefix.len], prefix);
    @memcpy(target[prefix.len..], address);
    return target;
}

fn tcpHost(address: []const u8) []const u8 {
    const colon = std.mem.lastIndexOfScalar(u8, address, ':') orelse return address;
    return address[0..colon];
}

fn isIpv4Address(host: []const u8) bool {
    if (host.len == 0) return false;
    var dots: usize = 0;
    for (host) |ch| {
        switch (ch) {
            '0'...'9' => {},
            '.' => dots += 1,
            else => return false,
        }
    }
    return dots == 3;
}

fn connectUnixEndpoint(allocator: std.mem.Allocator, endpoint: transport.Endpoint) !Channel {
    core.init();
    errdefer core.shutdown();

    const creds = core.c.grpc_insecure_credentials_create() orelse return error.CredentialsCreateFailed;
    defer core.c.grpc_channel_credentials_release(creds);

    const target = try unixTarget(allocator, endpoint.address);
    errdefer allocator.free(target);
    const raw = core.c.grpc_channel_create(target.ptr, creds, null) orelse return error.ChannelCreateFailed;
    return .{ .allocator = allocator, .endpoint = endpoint, .raw = raw, .owned_target = target };
}

fn unixTarget(allocator: std.mem.Allocator, path: []const u8) ![:0]u8 {
    const prefix = "unix:";
    const target = try allocator.allocSentinel(u8, prefix.len + path.len, 0);
    @memcpy(target[0..prefix.len], prefix);
    @memcpy(target[prefix.len..], path);
    return target;
}

fn unaryAllocRaw(
    channel: *core.c.grpc_channel,
    allocator: std.mem.Allocator,
    method: []const u8,
    request: []const u8,
    timeout_ms: i64,
) ![]u8 {
    const cq = core.c.grpc_completion_queue_create_for_pluck(null) orelse
        return error.CompletionQueueCreateFailed;
    defer {
        core.c.grpc_completion_queue_shutdown(cq);
        _ = core.c.grpc_completion_queue_pluck(cq, @ptrFromInt(999), core.deadlineAfterMillis(250), null);
        core.c.grpc_completion_queue_destroy(cq);
    }

    var request_slice = core.c.grpc_slice_from_copied_buffer(@ptrCast(request.ptr), request.len);
    defer core.c.grpc_slice_unref(request_slice);
    const request_bb = core.c.grpc_raw_byte_buffer_create(&request_slice, 1) orelse
        return error.ByteBufferCreateFailed;
    defer core.c.grpc_byte_buffer_destroy(request_bb);

    const method_slice = core.c.grpc_slice_from_copied_buffer(@ptrCast(method.ptr), method.len);
    defer core.c.grpc_slice_unref(method_slice);
    const call = core.c.grpc_channel_create_call(
        channel,
        null,
        core.c.GRPC_PROPAGATE_DEFAULTS,
        cq,
        method_slice,
        null,
        core.deadlineAfterMillis(timeout_ms),
        null,
    ) orelse return error.CallCreateFailed;
    defer core.c.grpc_call_unref(call);

    var initial_metadata: core.c.grpc_metadata_array = undefined;
    var trailing_metadata: core.c.grpc_metadata_array = undefined;
    core.c.grpc_metadata_array_init(&initial_metadata);
    core.c.grpc_metadata_array_init(&trailing_metadata);
    defer core.c.grpc_metadata_array_destroy(&initial_metadata);
    defer core.c.grpc_metadata_array_destroy(&trailing_metadata);

    var response_bb: ?*core.c.grpc_byte_buffer = null;
    var status: core.c.grpc_status_code = undefined;
    var status_details: core.c.grpc_slice = undefined;
    var error_string: [*c]const u8 = null;

    var ops: [6]core.c.grpc_op = [_]core.c.grpc_op{std.mem.zeroes(core.c.grpc_op)} ** 6;
    ops[0].op = core.c.GRPC_OP_SEND_INITIAL_METADATA;
    ops[0].data.send_initial_metadata.count = 0;
    ops[0].data.send_initial_metadata.metadata = null;
    ops[1].op = core.c.GRPC_OP_SEND_MESSAGE;
    ops[1].data.send_message.send_message = request_bb;
    ops[2].op = core.c.GRPC_OP_SEND_CLOSE_FROM_CLIENT;
    ops[3].op = core.c.GRPC_OP_RECV_INITIAL_METADATA;
    ops[3].data.recv_initial_metadata.recv_initial_metadata = &initial_metadata;
    ops[4].op = core.c.GRPC_OP_RECV_MESSAGE;
    ops[4].data.recv_message.recv_message = &response_bb;
    ops[5].op = core.c.GRPC_OP_RECV_STATUS_ON_CLIENT;
    ops[5].data.recv_status_on_client.trailing_metadata = &trailing_metadata;
    ops[5].data.recv_status_on_client.status = &status;
    ops[5].data.recv_status_on_client.status_details = &status_details;
    ops[5].data.recv_status_on_client.error_string = &error_string;

    var tag_token: u8 = 1;
    if (core.c.grpc_call_start_batch(call, &ops, ops.len, &tag_token, null) != core.c.GRPC_CALL_OK) {
        return error.CallStartBatchFailed;
    }

    const event = core.c.grpc_completion_queue_pluck(cq, &tag_token, core.deadlineAfterMillis(timeout_ms), null);
    if (event.type != core.c.GRPC_OP_COMPLETE or event.success == 0) return error.CallCompletionFailed;
    defer core.c.grpc_slice_unref(status_details);

    if (status != core.c.GRPC_STATUS_OK) {
        if (error_string != null) {
            std.debug.print("grpc status={} error={s}\n", .{ status, error_string });
        } else {
            std.debug.print("grpc status={}\n", .{status});
        }
        return error.GrpcStatusNotOk;
    }
    const bb = response_bb orelse return error.NoResponseMessage;
    defer core.c.grpc_byte_buffer_destroy(bb);
    return core.readByteBuffer(allocator, bb);
}

fn serverStreamRaw(
    channel: *core.c.grpc_channel,
    allocator: std.mem.Allocator,
    method: []const u8,
    request: []const u8,
    timeout_ms: i64,
) !ServerStream {
    _ = allocator;
    const cq = core.c.grpc_completion_queue_create_for_pluck(null) orelse
        return error.CompletionQueueCreateFailed;
    errdefer {
        core.c.grpc_completion_queue_shutdown(cq);
        _ = core.c.grpc_completion_queue_pluck(cq, @ptrFromInt(999), core.deadlineAfterMillis(250), null);
        core.c.grpc_completion_queue_destroy(cq);
    }

    var request_slice = core.c.grpc_slice_from_copied_buffer(@ptrCast(request.ptr), request.len);
    defer core.c.grpc_slice_unref(request_slice);
    const request_bb = core.c.grpc_raw_byte_buffer_create(&request_slice, 1) orelse
        return error.ByteBufferCreateFailed;
    defer core.c.grpc_byte_buffer_destroy(request_bb);

    const method_slice = core.c.grpc_slice_from_copied_buffer(@ptrCast(method.ptr), method.len);
    defer core.c.grpc_slice_unref(method_slice);
    const call = core.c.grpc_channel_create_call(
        channel,
        null,
        core.c.GRPC_PROPAGATE_DEFAULTS,
        cq,
        method_slice,
        null,
        core.deadlineAfterMillis(timeout_ms),
        null,
    ) orelse return error.CallCreateFailed;
    errdefer core.c.grpc_call_unref(call);

    var initial_metadata: core.c.grpc_metadata_array = undefined;
    var trailing_metadata: core.c.grpc_metadata_array = undefined;
    core.c.grpc_metadata_array_init(&initial_metadata);
    errdefer core.c.grpc_metadata_array_destroy(&initial_metadata);
    core.c.grpc_metadata_array_init(&trailing_metadata);
    errdefer core.c.grpc_metadata_array_destroy(&trailing_metadata);

    var ops: [4]core.c.grpc_op = [_]core.c.grpc_op{std.mem.zeroes(core.c.grpc_op)} ** 4;
    ops[0].op = core.c.GRPC_OP_SEND_INITIAL_METADATA;
    ops[0].data.send_initial_metadata.count = 0;
    ops[0].data.send_initial_metadata.metadata = null;
    ops[1].op = core.c.GRPC_OP_SEND_MESSAGE;
    ops[1].data.send_message.send_message = request_bb;
    ops[2].op = core.c.GRPC_OP_SEND_CLOSE_FROM_CLIENT;
    ops[3].op = core.c.GRPC_OP_RECV_INITIAL_METADATA;
    ops[3].data.recv_initial_metadata.recv_initial_metadata = &initial_metadata;

    var tag_token: u8 = 1;
    if (core.c.grpc_call_start_batch(call, &ops, ops.len, &tag_token, null) != core.c.GRPC_CALL_OK) {
        return error.CallStartBatchFailed;
    }
    const event = core.c.grpc_completion_queue_pluck(
        cq,
        &tag_token,
        core.deadlineAfterMillis(timeout_ms),
        null,
    );
    if (event.type != core.c.GRPC_OP_COMPLETE or event.success == 0) return error.CallCompletionFailed;

    return .{
        .cq = cq,
        .call = call,
        .initial_metadata = initial_metadata,
        .trailing_metadata = trailing_metadata,
        .timeout_ms = timeout_ms,
    };
}
