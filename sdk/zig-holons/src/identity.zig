const std = @import("std");

pub const PROTO_MANIFEST_FILE_NAME = "holon.proto";

pub const Error = error{
    MissingManifest,
    MissingSlug,
    InvalidManifestPath,
};

pub const Identity = struct {
    slug: []const u8,
    name: []const u8,
    version: []const u8,
};

pub const HolonIdentity = struct {
    uuid: []const u8 = "",
    given_name: []const u8 = "",
    family_name: []const u8 = "",
    motto: []const u8 = "",
    composer: []const u8 = "",
    clade: []const u8 = "",
    status: []const u8 = "",
    born: []const u8 = "",
    version: []const u8 = "",
    lang: []const u8 = "",
    parents: []const []const u8 = &.{},
    reproduction: []const u8 = "",
    generated_by: []const u8 = "",
    proto_status: []const u8 = "",
    aliases: []const []const u8 = &.{},

    pub fn deinit(self: *HolonIdentity, allocator: std.mem.Allocator) void {
        allocator.free(self.uuid);
        allocator.free(self.given_name);
        allocator.free(self.family_name);
        allocator.free(self.motto);
        allocator.free(self.composer);
        allocator.free(self.clade);
        allocator.free(self.status);
        allocator.free(self.born);
        allocator.free(self.version);
        allocator.free(self.lang);
        freeStringSlice(allocator, self.parents);
        allocator.free(self.reproduction);
        allocator.free(self.generated_by);
        allocator.free(self.proto_status);
        freeStringSlice(allocator, self.aliases);
        self.* = .{};
    }

    pub fn slugAlloc(self: HolonIdentity, allocator: std.mem.Allocator) ![]u8 {
        return slugForAlloc(allocator, self.given_name, self.family_name);
    }
};

pub const ResolvedSkill = struct {
    name: []const u8 = "",
    description: []const u8 = "",
    when: []const u8 = "",
    steps: []const []const u8 = &.{},

    pub fn deinit(self: *ResolvedSkill, allocator: std.mem.Allocator) void {
        allocator.free(self.name);
        allocator.free(self.description);
        allocator.free(self.when);
        freeStringSlice(allocator, self.steps);
        self.* = .{};
    }
};

pub const ResolvedSequenceParam = struct {
    name: []const u8 = "",
    description: []const u8 = "",
    required: bool = false,
    default: []const u8 = "",

    pub fn deinit(self: *ResolvedSequenceParam, allocator: std.mem.Allocator) void {
        allocator.free(self.name);
        allocator.free(self.description);
        allocator.free(self.default);
        self.* = .{};
    }
};

pub const ResolvedSequence = struct {
    name: []const u8 = "",
    description: []const u8 = "",
    params: []ResolvedSequenceParam = &.{},
    steps: []const []const u8 = &.{},

    pub fn deinit(self: *ResolvedSequence, allocator: std.mem.Allocator) void {
        allocator.free(self.name);
        allocator.free(self.description);
        for (self.params) |*param| param.deinit(allocator);
        allocator.free(self.params);
        freeStringSlice(allocator, self.steps);
        self.* = .{};
    }
};

pub const ResolvedManifest = struct {
    identity: HolonIdentity = .{},
    description: []const u8 = "",
    kind: []const u8 = "",
    build_runner: []const u8 = "",
    build_main: []const u8 = "",
    artifact_binary: []const u8 = "",
    artifact_primary: []const u8 = "",
    required_files: []const []const u8 = &.{},
    member_paths: []const []const u8 = &.{},
    skills: []ResolvedSkill = &.{},
    sequences: []ResolvedSequence = &.{},

    pub fn deinit(self: *ResolvedManifest, allocator: std.mem.Allocator) void {
        self.identity.deinit(allocator);
        allocator.free(self.description);
        allocator.free(self.kind);
        allocator.free(self.build_runner);
        allocator.free(self.build_main);
        allocator.free(self.artifact_binary);
        allocator.free(self.artifact_primary);
        freeStringSlice(allocator, self.required_files);
        freeStringSlice(allocator, self.member_paths);
        for (self.skills) |*skill| skill.deinit(allocator);
        allocator.free(self.skills);
        for (self.sequences) |*sequence| sequence.deinit(allocator);
        allocator.free(self.sequences);
        self.* = .{};
    }
};

pub fn fromManifestFields(slug: []const u8, name: []const u8, version: []const u8) Error!Identity {
    if (slug.len == 0) return error.MissingSlug;
    return .{ .slug = slug, .name = name, .version = version };
}

pub fn parseHolon(allocator: std.mem.Allocator, path: []const u8) !HolonIdentity {
    var manifest = try resolveProtoFile(allocator, path);
    errdefer manifest.deinit(allocator);
    const identity = manifest.identity;
    manifest.identity = .{};
    manifest.deinit(allocator);
    return identity;
}

pub fn resolve(allocator: std.mem.Allocator, root: []const u8) !ResolvedManifest {
    const path = try resolveManifestPath(allocator, root);
    defer allocator.free(path);
    return resolveProtoFile(allocator, path);
}

pub fn parseManifest(allocator: std.mem.Allocator, path: []const u8) !ResolvedManifest {
    return resolveProtoFile(allocator, path);
}

pub fn resolveProtoFile(allocator: std.mem.Allocator, path: []const u8) !ResolvedManifest {
    if (!std.mem.eql(u8, std.fs.path.basename(path), PROTO_MANIFEST_FILE_NAME)) {
        return error.InvalidManifestPath;
    }

    const text = try std.Io.Dir.cwd().readFileAlloc(std.Options.debug_io, path, allocator, .limited(16 * 1024 * 1024));
    defer allocator.free(text);

    const manifest_block = try extractManifestBlock(text) orelse return error.MissingManifest;
    const identity_block = extractBlock("identity", manifest_block) orelse "";
    const lineage_block = extractBlock("lineage", manifest_block) orelse "";
    const build_block = extractBlock("build", manifest_block) orelse "";
    const requires_block = extractBlock("requires", manifest_block) orelse "";
    const artifacts_block = extractBlock("artifacts", manifest_block) orelse "";

    var member_paths_list: std.ArrayList([]const u8) = .empty;
    errdefer freeArrayListStrings(allocator, &member_paths_list);
    const member_blocks = try extractBlocks(allocator, "members", build_block);
    defer freeStringSlice(allocator, member_blocks);
    for (member_blocks) |member_block| {
        const path_value = try scalarAlloc(allocator, "path", member_block);
        if (std.mem.trim(u8, path_value, " \t\r\n").len == 0) {
            allocator.free(path_value);
        } else {
            try member_paths_list.append(allocator, path_value);
        }
    }

    const skill_blocks = try extractBlocks(allocator, "skills", manifest_block);
    defer freeStringSlice(allocator, skill_blocks);
    var skills: std.ArrayList(ResolvedSkill) = .empty;
    errdefer {
        for (skills.items) |*skill| skill.deinit(allocator);
        skills.deinit(allocator);
    }
    for (skill_blocks) |skill_block| {
        try skills.append(allocator, .{
            .name = try scalarAlloc(allocator, "name", skill_block),
            .description = try scalarAlloc(allocator, "description", skill_block),
            .when = try scalarAlloc(allocator, "when", skill_block),
            .steps = try compactStrings(allocator, try stringListAlloc(allocator, "steps", skill_block)),
        });
    }

    const sequence_blocks = try extractBlocks(allocator, "sequences", manifest_block);
    defer freeStringSlice(allocator, sequence_blocks);
    var sequences: std.ArrayList(ResolvedSequence) = .empty;
    errdefer {
        for (sequences.items) |*sequence| sequence.deinit(allocator);
        sequences.deinit(allocator);
    }
    for (sequence_blocks) |sequence_block| {
        const param_blocks = try extractBlocks(allocator, "params", sequence_block);
        defer freeStringSlice(allocator, param_blocks);
        var params: std.ArrayList(ResolvedSequenceParam) = .empty;
        errdefer {
            for (params.items) |*param| param.deinit(allocator);
            params.deinit(allocator);
        }
        for (param_blocks) |param_block| {
            try params.append(allocator, .{
                .name = try scalarAlloc(allocator, "name", param_block),
                .description = try scalarAlloc(allocator, "description", param_block),
                .required = boolean("required", param_block),
                .default = try scalarAlloc(allocator, "default", param_block),
            });
        }
        try sequences.append(allocator, .{
            .name = try scalarAlloc(allocator, "name", sequence_block),
            .description = try scalarAlloc(allocator, "description", sequence_block),
            .params = try params.toOwnedSlice(allocator),
            .steps = try compactStrings(allocator, try stringListAlloc(allocator, "steps", sequence_block)),
        });
    }

    return .{
        .identity = .{
            .uuid = try scalarAlloc(allocator, "uuid", identity_block),
            .given_name = try scalarAlloc(allocator, "given_name", identity_block),
            .family_name = try scalarAlloc(allocator, "family_name", identity_block),
            .motto = try scalarAlloc(allocator, "motto", identity_block),
            .composer = try scalarAlloc(allocator, "composer", identity_block),
            .clade = try scalarAlloc(allocator, "clade", identity_block),
            .status = try scalarAlloc(allocator, "status", identity_block),
            .born = try scalarAlloc(allocator, "born", identity_block),
            .version = try scalarAlloc(allocator, "version", identity_block),
            .lang = try scalarAlloc(allocator, "lang", manifest_block),
            .parents = try stringListAlloc(allocator, "parents", lineage_block),
            .reproduction = try scalarAlloc(allocator, "reproduction", lineage_block),
            .generated_by = try scalarAlloc(allocator, "generated_by", lineage_block),
            .proto_status = try scalarAlloc(allocator, "proto_status", identity_block),
            .aliases = try stringListAlloc(allocator, "aliases", identity_block),
        },
        .description = try scalarAlloc(allocator, "description", manifest_block),
        .kind = try scalarAlloc(allocator, "kind", manifest_block),
        .build_runner = try scalarAlloc(allocator, "runner", build_block),
        .build_main = try scalarAlloc(allocator, "main", build_block),
        .artifact_binary = try scalarAlloc(allocator, "binary", artifacts_block),
        .artifact_primary = try scalarAlloc(allocator, "primary", artifacts_block),
        .required_files = try compactStrings(allocator, try stringListAlloc(allocator, "files", requires_block)),
        .member_paths = try member_paths_list.toOwnedSlice(allocator),
        .skills = try skills.toOwnedSlice(allocator),
        .sequences = try sequences.toOwnedSlice(allocator),
    };
}

pub fn findHolonProto(allocator: std.mem.Allocator, root: []const u8) !?[]u8 {
    const stat = std.Io.Dir.cwd().statFile(std.Options.debug_io, root, .{}) catch |err| switch (err) {
        error.FileNotFound => return null,
        else => return err,
    };
    if (stat.kind == .file) {
        if (std.mem.eql(u8, std.fs.path.basename(root), PROTO_MANIFEST_FILE_NAME)) {
            return allocator.dupe(u8, root);
        }
        return null;
    }
    if (stat.kind != .directory) return null;

    const direct = try std.fs.path.join(allocator, &.{ root, PROTO_MANIFEST_FILE_NAME });
    defer allocator.free(direct);
    if (existsFile(direct)) return allocator.dupe(u8, direct);

    const api_v1 = try std.fs.path.join(allocator, &.{ root, "api", "v1", PROTO_MANIFEST_FILE_NAME });
    defer allocator.free(api_v1);
    if (existsFile(api_v1)) return allocator.dupe(u8, api_v1);

    var candidates: std.ArrayList([]u8) = .empty;
    defer {
        for (candidates.items) |candidate| allocator.free(candidate);
        candidates.deinit(allocator);
    }
    try walkForHolonProto(allocator, root, &candidates);
    if (candidates.items.len == 0) return null;
    std.mem.sort([]u8, candidates.items, {}, struct {
        fn lessThan(_: void, a: []u8, b: []u8) bool {
            return std.mem.lessThan(u8, a, b);
        }
    }.lessThan);
    const out = candidates.items[0];
    candidates.items[0] = &.{};
    return out;
}

pub fn resolveManifestPath(allocator: std.mem.Allocator, root: []const u8) ![]u8 {
    if (try findHolonProto(allocator, root)) |path| return path;

    const parent = std.fs.path.dirname(root) orelse "";
    if (parent.len != 0 and !std.mem.eql(u8, parent, root)) {
        if (try findHolonProto(allocator, parent)) |path| return path;
    }
    return error.FileNotFound;
}

pub fn inferSlugFromProtoText(proto_text: []const u8) Error![]const u8 {
    if (extractManifestBlock(proto_text) catch null) |manifest_block| {
        const identity_block = extractBlock("identity", manifest_block) orelse "";
        const given = scalarBorrowed("given_name", identity_block);
        const family = scalarBorrowed("family_name", identity_block);
        if (given.len != 0 or family.len != 0) {
            return error.MissingSlug;
        }
    }
    const marker = "slug:";
    const idx = std.mem.indexOf(u8, proto_text, marker) orelse return error.MissingManifest;
    var rest = std.mem.trimStart(u8, proto_text[idx + marker.len ..], " \t\r\n\"");
    const end = std.mem.indexOfAny(u8, rest, "\"\r\n") orelse rest.len;
    rest = rest[0..end];
    if (rest.len == 0) return error.MissingSlug;
    return rest;
}

pub fn slugForAlloc(allocator: std.mem.Allocator, given_name: []const u8, family_name: []const u8) ![]u8 {
    const given = std.mem.trim(u8, given_name, " \t\r\n");
    var family = std.mem.trim(u8, family_name, " \t\r\n");
    while (family.len > 0 and family[family.len - 1] == '?') {
        family = family[0 .. family.len - 1];
    }
    if (given.len == 0 and family.len == 0) return allocator.dupe(u8, "");
    const joined = try std.fmt.allocPrint(allocator, "{s}-{s}", .{ given, family });
    defer allocator.free(joined);
    const lower = try std.ascii.allocLowerString(allocator, std.mem.trim(u8, joined, " \t\r\n"));
    for (lower) |*ch| {
        if (ch.* == ' ') ch.* = '-';
    }
    const trimmed = std.mem.trim(u8, lower, "-");
    if (trimmed.ptr == lower.ptr and trimmed.len == lower.len) return lower;
    const out = try allocator.dupe(u8, trimmed);
    allocator.free(lower);
    return out;
}

fn existsFile(path: []const u8) bool {
    const stat = std.Io.Dir.cwd().statFile(std.Options.debug_io, path, .{}) catch return false;
    return stat.kind == .file;
}

fn walkForHolonProto(allocator: std.mem.Allocator, root: []const u8, out: *std.ArrayList([]u8)) !void {
    var dir = std.Io.Dir.cwd().openDir(std.Options.debug_io, root, .{ .iterate = true }) catch return;
    defer dir.close(std.Options.debug_io);
    var it = dir.iterate();
    while (try it.next(std.Options.debug_io)) |entry| {
        const child = try std.fs.path.join(allocator, &.{ root, entry.name });
        errdefer allocator.free(child);
        switch (entry.kind) {
            .directory => {
                try walkForHolonProto(allocator, child, out);
                allocator.free(child);
            },
            .file => {
                if (std.mem.eql(u8, entry.name, PROTO_MANIFEST_FILE_NAME)) {
                    try out.append(allocator, child);
                } else {
                    allocator.free(child);
                }
            },
            else => allocator.free(child),
        }
    }
}

fn extractManifestBlock(source: []const u8) !?[]const u8 {
    const marker = "holons.v1.manifest";
    const marker_index = std.mem.indexOf(u8, source, marker) orelse return null;
    const brace_offset = std.mem.indexOfScalar(u8, source[marker_index..], '{') orelse return null;
    return balancedBlockContents(source, marker_index + brace_offset);
}

fn extractBlock(name: []const u8, source: []const u8) ?[]const u8 {
    var offset: usize = 0;
    while (std.mem.indexOf(u8, source[offset..], name)) |relative| {
        const start = offset + relative;
        if (!isTokenBoundary(source, start, name.len)) {
            offset = start + name.len;
            continue;
        }
        var cursor = start + name.len;
        cursor = skipWhitespace(source, cursor);
        if (cursor >= source.len or source[cursor] != ':') {
            offset = start + name.len;
            continue;
        }
        cursor = skipWhitespace(source, cursor + 1);
        if (cursor >= source.len or source[cursor] != '{') {
            offset = start + name.len;
            continue;
        }
        return balancedBlockContents(source, cursor);
    }
    return null;
}

fn extractBlocks(allocator: std.mem.Allocator, name: []const u8, source: []const u8) ![]const []const u8 {
    var blocks: std.ArrayList([]const u8) = .empty;
    errdefer freeArrayListStrings(allocator, &blocks);
    var offset: usize = 0;
    while (std.mem.indexOf(u8, source[offset..], name)) |relative| {
        const start = offset + relative;
        if (!isTokenBoundary(source, start, name.len)) {
            offset = start + name.len;
            continue;
        }
        var cursor = start + name.len;
        cursor = skipWhitespace(source, cursor);
        if (cursor >= source.len or source[cursor] != ':') {
            offset = start + name.len;
            continue;
        }
        cursor = skipWhitespace(source, cursor + 1);
        if (cursor >= source.len or source[cursor] != '{') {
            offset = start + name.len;
            continue;
        }
        const block = balancedBlockContents(source, cursor) orelse break;
        try blocks.append(allocator, try allocator.dupe(u8, block));
        offset = cursor + block.len + 2;
    }
    return blocks.toOwnedSlice(allocator);
}

fn balancedBlockContents(source: []const u8, opening_brace: usize) ?[]const u8 {
    var depth: usize = 0;
    var inside_string = false;
    var escaped = false;
    const content_start = opening_brace + 1;
    var index = opening_brace;
    while (index < source.len) : (index += 1) {
        const ch = source[index];
        if (inside_string) {
            if (escaped) {
                escaped = false;
            } else if (ch == '\\') {
                escaped = true;
            } else if (ch == '"') {
                inside_string = false;
            }
            continue;
        }

        if (ch == '"') {
            inside_string = true;
        } else if (ch == '{') {
            depth += 1;
        } else if (ch == '}') {
            depth -= 1;
            if (depth == 0) return source[content_start..index];
        }
    }
    return null;
}

fn scalarAlloc(allocator: std.mem.Allocator, name: []const u8, source: []const u8) ![]const u8 {
    return allocator.dupe(u8, scalarBorrowed(name, source));
}

fn scalarBorrowed(name: []const u8, source: []const u8) []const u8 {
    var offset: usize = 0;
    while (std.mem.indexOf(u8, source[offset..], name)) |relative| {
        const start = offset + relative;
        if (!isTokenBoundary(source, start, name.len)) {
            offset = start + name.len;
            continue;
        }
        var cursor = skipWhitespace(source, start + name.len);
        if (cursor >= source.len or source[cursor] != ':') {
            offset = start + name.len;
            continue;
        }
        cursor = skipWhitespace(source, cursor + 1);
        if (cursor >= source.len) return "";
        if (source[cursor] == '"') {
            const value_start = cursor + 1;
            cursor = value_start;
            var escaped = false;
            while (cursor < source.len) : (cursor += 1) {
                const ch = source[cursor];
                if (escaped) {
                    escaped = false;
                } else if (ch == '\\') {
                    escaped = true;
                } else if (ch == '"') {
                    return source[value_start..cursor];
                }
            }
            return "";
        }
        const value_start = cursor;
        while (cursor < source.len) : (cursor += 1) {
            switch (source[cursor]) {
                ' ', '\t', '\r', '\n', ',', ']', '}' => break,
                else => {},
            }
        }
        return source[value_start..cursor];
    }
    return "";
}

fn boolean(name: []const u8, source: []const u8) bool {
    const value = scalarBorrowed(name, source);
    return std.ascii.eqlIgnoreCase(value, "true") or std.mem.eql(u8, value, "1");
}

fn stringListAlloc(allocator: std.mem.Allocator, name: []const u8, source: []const u8) ![]const []const u8 {
    var offset: usize = 0;
    while (std.mem.indexOf(u8, source[offset..], name)) |relative| {
        const start = offset + relative;
        if (!isTokenBoundary(source, start, name.len)) {
            offset = start + name.len;
            continue;
        }
        var cursor = skipWhitespace(source, start + name.len);
        if (cursor >= source.len or source[cursor] != ':') {
            offset = start + name.len;
            continue;
        }
        cursor = skipWhitespace(source, cursor + 1);
        if (cursor >= source.len or source[cursor] != '[') {
            offset = start + name.len;
            continue;
        }
        const close = findListClose(source, cursor) orelse return allocator.alloc([]const u8, 0);
        return parseStringListBody(allocator, source[cursor + 1 .. close]);
    }
    return allocator.alloc([]const u8, 0);
}

fn parseStringListBody(allocator: std.mem.Allocator, body: []const u8) ![]const []const u8 {
    var values: std.ArrayList([]const u8) = .empty;
    errdefer freeArrayListStrings(allocator, &values);
    var cursor: usize = 0;
    while (cursor < body.len) {
        cursor = skipSeparators(body, cursor);
        if (cursor >= body.len) break;
        if (body[cursor] == '"') {
            const start = cursor + 1;
            cursor = start;
            var escaped = false;
            while (cursor < body.len) : (cursor += 1) {
                const ch = body[cursor];
                if (escaped) {
                    escaped = false;
                } else if (ch == '\\') {
                    escaped = true;
                } else if (ch == '"') {
                    try values.append(allocator, try allocator.dupe(u8, body[start..cursor]));
                    cursor += 1;
                    break;
                }
            }
        } else {
            const start = cursor;
            while (cursor < body.len) : (cursor += 1) {
                switch (body[cursor]) {
                    ' ', '\t', '\r', '\n', ',' => break,
                    else => {},
                }
            }
            if (cursor > start) try values.append(allocator, try allocator.dupe(u8, body[start..cursor]));
        }
    }
    return values.toOwnedSlice(allocator);
}

fn findListClose(source: []const u8, opening_bracket: usize) ?usize {
    var inside_string = false;
    var escaped = false;
    var depth: usize = 0;
    var index = opening_bracket;
    while (index < source.len) : (index += 1) {
        const ch = source[index];
        if (inside_string) {
            if (escaped) escaped = false else if (ch == '\\') escaped = true else if (ch == '"') inside_string = false;
            continue;
        }
        if (ch == '"') {
            inside_string = true;
        } else if (ch == '[') {
            depth += 1;
        } else if (ch == ']') {
            depth -= 1;
            if (depth == 0) return index;
        }
    }
    return null;
}

fn compactStrings(allocator: std.mem.Allocator, values: []const []const u8) ![]const []const u8 {
    var out: std.ArrayList([]const u8) = .empty;
    errdefer freeArrayListStrings(allocator, &out);
    for (values) |value| {
        const trimmed = std.mem.trim(u8, value, " \t\r\n");
        if (trimmed.len == 0) continue;
        try out.append(allocator, try allocator.dupe(u8, trimmed));
    }
    freeStringSlice(allocator, values);
    return out.toOwnedSlice(allocator);
}

fn isTokenBoundary(source: []const u8, start: usize, len: usize) bool {
    if (start > 0 and isIdent(source[start - 1])) return false;
    const end = start + len;
    if (end < source.len and isIdent(source[end])) return false;
    return true;
}

fn isIdent(ch: u8) bool {
    return std.ascii.isAlphanumeric(ch) or ch == '_';
}

fn skipWhitespace(source: []const u8, start: usize) usize {
    var cursor = start;
    while (cursor < source.len and std.ascii.isWhitespace(source[cursor])) : (cursor += 1) {}
    return cursor;
}

fn skipSeparators(source: []const u8, start: usize) usize {
    var cursor = start;
    while (cursor < source.len) : (cursor += 1) {
        switch (source[cursor]) {
            ' ', '\t', '\r', '\n', ',' => {},
            else => break,
        }
    }
    return cursor;
}

fn freeStringSlice(allocator: std.mem.Allocator, values: []const []const u8) void {
    for (values) |value| allocator.free(value);
    allocator.free(values);
}

fn freeArrayListStrings(allocator: std.mem.Allocator, values: *std.ArrayList([]const u8)) void {
    for (values.items) |value| allocator.free(value);
    values.deinit(allocator);
}

test "parse holon manifest fields" {
    const allocator = std.testing.allocator;
    var dir = std.testing.tmpDir(.{});
    defer dir.cleanup();
    try dir.dir.writeFile(std.testing.io, .{
        .sub_path = "holon.proto",
        .data =
        \\syntax = "proto3";
        \\package test.v1;
        \\option (holons.v1.manifest) = {
        \\  identity: {
        \\    uuid: "abc-123"
        \\    given_name: "Gabriel"
        \\    family_name: "Greeting?"
        \\    motto: "A test."
        \\    version: "1.2.3"
        \\    aliases: ["zig-greeting", "gzig"]
        \\  }
        \\  lineage: {
        \\    parents: ["a", "b"]
        \\    generated_by: "unit"
        \\  }
        \\  description: "Build-time manifest metadata."
        \\  lang: "zig"
        \\  kind: "native"
        \\  build: { runner: "zig" main: "src/main.zig" members: { path: "members/daemon" } }
        \\  requires: { files: ["build.zig"] }
        \\  artifacts: { binary: "gabriel-greeting-zig" primary: "gabriel-greeting-zig.holon" }
        \\  skills: { name: "hello" description: "Say hello." when: "Needed." steps: ["Call SayHello"] }
        \\  sequences: {
        \\    name: "greet-once"
        \\    params: { name: "name" description: "Caller." required: true default: "Bob" }
        \\    steps: ["op gabriel-greeting-zig SayHello"]
        \\  }
        \\};
        ,
    });
    const path = try std.fs.path.join(allocator, &.{ ".zig-cache", "tmp", dir.sub_path[0..], "holon.proto" });
    defer allocator.free(path);
    var manifest = try resolveProtoFile(allocator, path);
    defer manifest.deinit(allocator);

    try std.testing.expectEqualStrings("abc-123", manifest.identity.uuid);
    try std.testing.expectEqualStrings("zig", manifest.identity.lang);
    const slug = try manifest.identity.slugAlloc(allocator);
    defer allocator.free(slug);
    try std.testing.expectEqualStrings("gabriel-greeting", slug);
    try std.testing.expectEqualStrings("zig-greeting", manifest.identity.aliases[0]);
    try std.testing.expectEqualStrings("build.zig", manifest.required_files[0]);
    try std.testing.expectEqualStrings("members/daemon", manifest.member_paths[0]);
    try std.testing.expectEqualStrings("hello", manifest.skills[0].name);
    try std.testing.expect(manifest.sequences[0].params[0].required);
}
