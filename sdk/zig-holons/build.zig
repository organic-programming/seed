const std = @import("std");

const vendor_root = ".zig-vendor/native";
const grpc_build_dir = ".zig-cache/cmake/grpc-native";
const protobuf_c_build_dir = ".zig-cache/cmake/protobuf-c-native";
const gen_root = "gen/c";

const generated_c_sources = [_][]const u8{
    "google/protobuf/descriptor.pb-c.c",
    "holons/v1/manifest.pb-c.c",
    "holons/v1/describe.pb-c.c",
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
    "z",
};

fn sh(b: *std.Build, script: []const u8) *std.Build.Step.Run {
    return b.addSystemCommand(&.{ "bash", "-lc", script });
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

fn configureModule(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) *std.Build.Module {
    const mod = b.addModule("zig_holons", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    mod.addIncludePath(b.path(vendor_root ++ "/include"));
    mod.addIncludePath(b.path(gen_root));
    mod.addCSourceFiles(.{
        .root = b.path(gen_root),
        .files = &generated_c_sources,
        .flags = &.{ "-std=c99", "-Wno-unused-parameter" },
    });
    mod.addLibraryPath(b.path(vendor_root ++ "/lib"));
    mod.link_libc = true;
    mod.linkSystemLibrary("protobuf-c", .{ .use_pkg_config = .no, .preferred_link_mode = .static });
    for (grpc_unsecure_static_libs) |name| {
        mod.linkSystemLibrary(name, .{ .use_pkg_config = .no, .preferred_link_mode = .static });
    }
    mod.linkSystemLibrary("resolv", .{ .use_pkg_config = .no });
    mod.linkSystemLibrary("c++", .{ .use_pkg_config = .no });
    if (target.result.os.tag == .macos) {
        mod.linkFramework("CoreFoundation", .{});
    }
    return mod;
}

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const grpc_step = vendorGrpc(b);
    const protobuf_c_step = vendorProtobufC(b, grpc_step);

    const vendor_step = b.step("vendor", "Build vendored gRPC Core and protobuf-c for the host target");
    vendor_step.dependOn(&protobuf_c_step.step);

    const generate_protos = sh(b,
        \\set -euo pipefail
        \\rm -rf gen/c
        \\mkdir -p gen/c
        \\"$PWD/.zig-vendor/native/bin/protoc-31.1.0" \
        \\  --plugin=protoc-gen-c="$PWD/.zig-vendor/native/bin/protoc-gen-c" \
        \\  -I ../../holons/grace-op/_protos \
        \\  -I ../../examples/_protos \
        \\  -I third_party/grpc/third_party/protobuf/src \
        \\  third_party/grpc/third_party/protobuf/src/google/protobuf/descriptor.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/manifest.proto \
        \\  ../../holons/grace-op/_protos/holons/v1/describe.proto \
        \\  ../../examples/_protos/v1/greeting.proto \
        \\  --c_out=gen/c
    );
    generate_protos.step.dependOn(vendor_step);

    const proto_step = b.step("generate-protos", "Regenerate committed protobuf-c output");
    proto_step.dependOn(&generate_protos.step);

    const mod = configureModule(b, target, optimize);

    const lib = b.addLibrary(.{
        .name = "holons_zig",
        .linkage = .static,
        .root_module = mod,
    });
    lib.step.dependOn(vendor_step);
    b.installArtifact(lib);

    const mod_tests = b.addTest(.{
        .root_module = mod,
    });
    mod_tests.step.dependOn(vendor_step);

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
    integration_tests.step.dependOn(vendor_step);

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
    dial_tcp_tests.step.dependOn(vendor_step);
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
    dial_stdio_tests.step.dependOn(vendor_step);
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
    serve_tcp_tests.step.dependOn(vendor_step);
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
    serve_unix_tests.step.dependOn(vendor_step);
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
    serve_fixture.step.dependOn(vendor_step);
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
    serve_stdio_tests.step.dependOn(vendor_step);
    const run_serve_stdio_tests = b.addRunArtifact(serve_stdio_tests);
    run_serve_stdio_tests.step.dependOn(&install_serve_fixture.step);
    run_serve_stdio_tests.setEnvironmentVariable(
        "ZIG_HOLONS_SERVE_FIXTURE",
        b.pathJoin(&.{ b.install_path, "bin", serve_fixture.out_filename }),
    );
    test_step.dependOn(&run_serve_stdio_tests.step);

    const clean_vendor = sh(b,
        \\set -euo pipefail
        \\rm -rf .zig-vendor .zig-cache/cmake zig-out
    );
    const clean_step = b.step("clean", "Remove Zig and vendored CMake build outputs");
    clean_step.dependOn(&clean_vendor.step);

    _ = grpc_build_dir;
    _ = protobuf_c_build_dir;
}
