const std = @import("std");
const holons = @import("zig_holons");

const describe_generated = @import("describe_generated");
const greetings = @import("greetings.zig");

pub fn main(init: std.process.Init.Minimal) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.c_allocator);
    defer arena.deinit();
    const args = try init.args.toSlice(arena.allocator());

    holons.describe.useStaticResponse(describe_generated.staticDescribeResponse());

    var options = try holons.serve.parseOptions(args[1..]);
    options.methods = greetings.methods[0..];
    try holons.serve.runSingle(options);
}
