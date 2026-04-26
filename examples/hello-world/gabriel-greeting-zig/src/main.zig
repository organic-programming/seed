const std = @import("std");
const holons = @import("zig_holons");

const describe_generated = @import("describe_generated");
const greetings = @import("greetings.zig");

const c = @cImport({
    @cInclude("unistd.h");
});

pub fn main(init: std.process.Init.Minimal) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.c_allocator);
    defer arena.deinit();
    const args = try init.args.toSlice(arena.allocator());

    holons.describe.useStaticResponse(describe_generated.staticDescribeResponse());

    if (args.len > 1 and std.mem.eql(u8, args[1], "version")) {
        const output = try std.fmt.allocPrint(
            arena.allocator(),
            "gabriel-greeting-zig {s}\n",
            .{describe_generated.holonVersion},
        );
        try writeAll(c.STDOUT_FILENO, output);
        return;
    }

    var options = try holons.serve.parseOptions(args[1..]);
    options.methods = greetings.methods[0..];
    try holons.serve.runSingle(options);
}

fn writeAll(fd: c_int, bytes: []const u8) !void {
    var offset: usize = 0;
    while (offset < bytes.len) {
        const written = c.write(fd, bytes[offset..].ptr, bytes.len - offset);
        if (written < 0) return error.WriteFailed;
        if (written == 0) return error.WriteFailed;
        offset += @intCast(written);
    }
}
