const std = @import("std");
const identity_mod = @import("identity.zig");

pub const LOCAL: i32 = 0;
pub const PROXY: i32 = 1;
pub const DELEGATED: i32 = 2;

pub const SIBLINGS: i32 = 0x01;
pub const CWD: i32 = 0x02;
pub const SOURCE: i32 = 0x04;
pub const BUILT: i32 = 0x08;
pub const INSTALLED: i32 = 0x10;
pub const CACHED: i32 = 0x20;
pub const ALL: i32 = 0x3f;

pub const NO_LIMIT: i32 = 0;
pub const NO_TIMEOUT: u32 = 0;

pub const Scope = enum {
    siblings,
    cwd,
    source,
    built,
    installed,
    cached,
};

pub const IdentityInfo = struct {
    given_name: []const u8 = "",
    family_name: []const u8 = "",
    motto: []const u8 = "",
    aliases: []const []const u8 = &.{},

    pub fn deinit(self: *IdentityInfo, allocator: std.mem.Allocator) void {
        allocator.free(self.given_name);
        allocator.free(self.family_name);
        allocator.free(self.motto);
        freeStringSlice(allocator, self.aliases);
        self.* = .{};
    }
};

pub const HolonInfo = struct {
    slug: []const u8 = "",
    uuid: []const u8 = "",
    identity: IdentityInfo = .{},
    lang: []const u8 = "",
    runner: []const u8 = "",
    status: []const u8 = "",
    kind: []const u8 = "",
    transport: []const u8 = "",
    entrypoint: []const u8 = "",
    architectures: []const []const u8 = &.{},
    has_dist: bool = false,
    has_source: bool = false,

    pub fn deinit(self: *HolonInfo, allocator: std.mem.Allocator) void {
        allocator.free(self.slug);
        allocator.free(self.uuid);
        self.identity.deinit(allocator);
        allocator.free(self.lang);
        allocator.free(self.runner);
        allocator.free(self.status);
        allocator.free(self.kind);
        allocator.free(self.transport);
        allocator.free(self.entrypoint);
        freeStringSlice(allocator, self.architectures);
        self.* = .{};
    }
};

pub const HolonRef = struct {
    url: []const u8 = "",
    info: ?HolonInfo = null,
    error_message: ?[]const u8 = null,

    pub fn deinit(self: *HolonRef, allocator: std.mem.Allocator) void {
        allocator.free(self.url);
        if (self.info) |*info| info.deinit(allocator);
        if (self.error_message) |message| allocator.free(message);
        self.* = .{};
    }
};

pub const DiscoverResult = struct {
    found: []HolonRef = &.{},
    error_message: ?[]const u8 = null,

    pub fn deinit(self: *DiscoverResult, allocator: std.mem.Allocator) void {
        for (self.found) |*ref| ref.deinit(allocator);
        allocator.free(self.found);
        if (self.error_message) |message| allocator.free(message);
        self.* = .{};
    }
};

pub const ResolveResult = struct {
    ref: ?HolonRef = null,
    error_message: ?[]const u8 = null,

    pub fn deinit(self: *ResolveResult, allocator: std.mem.Allocator) void {
        if (self.ref) |*ref| ref.deinit(allocator);
        if (self.error_message) |message| allocator.free(message);
        self.* = .{};
    }
};

const DiscoveredEntry = struct {
    holon_ref: HolonRef,
    dir_path: []const u8,
    relative_path: []const u8,

    fn deinit(self: *DiscoveredEntry, allocator: std.mem.Allocator) void {
        self.holon_ref.deinit(allocator);
        allocator.free(self.dir_path);
        allocator.free(self.relative_path);
        self.* = undefined;
    }
};

pub fn discover(
    allocator: std.mem.Allocator,
    scope: i32,
    expression: ?[]const u8,
    root: ?[]const u8,
    specifiers: i32,
    limit: i32,
    timeout: u32,
) !DiscoverResult {
    if (scope != LOCAL) return errorResult(allocator, "scope not supported");
    if (specifiers < 0 or (specifiers & ~ALL) != 0) {
        return errorResult(allocator, "invalid specifiers: valid range is 0x00-0x3F");
    }
    const active_specifiers = if (specifiers == 0) ALL else specifiers;
    if (limit < 0) return .{ .found = try allocator.alloc(HolonRef, 0) };

    const normalized = normalizeExpression(expression);
    if (normalized) |needle| {
        if (isTransportUrl(needle)) {
            const found = try allocator.alloc(HolonRef, 1);
            found[0] = .{ .url = try allocator.dupe(u8, needle) };
            return .{ .found = found };
        }

        var root_cache: ?[]u8 = null;
        defer if (root_cache) |cached| allocator.free(cached);
        if (try discoverPathExpression(allocator, needle, root, &root_cache, timeout)) |path_result| {
            return path_result;
        }
    }

    const search_root = resolveDiscoverRoot(allocator, root) catch |err| {
        return errorResult(allocator, @errorName(err));
    };
    defer allocator.free(search_root);

    var entries = discoverEntries(allocator, search_root, active_specifiers, timeout) catch |err| {
        return errorResult(allocator, @errorName(err));
    };
    defer {
        for (entries.items) |*entry| entry.deinit(allocator);
        entries.deinit(allocator);
    }

    var refs: std.ArrayList(HolonRef) = .empty;
    errdefer {
        for (refs.items) |*ref| ref.deinit(allocator);
        refs.deinit(allocator);
    }

    for (entries.items) |*entry| {
        if (!matchesExpression(entry, normalized)) continue;
        try refs.append(allocator, try cloneRef(allocator, entry.holon_ref));
        if (limit > 0 and refs.items.len >= @as(usize, @intCast(limit))) break;
    }

    return .{ .found = try refs.toOwnedSlice(allocator) };
}

pub fn resolve(
    allocator: std.mem.Allocator,
    scope: i32,
    expression: []const u8,
    root: ?[]const u8,
    specifiers: i32,
    timeout: u32,
) !ResolveResult {
    var result = try discover(allocator, scope, expression, root, specifiers, 1, timeout);
    defer {
        if (result.found.len > 1) {
            for (result.found[1..]) |*ref| ref.deinit(allocator);
        }
        if (result.error_message) |message| allocator.free(message);
    }
    if (result.error_message) |message| {
        result.error_message = null;
        allocator.free(result.found);
        result.found = &.{};
        return .{ .error_message = message };
    }
    if (result.found.len == 0) {
        allocator.free(result.found);
        result.found = &.{};
        return .{ .error_message = try std.fmt.allocPrint(allocator, "holon \"{s}\" not found", .{expression}) };
    }
    const ref = result.found[0];
    allocator.free(result.found);
    result.found = &.{};
    return .{ .ref = ref };
}

pub fn resolveSourcePath(allocator: std.mem.Allocator, root: []const u8, slug: []const u8) ![]u8 {
    return std.fs.path.join(allocator, &.{ root, "examples", "hello-world", slug });
}

pub fn findBySlug(allocator: std.mem.Allocator, root: []const u8, slug: []const u8) !HolonRef {
    var result = try discover(allocator, LOCAL, slug, root, SOURCE | CWD | BUILT, 1, NO_TIMEOUT);
    defer {
        if (result.found.len > 1) {
            for (result.found[1..]) |*ref| ref.deinit(allocator);
        }
        if (result.error_message) |message| allocator.free(message);
    }
    if (result.error_message) |message| {
        defer allocator.free(message);
        allocator.free(result.found);
        result.found = &.{};
        return error.NotFound;
    }
    if (result.found.len == 0) {
        allocator.free(result.found);
        result.found = &.{};
        return error.NotFound;
    }
    const ref = result.found[0];
    allocator.free(result.found);
    result.found = &.{};
    return ref;
}

fn discoverEntries(allocator: std.mem.Allocator, root: []const u8, specifiers: i32, timeout: u32) !std.ArrayList(DiscoveredEntry) {
    var found: std.ArrayList(DiscoveredEntry) = .empty;
    errdefer {
        for (found.items) |*entry| entry.deinit(allocator);
        found.deinit(allocator);
    }

    const layers = [_]struct { flag: i32, name: []const u8 }{
        .{ .flag = SIBLINGS, .name = "siblings" },
        .{ .flag = CWD, .name = "cwd" },
        .{ .flag = SOURCE, .name = "source" },
        .{ .flag = BUILT, .name = "built" },
        .{ .flag = INSTALLED, .name = "installed" },
        .{ .flag = CACHED, .name = "cached" },
    };

    for (layers) |layer| {
        if ((specifiers & layer.flag) == 0) continue;
        var layer_entries: std.ArrayList(DiscoveredEntry) = .empty;
        switch (layer.flag) {
            SIBLINGS => if (try bundleHolonsRoot(allocator)) |bundle_root| {
                defer allocator.free(bundle_root);
                layer_entries = try discoverPackagesDirect(allocator, bundle_root, layer.name, timeout);
            },
            CWD => layer_entries = try discoverPackagesRecursive(allocator, root, layer.name, timeout),
            SOURCE => layer_entries = try discoverSourceEntries(allocator, root, timeout),
            BUILT => {
                const built_root = try std.fs.path.join(allocator, &.{ root, ".op", "build" });
                defer allocator.free(built_root);
                layer_entries = try discoverPackagesDirect(allocator, built_root, layer.name, timeout);
            },
            INSTALLED => {
                const bin_root = try opbin(allocator);
                defer allocator.free(bin_root);
                layer_entries = try discoverPackagesDirect(allocator, bin_root, layer.name, timeout);
            },
            CACHED => {
                const cached_root = try cacheDir(allocator);
                defer allocator.free(cached_root);
                layer_entries = try discoverPackagesRecursive(allocator, cached_root, layer.name, timeout);
            },
            else => {},
        }
        defer {
            for (layer_entries.items) |*entry| entry.deinit(allocator);
            layer_entries.deinit(allocator);
        }

        for (layer_entries.items) |*entry| {
            try appendDedupe(allocator, &found, try cloneEntry(allocator, entry.*));
        }
    }

    std.mem.sort(DiscoveredEntry, found.items, {}, compareEntries);
    return found;
}

fn discoverPackagesDirect(allocator: std.mem.Allocator, root: []const u8, origin: []const u8, timeout: u32) !std.ArrayList(DiscoveredEntry) {
    var dirs = try packageDirsDirect(allocator, root);
    defer freeStringArrayList(allocator, &dirs);
    return discoverPackagesFromDirs(allocator, root, origin, dirs.items, timeout);
}

fn discoverPackagesRecursive(allocator: std.mem.Allocator, root: []const u8, origin: []const u8, timeout: u32) !std.ArrayList(DiscoveredEntry) {
    var dirs = try packageDirsRecursive(allocator, root);
    defer freeStringArrayList(allocator, &dirs);
    return discoverPackagesFromDirs(allocator, root, origin, dirs.items, timeout);
}

fn discoverPackagesFromDirs(allocator: std.mem.Allocator, root: []const u8, origin: []const u8, dirs: []const []const u8, timeout: u32) !std.ArrayList(DiscoveredEntry) {
    _ = origin;
    _ = timeout;
    const abs_root = try absolutePath(allocator, root);
    defer allocator.free(abs_root);
    var entries: std.ArrayList(DiscoveredEntry) = .empty;
    errdefer {
        for (entries.items) |*entry| entry.deinit(allocator);
        entries.deinit(allocator);
    }
    for (dirs) |dir| {
        if (loadPackageEntry(allocator, abs_root, dir)) |entry| {
            try appendDedupe(allocator, &entries, entry);
        } else |_| {}
    }
    std.mem.sort(DiscoveredEntry, entries.items, {}, compareEntries);
    return entries;
}

fn loadPackageEntry(allocator: std.mem.Allocator, root: []const u8, dir: []const u8) !DiscoveredEntry {
    const json_path = try std.fs.path.join(allocator, &.{ dir, ".holon.json" });
    defer allocator.free(json_path);
    const data = try std.Io.Dir.cwd().readFileAlloc(std.Options.debug_io, json_path, allocator, .limited(16 * 1024 * 1024));
    defer allocator.free(data);

    const parsed = try std.json.parseFromSlice(std.json.Value, allocator, data, .{});
    defer parsed.deinit();
    const object = parsed.value.object;
    const schema = jsonString(object.get("schema")) orelse "";
    if (schema.len != 0 and !std.mem.eql(u8, schema, "holon-package/v1")) return error.UnsupportedPackageSchema;

    const identity_object = if (object.get("identity")) |value| switch (value) {
        .object => |obj| obj,
        else => null,
    } else null;

    var identity = IdentityInfo{
        .given_name = try allocator.dupe(u8, trim(jsonString(if (identity_object) |obj| obj.get("given_name") else null) orelse "")),
        .family_name = try allocator.dupe(u8, trim(jsonString(if (identity_object) |obj| obj.get("family_name") else null) orelse "")),
        .motto = try allocator.dupe(u8, trim(jsonString(if (identity_object) |obj| obj.get("motto") else null) orelse "")),
        .aliases = try jsonStringArray(allocator, if (identity_object) |obj| obj.get("aliases") else null),
    };
    errdefer identity.deinit(allocator);

    const raw_slug = trim(jsonString(object.get("slug")) orelse "");
    const slug = if (raw_slug.len == 0)
        try identity_mod.slugForAlloc(allocator, identity.given_name, identity.family_name)
    else
        try allocator.dupe(u8, raw_slug);
    errdefer allocator.free(slug);

    const abs_dir = try absolutePath(allocator, dir);
    errdefer allocator.free(abs_dir);
    const url = try fileUrl(allocator, abs_dir);
    errdefer allocator.free(url);
    const rel = try relativePath(allocator, root, abs_dir);
    errdefer allocator.free(rel);

    return .{
        .holon_ref = .{
            .url = url,
            .info = .{
                .slug = slug,
                .uuid = try allocator.dupe(u8, trim(jsonString(object.get("uuid")) orelse "")),
                .identity = identity,
                .lang = try allocator.dupe(u8, trim(jsonString(object.get("lang")) orelse "")),
                .runner = try allocator.dupe(u8, trim(jsonString(object.get("runner")) orelse "")),
                .status = try allocator.dupe(u8, trim(jsonString(object.get("status")) orelse "")),
                .kind = try allocator.dupe(u8, trim(jsonString(object.get("kind")) orelse "")),
                .transport = try allocator.dupe(u8, trim(jsonString(object.get("transport")) orelse "")),
                .entrypoint = try allocator.dupe(u8, trim(jsonString(object.get("entrypoint")) orelse "")),
                .architectures = try jsonStringArray(allocator, object.get("architectures")),
                .has_dist = jsonBool(object.get("has_dist")) orelse false,
                .has_source = jsonBool(object.get("has_source")) orelse false,
            },
        },
        .dir_path = abs_dir,
        .relative_path = rel,
    };
}

fn discoverSourceEntries(allocator: std.mem.Allocator, root: []const u8, timeout: u32) !std.ArrayList(DiscoveredEntry) {
    _ = timeout;
    const abs_root = try absolutePath(allocator, root);
    defer allocator.free(abs_root);
    var entries: std.ArrayList(DiscoveredEntry) = .empty;
    errdefer {
        for (entries.items) |*entry| entry.deinit(allocator);
        entries.deinit(allocator);
    }
    try walkSourceEntries(allocator, abs_root, abs_root, &entries);
    std.mem.sort(DiscoveredEntry, entries.items, {}, compareEntries);
    return entries;
}

fn walkSourceEntries(allocator: std.mem.Allocator, root: []const u8, current: []const u8, out: *std.ArrayList(DiscoveredEntry)) !void {
    var dir = std.Io.Dir.cwd().openDir(std.Options.debug_io, current, .{ .iterate = true }) catch return;
    defer dir.close(std.Options.debug_io);
    var it = dir.iterate();
    while (try it.next(std.Options.debug_io)) |entry| {
        const child = try std.fs.path.join(allocator, &.{ current, entry.name });
        errdefer allocator.free(child);
        switch (entry.kind) {
            .directory => {
                if (!shouldSkipDir(root, child, entry.name)) {
                    try walkSourceEntries(allocator, root, child, out);
                }
                allocator.free(child);
            },
            .file => {
                if (std.mem.eql(u8, entry.name, identity_mod.PROTO_MANIFEST_FILE_NAME)) {
                    if (sourceEntryFromProto(allocator, root, child)) |source_entry| {
                        try appendDedupe(allocator, out, source_entry);
                    } else |_| {}
                }
                allocator.free(child);
            },
            else => allocator.free(child),
        }
    }
}

fn sourceEntryFromProto(allocator: std.mem.Allocator, root: []const u8, proto_path: []const u8) !DiscoveredEntry {
    var manifest = try identity_mod.resolveProtoFile(allocator, proto_path);
    defer manifest.deinit(allocator);

    const holon_root = try sourceRootFromProto(allocator, proto_path);
    errdefer allocator.free(holon_root);
    const url = try fileUrl(allocator, holon_root);
    errdefer allocator.free(url);
    const rel = try relativePath(allocator, root, holon_root);
    errdefer allocator.free(rel);
    const slug = try manifest.identity.slugAlloc(allocator);
    errdefer allocator.free(slug);

    return .{
        .holon_ref = .{
            .url = url,
            .info = .{
                .slug = slug,
                .uuid = try allocator.dupe(u8, manifest.identity.uuid),
                .identity = .{
                    .given_name = try allocator.dupe(u8, manifest.identity.given_name),
                    .family_name = try allocator.dupe(u8, manifest.identity.family_name),
                    .motto = try allocator.dupe(u8, manifest.identity.motto),
                    .aliases = try dupeStringSlice(allocator, manifest.identity.aliases),
                },
                .lang = try allocator.dupe(u8, manifest.identity.lang),
                .runner = try allocator.dupe(u8, manifest.build_runner),
                .status = try allocator.dupe(u8, manifest.identity.status),
                .kind = try allocator.dupe(u8, manifest.kind),
                .transport = try allocator.dupe(u8, ""),
                .entrypoint = try allocator.dupe(u8, manifest.artifact_binary),
                .architectures = try allocator.alloc([]const u8, 0),
                .has_dist = false,
                .has_source = true,
            },
        },
        .dir_path = holon_root,
        .relative_path = rel,
    };
}

fn discoverPathExpression(
    allocator: std.mem.Allocator,
    expression: []const u8,
    root: ?[]const u8,
    root_cache: *?[]u8,
    timeout: u32,
) !?DiscoverResult {
    const candidate = try pathExpressionCandidate(allocator, expression, root, root_cache) orelse return null;
    defer allocator.free(candidate);
    const ref = try discoverRefAtPath(allocator, candidate, timeout) orelse {
        return .{ .found = try allocator.alloc(HolonRef, 0) };
    };
    const found = try allocator.alloc(HolonRef, 1);
    found[0] = ref;
    return .{ .found = found };
}

fn pathExpressionCandidate(allocator: std.mem.Allocator, expression: []const u8, root: ?[]const u8, root_cache: *?[]u8) !?[]u8 {
    const needle = trim(expression);
    if (needle.len == 0) return null;
    if (std.ascii.startsWithIgnoreCase(needle, "file://")) return try pathFromFileUrl(allocator, needle);
    if (std.mem.indexOf(u8, needle, "://") != null) return null;
    if (!(std.fs.path.isAbsolute(needle) or std.mem.startsWith(u8, needle, ".") or std.mem.indexOfScalar(u8, needle, '/') != null or std.mem.indexOfScalar(u8, needle, '\\') != null or std.mem.endsWith(u8, needle, ".holon"))) {
        return null;
    }
    if (std.fs.path.isAbsolute(needle)) return try allocator.dupe(u8, needle);
    if (root_cache.* == null) root_cache.* = try resolveDiscoverRoot(allocator, root);
    return try std.fs.path.join(allocator, &.{ root_cache.*.?, needle });
}

fn discoverRefAtPath(allocator: std.mem.Allocator, path: []const u8, timeout: u32) !?HolonRef {
    const abs_path = try absolutePath(allocator, path);
    defer allocator.free(abs_path);
    const stat = std.Io.Dir.cwd().statFile(std.Options.debug_io, abs_path, .{}) catch |err| switch (err) {
        error.FileNotFound => return null,
        else => return err,
    };
    if (stat.kind == .directory) {
        if (std.mem.endsWith(u8, std.fs.path.basename(abs_path), ".holon") or existsFileIn(abs_path, ".holon.json")) {
            const root = std.fs.path.dirname(abs_path) orelse abs_path;
            const entry = try loadPackageEntry(allocator, root, abs_path);
            return entry.holon_ref;
        }
        var entries = try discoverSourceEntries(allocator, abs_path, timeout);
        defer {
            for (entries.items) |*entry| entry.deinit(allocator);
            entries.deinit(allocator);
        }
        if (entries.items.len == 1) return try cloneRef(allocator, entries.items[0].holon_ref);
        for (entries.items) |entry| {
            if (std.mem.eql(u8, entry.dir_path, abs_path)) return try cloneRef(allocator, entry.holon_ref);
        }
        return null;
    }
    if (std.mem.eql(u8, std.fs.path.basename(abs_path), identity_mod.PROTO_MANIFEST_FILE_NAME)) {
        const root = std.fs.path.dirname(abs_path) orelse abs_path;
        const entry = try sourceEntryFromProto(allocator, root, abs_path);
        return entry.holon_ref;
    }
    return .{
        .url = try fileUrl(allocator, abs_path),
        .error_message = try allocator.dupe(u8, "binary describe probing is unavailable in the Zig discover source layer"),
    };
}

fn appendDedupe(allocator: std.mem.Allocator, entries: *std.ArrayList(DiscoveredEntry), new_entry: DiscoveredEntry) !void {
    const new_key = entryKey(new_entry);
    for (entries.items, 0..) |*existing, index| {
        const existing_key = entryKey(existing.*);
        if (std.mem.eql(u8, existing_key, new_key)) {
            if (pathDepth(new_entry.relative_path) < pathDepth(existing.relative_path)) {
                existing.deinit(allocator);
                entries.items[index] = new_entry;
            } else {
                var discard = new_entry;
                discard.deinit(allocator);
            }
            return;
        }
    }
    try entries.append(allocator, new_entry);
}

fn matchesExpression(entry: *const DiscoveredEntry, expression: ?[]const u8) bool {
    const needle = expression orelse return true;
    if (needle.len == 0) return false;
    if (entry.holon_ref.info) |info| {
        if (std.mem.eql(u8, info.slug, needle)) return true;
        if (info.uuid.len != 0 and std.mem.startsWith(u8, info.uuid, needle)) return true;
        for (info.identity.aliases) |alias| {
            if (std.mem.eql(u8, alias, needle)) return true;
        }
    }
    var base = std.fs.path.basename(entry.dir_path);
    if (std.mem.endsWith(u8, base, ".holon")) base = base[0 .. base.len - ".holon".len];
    return std.mem.eql(u8, base, needle);
}

fn packageDirsDirect(allocator: std.mem.Allocator, root: []const u8) !std.ArrayList([]u8) {
    var dirs: std.ArrayList([]u8) = .empty;
    errdefer freeStringArrayList(allocator, &dirs);
    var dir = std.Io.Dir.cwd().openDir(std.Options.debug_io, root, .{ .iterate = true }) catch return dirs;
    defer dir.close(std.Options.debug_io);
    var it = dir.iterate();
    while (try it.next(std.Options.debug_io)) |entry| {
        if (entry.kind != .directory or !std.mem.endsWith(u8, entry.name, ".holon")) continue;
        try dirs.append(allocator, try std.fs.path.join(allocator, &.{ root, entry.name }));
    }
    std.mem.sort([]u8, dirs.items, {}, stringLess);
    return dirs;
}

fn packageDirsRecursive(allocator: std.mem.Allocator, root: []const u8) !std.ArrayList([]u8) {
    var dirs: std.ArrayList([]u8) = .empty;
    errdefer freeStringArrayList(allocator, &dirs);
    try walkPackageDirs(allocator, root, root, &dirs);
    std.mem.sort([]u8, dirs.items, {}, stringLess);
    return dirs;
}

fn walkPackageDirs(allocator: std.mem.Allocator, root: []const u8, current: []const u8, out: *std.ArrayList([]u8)) !void {
    var dir = std.Io.Dir.cwd().openDir(std.Options.debug_io, current, .{ .iterate = true }) catch return;
    defer dir.close(std.Options.debug_io);
    var it = dir.iterate();
    while (try it.next(std.Options.debug_io)) |entry| {
        if (entry.kind != .directory) continue;
        const child = try std.fs.path.join(allocator, &.{ current, entry.name });
        errdefer allocator.free(child);
        if (std.mem.endsWith(u8, entry.name, ".holon")) {
            try out.append(allocator, child);
            continue;
        }
        if (shouldSkipDir(root, child, entry.name)) {
            allocator.free(child);
            continue;
        }
        try walkPackageDirs(allocator, root, child, out);
        allocator.free(child);
    }
}

fn shouldSkipDir(root: []const u8, path: []const u8, name: []const u8) bool {
    if (std.mem.eql(u8, root, path)) return false;
    if (std.mem.endsWith(u8, name, ".holon")) return false;
    if (std.mem.startsWith(u8, name, ".")) return true;
    return std.mem.eql(u8, name, ".git") or
        std.mem.eql(u8, name, ".op") or
        std.mem.eql(u8, name, "node_modules") or
        std.mem.eql(u8, name, "vendor") or
        std.mem.eql(u8, name, "build") or
        std.mem.eql(u8, name, "testdata");
}

fn resolveDiscoverRoot(allocator: std.mem.Allocator, root: ?[]const u8) ![]u8 {
    const raw = root orelse return std.fs.path.resolve(allocator, &.{"."});
    const cleaned = trim(raw);
    if (cleaned.len == 0) return error.EmptyRoot;
    const abs = try absolutePath(allocator, cleaned);
    const stat = try std.Io.Dir.cwd().statFile(std.Options.debug_io, abs, .{});
    if (stat.kind != .directory) {
        allocator.free(abs);
        return error.NotDir;
    }
    return abs;
}

fn bundleHolonsRoot(allocator: std.mem.Allocator) !?[]u8 {
    const executable = getEnvOwned(allocator, "ZIG_HOLONS_TEST_EXECUTABLE") catch |err| switch (err) {
        error.EnvironmentVariableNotFound => try std.process.executablePathAlloc(std.Options.debug_io, allocator),
        else => return err,
    };
    defer allocator.free(executable);
    var current = std.fs.path.dirname(executable) orelse return null;
    while (true) {
        if (std.mem.endsWith(u8, std.fs.path.basename(current), ".app")) {
            const candidate = try std.fs.path.join(allocator, &.{ current, "Contents", "Resources", "Holons" });
            if (isDir(candidate)) return candidate;
            allocator.free(candidate);
        }
        const parent = std.fs.path.dirname(current) orelse return null;
        if (std.mem.eql(u8, parent, current)) return null;
        current = parent;
    }
}

fn oppath(allocator: std.mem.Allocator) ![]u8 {
    if (getEnvOwned(allocator, "OPPATH")) |value| {
        if (trim(value).len != 0) return value;
        allocator.free(value);
    } else |_| {}
    if (getEnvOwned(allocator, "HOME")) |home| {
        defer allocator.free(home);
        return std.fs.path.join(allocator, &.{ home, ".op" });
    } else |_| {}
    return allocator.dupe(u8, ".op");
}

fn opbin(allocator: std.mem.Allocator) ![]u8 {
    if (getEnvOwned(allocator, "OPBIN")) |value| {
        if (trim(value).len != 0) return value;
        allocator.free(value);
    } else |_| {}
    const home = try oppath(allocator);
    defer allocator.free(home);
    return std.fs.path.join(allocator, &.{ home, "bin" });
}

fn cacheDir(allocator: std.mem.Allocator) ![]u8 {
    const home = try oppath(allocator);
    defer allocator.free(home);
    return std.fs.path.join(allocator, &.{ home, "cache" });
}

fn getEnvOwned(allocator: std.mem.Allocator, key: []const u8) ![]u8 {
    const key_z = try allocator.dupeZ(u8, key);
    defer allocator.free(key_z);
    const value = std.c.getenv(key_z.ptr) orelse return error.EnvironmentVariableNotFound;
    return allocator.dupe(u8, std.mem.span(value));
}

fn sourceRootFromProto(allocator: std.mem.Allocator, proto_path: []const u8) ![]u8 {
    const parent = std.fs.path.dirname(proto_path) orelse return allocator.dupe(u8, proto_path);
    const v1 = std.fs.path.basename(parent);
    const api = if (std.fs.path.dirname(parent)) |api_dir| std.fs.path.basename(api_dir) else "";
    if (std.mem.eql(u8, v1, "v1") and std.mem.eql(u8, api, "api")) {
        return allocator.dupe(u8, std.fs.path.dirname(std.fs.path.dirname(parent).?) orelse parent);
    }
    return allocator.dupe(u8, parent);
}

fn absolutePath(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    return std.fs.path.resolve(allocator, &.{path});
}

fn relativePath(allocator: std.mem.Allocator, root: []const u8, path: []const u8) ![]u8 {
    if (std.mem.startsWith(u8, path, root)) {
        var rel = path[root.len..];
        while (rel.len > 0 and (rel[0] == '/' or rel[0] == '\\')) {
            rel = rel[1..];
        }
        if (rel.len == 0) return allocator.dupe(u8, ".");
        return allocator.dupe(u8, rel);
    }
    return allocator.dupe(u8, path);
}

fn pathDepth(relative_path: []const u8) usize {
    const cleaned = std.mem.trim(u8, relative_path, "/ \t\r\n");
    if (cleaned.len == 0 or std.mem.eql(u8, cleaned, ".")) return 0;
    var count: usize = 1;
    for (cleaned) |ch| {
        if (ch == '/') count += 1;
    }
    return count;
}

fn fileUrl(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    return std.fmt.allocPrint(allocator, "file://{s}", .{path});
}

fn pathFromFileUrl(allocator: std.mem.Allocator, raw_url: []const u8) ![]u8 {
    if (!std.ascii.startsWithIgnoreCase(raw_url, "file://")) return error.InvalidFileUrl;
    return allocator.dupe(u8, raw_url["file://".len..]);
}

fn isTransportUrl(value: []const u8) bool {
    if (std.ascii.startsWithIgnoreCase(value, "file://")) return false;
    return std.mem.indexOf(u8, value, "://") != null;
}

fn normalizeExpression(expression: ?[]const u8) ?[]const u8 {
    const raw = expression orelse return null;
    return trim(raw);
}

fn trim(value: []const u8) []const u8 {
    return std.mem.trim(u8, value, " \t\r\n");
}

fn entryKey(entry: DiscoveredEntry) []const u8 {
    if (entry.holon_ref.info) |info| {
        const uuid = trim(info.uuid);
        if (uuid.len != 0) return uuid;
    }
    return entry.dir_path;
}

fn compareEntries(_: void, left: DiscoveredEntry, right: DiscoveredEntry) bool {
    const rel_order = std.mem.order(u8, left.relative_path, right.relative_path);
    if (rel_order != .eq) return rel_order == .lt;
    return std.mem.lessThan(u8, entrySortKey(left), entrySortKey(right));
}

fn entrySortKey(entry: DiscoveredEntry) []const u8 {
    if (entry.holon_ref.info) |info| {
        const uuid = trim(info.uuid);
        if (uuid.len != 0) return uuid;
    }
    return entry.holon_ref.url;
}

fn cloneEntry(allocator: std.mem.Allocator, entry: DiscoveredEntry) !DiscoveredEntry {
    return .{
        .holon_ref = try cloneRef(allocator, entry.holon_ref),
        .dir_path = try allocator.dupe(u8, entry.dir_path),
        .relative_path = try allocator.dupe(u8, entry.relative_path),
    };
}

fn cloneRef(allocator: std.mem.Allocator, ref: HolonRef) !HolonRef {
    return .{
        .url = try allocator.dupe(u8, ref.url),
        .info = if (ref.info) |info| try cloneInfo(allocator, info) else null,
        .error_message = if (ref.error_message) |message| try allocator.dupe(u8, message) else null,
    };
}

fn cloneInfo(allocator: std.mem.Allocator, info: HolonInfo) !HolonInfo {
    return .{
        .slug = try allocator.dupe(u8, info.slug),
        .uuid = try allocator.dupe(u8, info.uuid),
        .identity = .{
            .given_name = try allocator.dupe(u8, info.identity.given_name),
            .family_name = try allocator.dupe(u8, info.identity.family_name),
            .motto = try allocator.dupe(u8, info.identity.motto),
            .aliases = try dupeStringSlice(allocator, info.identity.aliases),
        },
        .lang = try allocator.dupe(u8, info.lang),
        .runner = try allocator.dupe(u8, info.runner),
        .status = try allocator.dupe(u8, info.status),
        .kind = try allocator.dupe(u8, info.kind),
        .transport = try allocator.dupe(u8, info.transport),
        .entrypoint = try allocator.dupe(u8, info.entrypoint),
        .architectures = try dupeStringSlice(allocator, info.architectures),
        .has_dist = info.has_dist,
        .has_source = info.has_source,
    };
}

fn errorResult(allocator: std.mem.Allocator, message: []const u8) !DiscoverResult {
    return .{
        .found = try allocator.alloc(HolonRef, 0),
        .error_message = try allocator.dupe(u8, message),
    };
}

fn jsonString(value: ?std.json.Value) ?[]const u8 {
    const unwrapped = value orelse return null;
    return switch (unwrapped) {
        .string => |str| str,
        else => null,
    };
}

fn jsonBool(value: ?std.json.Value) ?bool {
    const unwrapped = value orelse return null;
    return switch (unwrapped) {
        .bool => |b| b,
        else => null,
    };
}

fn jsonStringArray(allocator: std.mem.Allocator, value: ?std.json.Value) ![]const []const u8 {
    const unwrapped = value orelse return allocator.alloc([]const u8, 0);
    if (unwrapped != .array) return allocator.alloc([]const u8, 0);
    var out: std.ArrayList([]const u8) = .empty;
    errdefer freeStringArrayListConst(allocator, &out);
    for (unwrapped.array.items) |item| {
        if (item == .string) {
            const value_trimmed = trim(item.string);
            if (value_trimmed.len != 0) try out.append(allocator, try allocator.dupe(u8, value_trimmed));
        }
    }
    return out.toOwnedSlice(allocator);
}

fn dupeStringSlice(allocator: std.mem.Allocator, values: []const []const u8) ![]const []const u8 {
    var out = try allocator.alloc([]const u8, values.len);
    errdefer allocator.free(out);
    for (values, 0..) |value, index| {
        out[index] = try allocator.dupe(u8, value);
    }
    return out;
}

fn freeStringSlice(allocator: std.mem.Allocator, values: []const []const u8) void {
    for (values) |value| allocator.free(value);
    allocator.free(values);
}

fn freeStringArrayList(allocator: std.mem.Allocator, values: *std.ArrayList([]u8)) void {
    for (values.items) |value| allocator.free(value);
    values.deinit(allocator);
}

fn freeStringArrayListConst(allocator: std.mem.Allocator, values: *std.ArrayList([]const u8)) void {
    for (values.items) |value| allocator.free(value);
    values.deinit(allocator);
}

fn stringLess(_: void, a: []u8, b: []u8) bool {
    return std.mem.lessThan(u8, a, b);
}

fn isDir(path: []const u8) bool {
    const stat = std.Io.Dir.cwd().statFile(std.Options.debug_io, path, .{}) catch return false;
    return stat.kind == .directory;
}

fn existsFileIn(dir: []const u8, file: []const u8) bool {
    const path = std.fs.path.join(std.heap.page_allocator, &.{ dir, file }) catch return false;
    defer std.heap.page_allocator.free(path);
    const stat = std.Io.Dir.cwd().statFile(std.Options.debug_io, path, .{}) catch return false;
    return stat.kind == .file;
}

test "source path follows hello-world layout" {
    const path = try resolveSourcePath(std.testing.allocator, "/repo", "gabriel-greeting-zig");
    defer std.testing.allocator.free(path);
    try std.testing.expect(std.mem.endsWith(u8, path, "examples/hello-world/gabriel-greeting-zig"));
}
