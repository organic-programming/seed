const std = @import("std");

const default_vendor_root = ".zig-vendor/native";
const grpc_build_dir = ".zig-cache/cmake/grpc-native";
const protobuf_c_build_dir = ".zig-cache/cmake/protobuf-c-native";
const gen_root = "gen/c";

const generated_c_sources = [_][]const u8{
    "google/protobuf/descriptor.pb-c.c",
    "google/protobuf/timestamp.pb-c.c",
    "google/protobuf/duration.pb-c.c",
    "holons/v1/manifest.pb-c.c",
    "holons/v1/describe.pb-c.c",
    "holons/v1/coax.pb-c.c",
    "holons/v1/session.pb-c.c",
    "holons/v1/observability.pb-c.c",
    "holons/v1/instance.pb-c.c",
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

fn sh(b: *std.Build, script: []const u8) *std.Build.Step.Run {
    return b.addSystemCommand(&.{ "bash", "-lc", script });
}

fn selectedNativeRoot(b: *std.Build) NativeRoot {
    if (b.graph.environ_map.get("OP_SDK_ZIG_PATH")) |value| {
        const trimmed = std.mem.trim(u8, value, " \t\r\n");
        if (trimmed.len > 0) {
            return .{ .path = trimmed, .from_env = true };
        }
    }
    return .{ .path = default_vendor_root, .from_env = false };
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

fn nativeStringPath(b: *std.Build, root: NativeRoot, sub_path: []const u8) []const u8 {
    return b.pathJoin(&.{ root.path, sub_path });
}

fn checkNativeRoot(b: *std.Build, root: NativeRoot) *std.Build.Step.Run {
    const step = sh(b,
        \\set -euo pipefail
        \\root="${ZIG_HOLONS_NATIVE_ROOT}"
        \\if [ -f "$root/include/grpc/grpc.h" ] && [ -f "$root/include/protobuf-c/protobuf-c.h" ] && [ -d "$root/lib" ]; then
        \\  exit 0
        \\fi
        \\echo "Zig SDK native prebuilt not found at $root" >&2
        \\echo "Run: op sdk install zig" >&2
        \\echo "SDK contributors can also run: cd sdk/zig-holons && zig build vendor" >&2
        \\exit 1
    );
    step.setEnvironmentVariable("ZIG_HOLONS_NATIVE_ROOT", root.path);
    return step;
}

fn addStaticArchiveArgs(b: *std.Build, run: *std.Build.Step.Run, root: NativeRoot, names: []const []const u8) void {
    for (names) |name| {
        run.addArg(b.fmt("{s}/lib/lib{s}.a", .{ root.path, name }));
    }
}

fn lockedVendorSh(b: *std.Build, comptime script: []const u8) *std.Build.Step.Run {
    const prefix =
        \\set -euo pipefail
        \\mkdir -p .zig-cache
        \\lock_dir=".zig-cache/vendor.lock"
        \\while ! mkdir "$lock_dir" 2>/dev/null; do
        \\  sleep 1
        \\done
        \\cleanup_lock() {
        \\  rmdir "$lock_dir" 2>/dev/null || true
        \\}
        \\trap cleanup_lock EXIT
    ;
    return sh(b, prefix ++ "\n" ++ script);
}

fn vendorGrpc(b: *std.Build) *std.Build.Step.Run {
    return lockedVendorSh(b,
        \\set -euo pipefail
        \\if [ -f .zig-cache/cmake/grpc-native/CMakeCache.txt ]; then
        \\  cache_home="$(grep '^CMAKE_HOME_DIRECTORY:INTERNAL=' .zig-cache/cmake/grpc-native/CMakeCache.txt | cut -d= -f2- || true)"
        \\  if [ "$cache_home" != "$PWD/third_party/grpc" ]; then
        \\    rm -rf .zig-cache/cmake/grpc-native
        \\  fi
        \\fi
        \\cmake -S third_party/grpc -B .zig-cache/cmake/grpc-native -G Ninja \
        \\  -DCMAKE_BUILD_TYPE=Release \
        \\  -DCMAKE_INSTALL_PREFIX="$PWD/.zig-vendor/native" \
        \\  -DBUILD_SHARED_LIBS=OFF \
        \\  -DgRPC_INSTALL=ON \
        \\  -DgRPC_BUILD_TESTS=OFF \
        \\  -DgRPC_BUILD_CODEGEN=OFF \
        \\  -DgRPC_BUILD_GRPC_CPP_PLUGIN=OFF \
        \\  -DgRPC_BUILD_GRPC_CSHARP_PLUGIN=OFF \
        \\  -DgRPC_BUILD_GRPC_NODE_PLUGIN=OFF \
        \\  -DgRPC_BUILD_GRPC_OBJECTIVE_C_PLUGIN=OFF \
        \\  -DgRPC_BUILD_GRPC_PHP_PLUGIN=OFF \
        \\  -DgRPC_BUILD_GRPC_PYTHON_PLUGIN=OFF \
        \\  -DgRPC_BUILD_GRPC_RUBY_PLUGIN=OFF \
        \\  -DgRPC_ABSL_PROVIDER=module \
        \\  -DgRPC_CARES_PROVIDER=module \
        \\  -DgRPC_PROTOBUF_PROVIDER=module \
        \\  -DgRPC_RE2_PROVIDER=module \
        \\  -DgRPC_SSL_PROVIDER=module \
        \\  -DgRPC_ZLIB_PROVIDER=module
        \\cmake --build .zig-cache/cmake/grpc-native --target install --parallel "${ZIG_HOLONS_JOBS:-8}"
    );
}

fn vendorProtobufC(b: *std.Build, grpc_step: *std.Build.Step.Run) *std.Build.Step.Run {
    const step = lockedVendorSh(b,
        \\set -euo pipefail
        \\if [ -f .zig-cache/cmake/protobuf-c-native/CMakeCache.txt ]; then
        \\  cache_home="$(grep '^CMAKE_HOME_DIRECTORY:INTERNAL=' .zig-cache/cmake/protobuf-c-native/CMakeCache.txt | cut -d= -f2- || true)"
        \\  if [ "$cache_home" != "$PWD/third_party/protobuf-c/build-cmake" ]; then
        \\    rm -rf .zig-cache/cmake/protobuf-c-native
        \\  fi
        \\fi
        \\cmake -S third_party/protobuf-c/build-cmake -B .zig-cache/cmake/protobuf-c-native -G Ninja \
        \\  -DCMAKE_BUILD_TYPE=Release \
        \\  -DCMAKE_INSTALL_PREFIX="$PWD/.zig-vendor/native" \
        \\  -DCMAKE_PREFIX_PATH="$PWD/.zig-vendor/native" \
        \\  -DBUILD_SHARED_LIBS=OFF \
        \\  -DBUILD_PROTOC=ON \
        \\  -DBUILD_TESTS=OFF \
        \\  -DProtobuf_USE_STATIC_LIBS=ON
        \\cmake --build .zig-cache/cmake/protobuf-c-native --target install --parallel "${ZIG_HOLONS_JOBS:-8}"
    );
    step.step.dependOn(&grpc_step.step);
    return step;
}

fn configureModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    native_root: NativeRoot,
) *std.Build.Module {
    const mod = b.addModule("zig_holons", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    mod.addIncludePath(nativePath(b, native_root, "include"));
    mod.addIncludePath(b.path(gen_root));
    mod.addCSourceFiles(.{
        .root = b.path(gen_root),
        .files = &generated_c_sources,
        .flags = &.{ "-std=c99", "-Wno-unused-parameter", "-fno-sanitize=undefined" },
    });
    mod.addLibraryPath(nativePath(b, native_root, "lib"));
    mod.link_libc = true;
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
        if (target.result.os.tag != .macos) {
            mod.linkSystemLibrary("resolv", .{ .use_pkg_config = .no });
        }
    }
    mod.linkSystemLibrary("c++", .{ .use_pkg_config = .no });
    if (target.result.os.tag == .macos) {
        if (b.graph.environ_map.get("SDKROOT")) |sdk| {
            mod.addFrameworkPath(.{ .cwd_relative = b.fmt("{s}/System/Library/Frameworks", .{sdk}) });
        }
        mod.linkFramework("CoreFoundation", .{});
    }
    return mod;
}

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});
    const native_root = selectedNativeRoot(b);
    const native_ready = checkNativeRoot(b, native_root);

    const grpc_step = vendorGrpc(b);
    const protobuf_c_step = vendorProtobufC(b, grpc_step);

    const vendor_step = b.step("vendor", "Build vendored gRPC Core and protobuf-c for the host target");
    vendor_step.dependOn(&protobuf_c_step.step);

    const generate_protos = sh(b,
        \\set -euo pipefail
        \\rm -rf gen/c
        \\mkdir -p gen/c
        \\protoc="$ZIG_HOLONS_NATIVE_ROOT/bin/protoc-31.1.0"
        \\if [ ! -x "$protoc" ]; then
        \\  protoc="$ZIG_HOLONS_NATIVE_ROOT/bin/protoc"
        \\fi
        \\"$protoc" \
        \\  --plugin=protoc-gen-c="$ZIG_HOLONS_NATIVE_ROOT/bin/protoc-gen-c" \
        \\  -I ../../holons/grace-op/_protos \
        \\  -I ../../examples/_protos \
        \\  -I third_party/grpc/third_party/protobuf/src \
        \\  third_party/grpc/third_party/protobuf/src/google/protobuf/descriptor.proto \
        \\  third_party/grpc/third_party/protobuf/src/google/protobuf/timestamp.proto \
        \\  third_party/grpc/third_party/protobuf/src/google/protobuf/duration.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/manifest.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/describe.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/coax.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/session.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/observability.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/instance.proto \
        \\  ../../examples/_protos/v1/greeting.proto \
        \\  --c_out=gen/c
    );
    generate_protos.setEnvironmentVariable("ZIG_HOLONS_NATIVE_ROOT", native_root.path);
    generate_protos.step.dependOn(&native_ready.step);

    const proto_step = b.step("generate-protos", "Regenerate committed protobuf-c output");
    proto_step.dependOn(&generate_protos.step);

    const mod = configureModule(b, target, optimize, native_root);

    const lib = b.addLibrary(.{
        .name = "holons_zig",
        .linkage = .static,
        .root_module = mod,
    });
    lib.step.dependOn(&native_ready.step);
    b.installArtifact(lib);

    const header_gen = b.addExecutable(.{
        .name = "zig-holons-headergen",
        .root_module = b.createModule(.{
            .root_source_file = b.path("tools/emit_headers.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    header_gen.step.dependOn(&native_ready.step);
    const run_header_gen = b.addRunArtifact(header_gen);
    run_header_gen.addArg("include/holons_sdk.h");
    const headers_step = b.step("headers", "Emit the public C ABI header");
    headers_step.dependOn(&run_header_gen.step);

    const mod_tests = b.addTest(.{
        .root_module = mod,
    });
    mod_tests.step.dependOn(&native_ready.step);

    const run_mod_tests = b.addRunArtifact(mod_tests);
    const integration_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/skeleton_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    integration_tests.step.dependOn(&native_ready.step);

    const run_integration_tests = b.addRunArtifact(integration_tests);
    const test_step = b.step("test", "Run Zig SDK tests");
    test_step.dependOn(&run_mod_tests.step);
    test_step.dependOn(&run_integration_tests.step);

    const dial_tcp_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/dial_tcp_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    dial_tcp_tests.step.dependOn(&native_ready.step);
    const run_dial_tcp_tests = b.addRunArtifact(dial_tcp_tests);
    test_step.dependOn(&run_dial_tcp_tests.step);

    const dial_stdio_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/dial_stdio_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    dial_stdio_tests.step.dependOn(&native_ready.step);
    const run_dial_stdio_tests = b.addRunArtifact(dial_stdio_tests);
    test_step.dependOn(&run_dial_stdio_tests.step);

    const serve_tcp_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/serve_tcp_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    serve_tcp_tests.step.dependOn(&native_ready.step);
    const run_serve_tcp_tests = b.addRunArtifact(serve_tcp_tests);
    test_step.dependOn(&run_serve_tcp_tests.step);

    const serve_unix_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/serve_unix_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    serve_unix_tests.step.dependOn(&native_ready.step);
    const run_serve_unix_tests = b.addRunArtifact(serve_unix_tests);
    test_step.dependOn(&run_serve_unix_tests.step);

    const serve_fixture = b.addExecutable(.{
        .name = "zig-holons-serve-fixture",
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/support/serve_fixture_main.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    serve_fixture.step.dependOn(&native_ready.step);
    const install_serve_fixture = b.addInstallArtifact(serve_fixture, .{});

    const serve_stdio_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/serve_stdio_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    serve_stdio_tests.step.dependOn(&native_ready.step);
    const run_serve_stdio_tests = b.addRunArtifact(serve_stdio_tests);
    run_serve_stdio_tests.step.dependOn(&install_serve_fixture.step);
    run_serve_stdio_tests.setEnvironmentVariable(
        "ZIG_HOLONS_SERVE_FIXTURE",
        b.pathJoin(&.{ b.install_path, "bin", serve_fixture.out_filename }),
    );
    test_step.dependOn(&run_serve_stdio_tests.step);

    const transport_ws_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/transport_ws_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    transport_ws_tests.step.dependOn(&native_ready.step);
    const run_transport_ws_tests = b.addRunArtifact(transport_ws_tests);
    const test_ws_step = b.step("test-ws", "Run ws:// Holon-RPC dial tests");
    test_ws_step.dependOn(&run_transport_ws_tests.step);
    test_step.dependOn(&run_transport_ws_tests.step);

    const transport_wss_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/transport_wss_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    transport_wss_tests.step.dependOn(&native_ready.step);
    const run_transport_wss_tests = b.addRunArtifact(transport_wss_tests);
    const test_wss_step = b.step("test-wss", "Run wss:// Holon-RPC dial tests");
    test_wss_step.dependOn(&run_transport_wss_tests.step);
    test_step.dependOn(&run_transport_wss_tests.step);

    const transport_rest_sse_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/transport_rest_sse_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    transport_rest_sse_tests.step.dependOn(&native_ready.step);
    const run_transport_rest_sse_tests = b.addRunArtifact(transport_rest_sse_tests);
    const test_rest_sse_step = b.step("test-rest-sse", "Run rest+sse:// Holon-RPC dial tests");
    test_rest_sse_step.dependOn(&run_transport_rest_sse_tests.step);
    test_step.dependOn(&run_transport_rest_sse_tests.step);

    const hub_client_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/hub_client_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    hub_client_tests.step.dependOn(&native_ready.step);
    const run_hub_client_tests = b.addRunArtifact(hub_client_tests);
    const test_hub_client_step = b.step("test-hub-client", "Run hub-api client tests");
    test_hub_client_step.dependOn(&run_hub_client_tests.step);
    test_step.dependOn(&run_hub_client_tests.step);

    const discover_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/discover_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    discover_tests.step.dependOn(&native_ready.step);
    const run_discover_tests = b.addRunArtifact(discover_tests);
    const test_discover_step = b.step("test-discover", "Run discover tests");
    test_discover_step.dependOn(&run_discover_tests.step);
    test_step.dependOn(&run_discover_tests.step);

    const identity_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/identity_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    identity_tests.step.dependOn(&native_ready.step);
    const run_identity_tests = b.addRunArtifact(identity_tests);
    const test_identity_step = b.step("test-identity", "Run identity tests");
    test_identity_step.dependOn(&run_identity_tests.step);
    test_step.dependOn(&run_identity_tests.step);

    const observability_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("tests/observability_test.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "zig_holons", .module = mod },
            },
        }),
    });
    observability_tests.step.dependOn(&native_ready.step);
    const run_observability_tests = b.addRunArtifact(observability_tests);
    const test_observability_step = b.step("test-observability", "Run observability tests");
    test_observability_step.dependOn(&run_observability_tests.step);
    test_step.dependOn(&run_observability_tests.step);

    const prepare_c_abi = sh(b, "mkdir -p zig-out/bin");
    prepare_c_abi.step.dependOn(b.getInstallStep());

    const compile_c_abi = b.addSystemCommand(&.{"cc"});
    compile_c_abi.addArgs(&.{
        "tests/c_abi/main.c",
        "zig-out/lib/libholons_zig.a",
        nativeStringPath(b, native_root, "lib/libprotobuf-c.a"),
    });
    addStaticArchiveArgs(b, compile_c_abi, native_root, &grpc_unsecure_static_libs);
    if (target.result.os.tag == .windows) {
        compile_c_abi.addArg(nativeStringPath(b, native_root, "lib/libzlibstatic.a"));
    } else {
        compile_c_abi.addArg(nativeStringPath(b, native_root, "lib/libz.a"));
    }
    compile_c_abi.addArgs(&.{
        "-I",
        "include",
        "-I",
        nativeStringPath(b, native_root, "include"),
        "-I",
        "gen/c",
        "-lc++",
    });
    if (target.result.os.tag != .windows and target.result.os.tag != .macos) {
        compile_c_abi.addArg("-lresolv");
    }
    if (target.result.os.tag == .macos) {
        if (b.graph.environ_map.get("SDKROOT")) |sdk| {
            compile_c_abi.addArgs(&.{ "-F", b.fmt("{s}/System/Library/Frameworks", .{sdk}) });
        }
        compile_c_abi.addArgs(&.{ "-framework", "CoreFoundation" });
    }
    compile_c_abi.addArgs(&.{ "-o", "zig-out/bin/holons-c-abi-smoke" });
    compile_c_abi.step.dependOn(&prepare_c_abi.step);
    compile_c_abi.step.dependOn(&run_header_gen.step);

    const run_c_abi_smoke = sh(b,
        \\set -euo pipefail
        \\PORT="$(python3 - <<'PY'
        \\import socket
        \\sock = socket.socket()
        \\sock.bind(("127.0.0.1", 0))
        \\print(sock.getsockname()[1])
        \\sock.close()
        \\PY
        \\)"
        \\(
        \\  cd ../../examples/hello-world/gabriel-greeting-go
        \\  exec go run ./cmd serve --listen "tcp://127.0.0.1:${PORT}"
        \\) &
        \\PID="$!"
        \\cleanup() {
        \\  kill -TERM "$PID" 2>/dev/null || true
        \\  wait "$PID" 2>/dev/null || true
        \\}
        \\trap cleanup EXIT
        \\python3 - <<PY
        \\import socket, sys, time
        \\port = int("${PORT}")
        \\for _ in range(120):
        \\    sock = socket.socket()
        \\    sock.settimeout(0.2)
        \\    try:
        \\        sock.connect(("127.0.0.1", port))
        \\        sock.close()
        \\        sys.exit(0)
        \\    except OSError:
        \\        time.sleep(0.125)
        \\raise SystemExit("go greeting server did not start")
        \\PY
        \\zig-out/bin/holons-c-abi-smoke "tcp://127.0.0.1:${PORT}"
    );
    run_c_abi_smoke.step.dependOn(&compile_c_abi.step);
    const test_c_abi_step = b.step("test-c-abi", "Run the pure-C C ABI smoke test");
    test_c_abi_step.dependOn(&run_c_abi_smoke.step);
    test_step.dependOn(&run_c_abi_smoke.step);

    const clean_vendor = sh(b,
        \\set -euo pipefail
        \\rm -rf .zig-vendor .zig-cache/cmake zig-out
    );
    const clean_step = b.step("clean", "Remove Zig and vendored CMake build outputs");
    clean_step.dependOn(&clean_vendor.step);

    _ = grpc_build_dir;
    _ = protobuf_c_build_dir;
}
