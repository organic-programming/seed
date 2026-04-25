const std = @import("std");
const describe = @import("describe.zig");
const grpc_server = @import("grpc/server.zig");
const transport = @import("transport.zig");

pub const Options = struct {
    listen_uri: []const u8 = "stdio://",
};

pub const Error = error{
    MissingListenValue,
    UnsupportedListenTransport,
} || transport.uri.ParseError || describe.Error;

pub fn parseOptions(args: []const []const u8) Error!Options {
    var options = Options{};
    var i: usize = 0;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "serve")) continue;
        if (std.mem.eql(u8, arg, "--listen")) {
            i += 1;
            if (i >= args.len) return error.MissingListenValue;
            options.listen_uri = args[i];
            continue;
        }
        if (std.mem.startsWith(u8, arg, "--listen=")) {
            options.listen_uri = arg["--listen=".len..];
        }
    }
    return options;
}

pub fn runSingle(options: Options) Error!void {
    _ = try describe.current();
    var server = try grpc_server.bind(options.listen_uri);
    if (!transport.supportsServe(server.endpoint.scheme)) return error.UnsupportedListenTransport;
    grpc_server.start(&server);
    grpc_server.shutdown(&server);
}

test "parse listen option" {
    const options = try parseOptions(&.{ "serve", "--listen", "tcp://127.0.0.1:9090" });
    try std.testing.expectEqualStrings("tcp://127.0.0.1:9090", options.listen_uri);
}
