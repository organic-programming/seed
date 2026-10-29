const std = @import("std");

const sdk_root = "../../../sdk/zig-holons";
const sdk_vendor_root = sdk_root ++ "/.zig-vendor/native";
const sdk_gen_root = sdk_root ++ "/gen/c";

const generated_c_sources = [_][]const u8{
    "google/protobuf/descriptor.pb-c.c",
    "google/protobuf/duration.pb-c.c",
    "google/protobuf/timestamp.pb-c.c",
    "holons/v1/describe.pb-c.c",
    "holons/v1/manifest.pb-c.c",
    "holons/v1/observability.pb-c.c",
    "holons/v1/session.pb-c.c",
    "v1/greeting.pb-c.c",
};

const grpc_unsecure_static_libs = [_][]const u8{
    "grpc_unsecure",
    "address_sorting",
    "upb_textformat_lib",
    "upb_reflection_lib",
    "upb_wire_lib",
    "upb_message_lib",
    "utf8_range_lib",
    "upb_mini_descriptor_lib",
    "upb_mini_table_lib",
    "upb_hash_lib",
    "upb_mem_lib",
    "upb_base_lib",
    "upb_lex_lib",
    "absl_statusor",
    "gpr",
    "absl_log_internal_check_op",
    "absl_flags_internal",
    "absl_flags_reflection",
    "absl_flags_private_handle_accessor",
    "absl_flags_commandlineflag",
    "absl_flags_commandlineflag_internal",
    "absl_flags_config",
    "absl_flags_program_name",
    "absl_raw_hash_set",
    "absl_hashtablez_sampler",
    "absl_flags_marshalling",
    "absl_log_internal_conditions",
    "absl_log_internal_message",
    "absl_examine_stack",
    "absl_log_internal_format",
    "absl_log_internal_nullguard",
    "absl_log_internal_structured_proto",
    "absl_log_internal_proto",
    "absl_log_internal_log_sink_set",
    "absl_log_internal_globals",
    "absl_log_sink",
    "absl_log_globals",
    "absl_hash",
    "absl_city",
    "absl_low_level_hash",
    "absl_vlog_config_internal",
    "absl_log_internal_fnmatch",
    "absl_random_distributions",
    "absl_random_seed_sequences",
    "absl_random_internal_entropy_pool",
    "absl_random_internal_randen",
    "absl_random_internal_randen_hwaes",
    "absl_random_internal_randen_hwaes_impl",
    "absl_random_internal_randen_slow",
    "absl_random_internal_platform",
    "absl_random_internal_seed_material",
    "absl_random_seed_gen_exception",
    "absl_status",
    "absl_cord",
    "absl_cordz_info",
    "absl_cord_internal",
    "absl_cordz_functions",
    "absl_exponential_biased",
    "absl_cordz_handle",
    "absl_crc_cord_state",
    "absl_crc32c",
    "absl_crc_internal",
    "absl_crc_cpu_detect",
    "absl_leak_check",
    "absl_strerror",
    "absl_str_format_internal",
    "absl_synchronization",
    "absl_graphcycles_internal",
    "absl_kernel_timeout_internal",
    "absl_stacktrace",
    "absl_symbolize",
    "absl_debugging_internal",
    "absl_demangle_internal",
    "absl_demangle_rust",
    "absl_decode_rust_punycode",
    "absl_utf8_for_code_point",
    "absl_malloc_internal",
    "absl_tracing_internal",
    "absl_time",
    "absl_civil_time",
    "absl_strings",
    "absl_strings_internal",
    "absl_string_view",
    "absl_int128",
    "absl_base",
    "absl_spinlock_wait",
    "absl_throw_delegate",
    "absl_raw_logging_internal",
    "absl_log_severity",
    "absl_time_zone",
    "cares",
};

const NativeRoot = struct {
    path: []const u8,
    from_env: bool,
};

fn selectedNativeRoot(b: *std.Build) NativeRoot {
    if (b.graph.environ_map.get("OP_SDK_ZIG_PATH")) |value| {
        const trimmed = std.mem.trim(u8, value, " \t\r\n");
        if (trimmed.len > 0) {
            return .{ .path = trimmed, .from_env = true };
        }
    }
    return .{ .path = sdk_vendor_root, .from_env = false };
}

fn lazyPath(b: *std.Build, path: []const u8) std.Build.LazyPath {
    if (std.fs.path.isAbsolute(path)) {
        return .{ .cwd_relative = path };
    }
    return b.path(path);
}

fn nativePath(b: *std.Build, root: NativeRoot, sub_path: []const u8) std.Build.LazyPath {
    return lazyPath(b, b.pathJoin(&.{ root.path, sub_path }));
}

fn checkNativeRoot(b: *std.Build, root: NativeRoot) *std.Build.Step.Run {
    const step = b.addSystemCommand(&.{
        "bash", "-lc",
        \\set -euo pipefail
        \\root="${ZIG_HOLONS_NATIVE_ROOT}"
        \\if [ -f "$root/include/grpc/grpc.h" ] && [ -f "$root/include/protobuf-c/protobuf-c.h" ] && [ -d "$root/lib" ]; then
        \\  exit 0
        \\fi
        \\echo "Zig SDK native prebuilt not found at $root" >&2
        \\echo "Run: op sdk install zig" >&2
        \\echo "SDK contributors can also run: cd sdk/zig-holons && zig build vendor" >&2
        \\exit 1
    });
    step.setEnvironmentVariable("ZIG_HOLONS_NATIVE_ROOT", root.path);
    return step;
}

fn holonsModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    native_root: NativeRoot,
) *std.Build.Module {
    const mod = b.addModule("zig_holons", .{
        .root_source_file = b.path(sdk_root ++ "/src/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    mod.addIncludePath(nativePath(b, native_root, "include"));
    mod.addIncludePath(b.path(sdk_gen_root));
    mod.addCSourceFiles(.{
        .root = b.path(sdk_gen_root),
        .files = &generated_c_sources,
        .flags = &.{ "-std=c99", "-Wno-unused-parameter" },
    });
    mod.link_libc = true;
    // macOS native archives are compiled with Apple Clang. Link them with
    // Apple's linker and libc++ after Zig emits its object file.
    if (target.result.os.tag == .macos) return mod;

    mod.addLibraryPath(nativePath(b, native_root, "lib"));
    mod.linkSystemLibrary("protobuf-c", .{ .use_pkg_config = .no, .preferred_link_mode = .static });
    for (grpc_unsecure_static_libs) |name| {
        mod.linkSystemLibrary(name, .{ .use_pkg_config = .no, .preferred_link_mode = .static });
    }
    if (target.result.os.tag == .windows) {
        mod.linkSystemLibrary("zlibstatic", .{ .use_pkg_config = .no, .preferred_link_mode = .static });
        mod.linkSystemLibrary("ws2_32", .{ .use_pkg_config = .no });
        mod.linkSystemLibrary("iphlpapi", .{ .use_pkg_config = .no });
        mod.linkSystemLibrary("dbghelp", .{ .use_pkg_config = .no });
        mod.linkSystemLibrary("bcrypt", .{ .use_pkg_config = .no });
    } else {
        mod.linkSystemLibrary("z", .{ .use_pkg_config = .no, .preferred_link_mode = .static });
        mod.linkSystemLibrary("resolv", .{ .use_pkg_config = .no });
    }
    mod.linkSystemLibrary("c++", .{ .use_pkg_config = .no });
    return mod;
}

fn linkMacOSObject(
    b: *std.Build,
    object: *std.Build.Step.Compile,
    target: std.Build.ResolvedTarget,
    native_root: NativeRoot,
    name: []const u8,
) std.Build.LazyPath {
    // Zig's bundled libc++ headers and Mach-O linker do not support all
    // newer Apple SDK headers and weak C++ relocations. Keep the native
    // compiler/linker pair consistent without changing the deployment target.
    object.bundle_compiler_rt = true;
    object.bundle_ubsan_rt = true;
    const arch = switch (target.result.cpu.arch) {
        .aarch64 => "arm64",
        .x86_64 => "x86_64",
        else => @panic("unsupported macOS architecture"),
    };
    const minimum = target.result.os.version_range.semver.min;
    const link = b.addSystemCommand(&.{ "xcrun", "--sdk", "macosx", "clang++", "-arch", arch });
    link.addArg(b.fmt("-mmacosx-version-min={d}.{d}.{d}", .{ minimum.major, minimum.minor, minimum.patch }));
    link.addFileArg(object.getEmittedBin());
    // Explicit archive paths prevent a stale dylib in the prebuilt prefix
    // from adding an unresolved @rpath dependency to the executable.
    link.addFileArg(nativePath(b, native_root, "lib/libprotobuf-c.a"));
    for (grpc_unsecure_static_libs) |lib| {
        link.addFileArg(nativePath(b, native_root, b.fmt("lib/lib{s}.a", .{lib})));
    }
    link.addFileArg(nativePath(b, native_root, "lib/libz.a"));
    link.addArgs(&.{ "-lresolv", "-framework", "CoreFoundation", "-o" });
    return link.addOutputFileArg(name);
}

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});
    const is_macos = target.result.os.tag == .macos;
    const native_root = selectedNativeRoot(b);
    const native_ready = checkNativeRoot(b, native_root);
    const holons_mod = holonsModule(b, target, optimize, native_root);
    const describe_mod = b.createModule(.{
        .root_source_file = b.path("gen/describe_generated.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "zig_holons", .module = holons_mod },
        },
    });

    const root_mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "zig_holons", .module = holons_mod },
            .{ .name = "describe_generated", .module = describe_mod },
        },
    });
    if (is_macos) {
        const object = b.addObject(.{
            .name = "gabriel-greeting-zig",
            .root_module = root_mod,
            .use_llvm = true,
        });
        object.step.dependOn(&native_ready.step);
        const binary = linkMacOSObject(b, object, target, native_root, "gabriel-greeting-zig");
        b.getInstallStep().dependOn(&b.addInstallBinFile(binary, "gabriel-greeting-zig").step);
    } else {
        const exe = b.addExecutable(.{
            .name = "gabriel-greeting-zig",
            .root_module = root_mod,
        });
        exe.step.dependOn(&native_ready.step);
        b.installArtifact(exe);
    }

    const tests = b.addTest(.{
        .emit_object = is_macos,
        .use_llvm = if (is_macos) true else null,
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/greetings.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = holons_mod },
            },
        }),
    });
    tests.step.dependOn(&native_ready.step);

    const test_step = b.step("test", "Run gabriel-greeting-zig tests");
    if (is_macos) {
        const binary = linkMacOSObject(b, tests, target, native_root, "gabriel-greeting-zig-tests");
        const run_tests = std.Build.Step.Run.create(b, "run gabriel-greeting-zig tests");
        run_tests.addFileArg(binary);
        run_tests.stdio = .inherit;
        test_step.dependOn(&run_tests.step);
    } else {
        const run_tests = b.addRunArtifact(tests);
        test_step.dependOn(&run_tests.step);
    }

    const clean_command = b.addSystemCommand(&.{ "rm", "-rf", ".zig-cache", "zig-out" });
    const clean_step = b.step("clean", "Remove Zig build outputs");
    clean_step.dependOn(&clean_command.step);
}
