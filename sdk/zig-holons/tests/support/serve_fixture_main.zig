const std = @import("std");
const holons = @import("zig_holons");
const greeting = @import("zig_greeting_server.zig");

pub fn main(init: std.process.Init.Minimal) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.c_allocator);
    defer arena.deinit();
    const args = try init.args.toSlice(arena.allocator());

    greeting.registerDescribe();
    const parsed = try holons.serve.parseOptions(args[1..]);
    try holons.serve.runSingle(.{
        .listen_uri = parsed.listen_uri,
        .methods = greeting.methods[0..],
    });
}
