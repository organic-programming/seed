const std = @import("std");
const core = @import("core.zig");
const describe = @import("../describe.zig");
const transport = @import("../transport.zig");

const posix = @cImport({
    @cInclude("unistd.h");
});

pub const UnaryHandler = *const fn (std.mem.Allocator, []const u8) anyerror![]u8;

pub const Method = struct {
    path: []const u8,
    handler: UnaryHandler,
};

const RuntimeMethod = struct {
    path: [:0]u8,
    handler: UnaryHandler,
    registered: ?*anyopaque = null,
};

const TagKind = enum {
    request,
    response,
};

const CompletionTag = struct {
    kind: TagKind,
    data: *anyopaque,
};

const RequestTag = struct {
    method_index: usize,
    call: ?*core.c.grpc_call = null,
    deadline: core.c.gpr_timespec = undefined,
    metadata: core.c.grpc_metadata_array = undefined,
    payload: ?*core.c.grpc_byte_buffer = null,
};

const ResponseTag = struct {
    call: *core.c.grpc_call,
    response_bb: ?*core.c.grpc_byte_buffer = null,
    status_details: core.c.grpc_slice,
};

const describe_method = Method{
    .path = "/holons.v1.HolonMeta/Describe",
    .handler = describeHandler,
};

const shutdown_tag: *anyopaque = @ptrFromInt(0x484f4c4f4e5a5344);

pub const Server = struct {
    allocator: std.mem.Allocator,
    endpoint: transport.Endpoint,
    owned_uri: ?[]u8 = null,
    raw: *core.c.grpc_server,
    cq: *core.c.grpc_completion_queue,
    methods: []RuntimeMethod,
    stdio_bridge: ?transport.stdio.ServerBridge = null,
    worker: ?std.Thread = null,
    accepting: std.atomic.Value(bool),
    shutdown_requested: std.atomic.Value(bool),

    pub fn start(self: *Server) !void {
        for (self.methods) |*method| {
            method.registered = core.c.grpc_server_register_method(
                self.raw,
                method.path.ptr,
                null,
                core.c.GRPC_SRM_PAYLOAD_READ_INITIAL_BYTE_BUFFER,
                0,
            ) orelse return error.RegisterMethodFailed;
        }
        core.c.grpc_server_start(self.raw);
        if (self.stdio_bridge) |bridge| {
            const creds = core.c.grpc_insecure_server_credentials_create() orelse
                return error.CredentialsCreateFailed;
            defer core.c.grpc_server_credentials_release(creds);
            core.c.grpc_server_add_channel_from_fd(self.raw, bridge.socket_fd, creds);
        }
        self.accepting.store(true, .release);
        for (self.methods, 0..) |_, index| {
            try self.requestNext(index);
        }
        self.worker = try std.Thread.spawn(.{}, eventLoop, .{self});
    }

    pub fn shutdown(self: *Server) void {
        if (self.shutdown_requested.swap(true, .acq_rel)) return;
        self.accepting.store(false, .release);
        core.c.grpc_server_shutdown_and_notify(self.raw, self.cq, shutdown_tag);
    }

    pub fn wait(self: *Server) void {
        if (self.worker) |worker| {
            worker.join();
            self.worker = null;
        }
    }

    pub fn deinit(self: *Server) void {
        self.shutdown();
        self.wait();
        core.c.grpc_server_destroy(self.raw);
        core.c.grpc_completion_queue_destroy(self.cq);
        if (self.stdio_bridge) |*bridge| bridge.close();
        if (self.owned_uri) |owned_uri| self.allocator.free(owned_uri);
        for (self.methods) |method| self.allocator.free(method.path);
        self.allocator.free(self.methods);
        core.shutdown();
    }

    fn requestNext(self: *Server, method_index: usize) !void {
        var tag = try self.allocator.create(RequestTag);
        errdefer self.allocator.destroy(tag);
        tag.* = .{ .method_index = method_index };
        core.c.grpc_metadata_array_init(&tag.metadata);
        errdefer core.c.grpc_metadata_array_destroy(&tag.metadata);
        const completion = try self.allocator.create(CompletionTag);
        errdefer self.allocator.destroy(completion);
        completion.* = .{
            .kind = .request,
            .data = tag,
        };

        const err = core.c.grpc_server_request_registered_call(
            self.raw,
            self.methods[method_index].registered,
            &tag.call,
            &tag.deadline,
            &tag.metadata,
            &tag.payload,
            self.cq,
            self.cq,
            completion,
        );
        if (err != core.c.GRPC_CALL_OK) return error.RequestRegisteredCallFailed;
    }
};

pub fn bind(allocator: std.mem.Allocator, raw_uri: []const u8, methods: []const Method) !Server {
    var endpoint = try transport.parse(raw_uri);
    if (!transport.supportsServe(endpoint.scheme)) return error.UnsupportedListenTransport;

    core.init();
    errdefer core.shutdown();

    const raw = core.c.grpc_server_create(null, null) orelse return error.ServerCreateFailed;
    errdefer core.c.grpc_server_destroy(raw);

    const cq = core.c.grpc_completion_queue_create_for_next(null) orelse
        return error.CompletionQueueCreateFailed;
    errdefer core.c.grpc_completion_queue_destroy(cq);

    core.c.grpc_server_register_completion_queue(raw, cq, null);

    var stdio_bridge: ?transport.stdio.ServerBridge = null;
    errdefer if (stdio_bridge) |*bridge| bridge.close();
    var owned_uri: ?[]u8 = null;
    errdefer if (owned_uri) |uri| allocator.free(uri);

    switch (endpoint.scheme) {
        .tcp => {
            owned_uri = try addHttp2Port(allocator, raw, endpoint.address);
            if (owned_uri) |uri| endpoint = try transport.parse(uri);
        },
        .unix => try addUnixPort(allocator, raw, endpoint.address),
        .stdio => {
            stdio_bridge = try transport.stdio.openServerBridge();
        },
        .ws, .wss, .rest_sse => return error.UnsupportedListenTransport,
    }

    var runtime_methods = try allocator.alloc(RuntimeMethod, methods.len + 1);
    errdefer allocator.free(runtime_methods);
    runtime_methods[0] = .{
        .path = try allocator.dupeZ(u8, describe_method.path),
        .handler = describe_method.handler,
    };
    errdefer allocator.free(runtime_methods[0].path);
    for (methods, 0..) |method, index| {
        runtime_methods[index + 1] = .{
            .path = try allocator.dupeZ(u8, method.path),
            .handler = method.handler,
        };
    }
    errdefer {
        for (runtime_methods[1 .. methods.len + 1]) |method| allocator.free(method.path);
    }

    return .{
        .allocator = allocator,
        .endpoint = endpoint,
        .owned_uri = owned_uri,
        .raw = raw,
        .cq = cq,
        .methods = runtime_methods,
        .stdio_bridge = stdio_bridge,
        .accepting = std.atomic.Value(bool).init(false),
        .shutdown_requested = std.atomic.Value(bool).init(false),
    };
}

pub fn runBlocking(allocator: std.mem.Allocator, raw_uri: []const u8, methods: []const Method) !void {
    var server = try bind(allocator, raw_uri, methods);
    defer server.deinit();
    try server.start();
    server.wait();
}

fn addHttp2Port(allocator: std.mem.Allocator, raw: *core.c.grpc_server, address: []const u8) !?[]u8 {
    const hostless = std.mem.startsWith(u8, address, ":");
    const bind_address = if (hostless)
        try std.fmt.allocPrint(allocator, "0.0.0.0{s}", .{address})
    else
        try allocator.dupe(u8, address);
    defer allocator.free(bind_address);

    const target = try std.heap.c_allocator.dupeZ(u8, bind_address);
    defer std.heap.c_allocator.free(target);
    const creds = core.c.grpc_insecure_server_credentials_create() orelse
        return error.CredentialsCreateFailed;
    defer core.c.grpc_server_credentials_release(creds);
    const port = core.c.grpc_server_add_http2_port(raw, target.ptr, creds);
    if (port == 0) return error.BindFailed;
    const host_end = std.mem.lastIndexOfScalar(u8, bind_address, ':') orelse return null;
    const host = bind_address[0..host_end];
    const binds_all = hostless or std.mem.eql(u8, host, "0.0.0.0");
    if (!addressUsesEphemeralPort(address) and !binds_all) return null;
    const advertised_host = if (std.mem.eql(u8, host, "0.0.0.0")) "127.0.0.1" else host;
    return @as(?[]u8, try std.fmt.allocPrint(allocator, "tcp://{s}:{d}", .{ advertised_host, port }));
}

fn addressUsesEphemeralPort(address: []const u8) bool {
    const colon = std.mem.lastIndexOfScalar(u8, address, ':') orelse return false;
    return std.mem.eql(u8, address[colon + 1 ..], "0");
}

fn addUnixPort(allocator: std.mem.Allocator, raw: *core.c.grpc_server, path: []const u8) !void {
    const path_z = try allocator.dupeZ(u8, path);
    defer allocator.free(path_z);
    _ = posix.unlink(path_z.ptr);

    const target = try unixTarget(allocator, path);
    defer allocator.free(target);
    const creds = core.c.grpc_insecure_server_credentials_create() orelse
        return error.CredentialsCreateFailed;
    defer core.c.grpc_server_credentials_release(creds);
    if (core.c.grpc_server_add_http2_port(raw, target.ptr, creds) == 0) return error.BindFailed;
}

fn unixTarget(allocator: std.mem.Allocator, path: []const u8) ![:0]u8 {
    const prefix = "unix:";
    const target = try allocator.allocSentinel(u8, prefix.len + path.len, 0);
    @memcpy(target[0..prefix.len], prefix);
    @memcpy(target[prefix.len..], path);
    return target;
}

fn eventLoop(self: *Server) void {
    while (true) {
        const event = core.c.grpc_completion_queue_next(
            self.cq,
            core.deadlineAfterSeconds(86_400),
            null,
        );
        switch (event.type) {
            core.c.GRPC_QUEUE_SHUTDOWN => break,
            core.c.GRPC_OP_COMPLETE => {
                if (event.tag == shutdown_tag) {
                    core.c.grpc_completion_queue_shutdown(self.cq);
                    drainQueue(self.cq);
                    break;
                }
                const completion: *CompletionTag = @ptrCast(@alignCast(event.tag));
                switch (completion.kind) {
                    .request => handleRequest(self, completion, @ptrCast(@alignCast(completion.data)), event.success != 0),
                    .response => handleResponse(self, completion, @ptrCast(@alignCast(completion.data))),
                }
            },
            else => {},
        }
    }
}

fn drainQueue(cq: *core.c.grpc_completion_queue) void {
    while (true) {
        const event = core.c.grpc_completion_queue_next(cq, core.deadlineAfterMillis(250), null);
        if (event.type == core.c.GRPC_QUEUE_SHUTDOWN) return;
    }
}

fn handleRequest(self: *Server, completion: *CompletionTag, tag: *RequestTag, success: bool) void {
    defer {
        if (tag.payload) |payload| core.c.grpc_byte_buffer_destroy(payload);
        core.c.grpc_metadata_array_destroy(&tag.metadata);
        self.allocator.destroy(tag);
        self.allocator.destroy(completion);
    }

    const call = tag.call orelse return;

    if (!success) {
        core.c.grpc_call_unref(call);
        return;
    }
    if (self.accepting.load(.acquire)) {
        self.requestNext(tag.method_index) catch {};
    }

    const payload = tag.payload orelse {
        sendStatusAsync(self, call, core.c.GRPC_STATUS_INTERNAL, "missing request payload") catch
            core.c.grpc_call_unref(call);
        return;
    };
    const request = core.readByteBuffer(self.allocator, payload) catch {
        sendStatusAsync(self, call, core.c.GRPC_STATUS_INTERNAL, "failed to read request payload") catch
            core.c.grpc_call_unref(call);
        return;
    };
    defer self.allocator.free(request);

    const response = self.methods[tag.method_index].handler(self.allocator, request) catch {
        sendStatusAsync(self, call, core.c.GRPC_STATUS_INTERNAL, "handler failed") catch
            core.c.grpc_call_unref(call);
        return;
    };
    defer self.allocator.free(response);
    sendResponseAsync(self, call, response) catch core.c.grpc_call_unref(call);
}

fn handleResponse(self: *Server, completion: *CompletionTag, tag: *ResponseTag) void {
    if (tag.response_bb) |response_bb| core.c.grpc_byte_buffer_destroy(response_bb);
    core.c.grpc_slice_unref(tag.status_details);
    core.c.grpc_call_unref(tag.call);
    self.allocator.destroy(tag);
    self.allocator.destroy(completion);
}

fn sendResponseAsync(self: *Server, call: *core.c.grpc_call, response: []const u8) !void {
    var response_slice = core.c.grpc_slice_from_copied_buffer(@ptrCast(response.ptr), response.len);
    errdefer core.c.grpc_slice_unref(response_slice);
    const response_bb = core.c.grpc_raw_byte_buffer_create(&response_slice, 1) orelse
        return error.ByteBufferCreateFailed;
    errdefer core.c.grpc_byte_buffer_destroy(response_bb);

    var tag = try self.allocator.create(ResponseTag);
    errdefer self.allocator.destroy(tag);
    tag.* = .{
        .call = call,
        .response_bb = response_bb,
        .status_details = core.c.grpc_empty_slice(),
    };
    errdefer core.c.grpc_slice_unref(tag.status_details);
    const completion = try self.allocator.create(CompletionTag);
    errdefer self.allocator.destroy(completion);
    completion.* = .{
        .kind = .response,
        .data = tag,
    };

    var ops: [3]core.c.grpc_op = [_]core.c.grpc_op{std.mem.zeroes(core.c.grpc_op)} ** 3;
    ops[0].op = core.c.GRPC_OP_SEND_INITIAL_METADATA;
    ops[0].data.send_initial_metadata.count = 0;
    ops[0].data.send_initial_metadata.metadata = null;
    ops[1].op = core.c.GRPC_OP_SEND_MESSAGE;
    ops[1].data.send_message.send_message = response_bb;
    ops[2].op = core.c.GRPC_OP_SEND_STATUS_FROM_SERVER;
    ops[2].data.send_status_from_server.trailing_metadata_count = 0;
    ops[2].data.send_status_from_server.trailing_metadata = null;
    ops[2].data.send_status_from_server.status = core.c.GRPC_STATUS_OK;
    ops[2].data.send_status_from_server.status_details = &tag.status_details;

    if (core.c.grpc_call_start_batch(call, &ops, ops.len, completion, null) != core.c.GRPC_CALL_OK) {
        return error.CallStartBatchFailed;
    }
}

fn sendStatusAsync(
    self: *Server,
    call: *core.c.grpc_call,
    status: core.c.grpc_status_code,
    details: [:0]const u8,
) !void {
    var tag = try self.allocator.create(ResponseTag);
    errdefer self.allocator.destroy(tag);
    tag.* = .{
        .call = call,
        .status_details = core.c.grpc_slice_from_static_string(details.ptr),
    };
    errdefer core.c.grpc_slice_unref(tag.status_details);
    const completion = try self.allocator.create(CompletionTag);
    errdefer self.allocator.destroy(completion);
    completion.* = .{
        .kind = .response,
        .data = tag,
    };

    var ops: [2]core.c.grpc_op = [_]core.c.grpc_op{std.mem.zeroes(core.c.grpc_op)} ** 2;
    ops[0].op = core.c.GRPC_OP_SEND_INITIAL_METADATA;
    ops[0].data.send_initial_metadata.count = 0;
    ops[0].data.send_initial_metadata.metadata = null;
    ops[1].op = core.c.GRPC_OP_SEND_STATUS_FROM_SERVER;
    ops[1].data.send_status_from_server.trailing_metadata_count = 0;
    ops[1].data.send_status_from_server.trailing_metadata = null;
    ops[1].data.send_status_from_server.status = status;
    ops[1].data.send_status_from_server.status_details = &tag.status_details;

    if (core.c.grpc_call_start_batch(call, &ops, ops.len, completion, null) != core.c.GRPC_CALL_OK) {
        return error.CallStartBatchFailed;
    }
}

fn describeHandler(allocator: std.mem.Allocator, request: []const u8) anyerror![]u8 {
    _ = request;
    return describe.currentProtoAlloc(allocator);
}
