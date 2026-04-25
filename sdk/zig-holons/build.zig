const std = @import("std");

const vendor_root = ".zig-vendor/native";
const grpc_build_dir = ".zig-cache/cmake/grpc-native";
const protobuf_c_build_dir = ".zig-cache/cmake/protobuf-c-native";

fn sh(b: *std.Build, script: []const u8) *std.Build.Step.Run {
    return b.addSystemCommand(&.{ "bash", "-lc", script });
}

fn vendorGrpc(b: *std.Build) *std.Build.Step.Run {
    return sh(b,
        \\set -euo pipefail
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
    const step = sh(b,
        \\set -euo pipefail
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
    return mod;
}

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const grpc_step = vendorGrpc(b);
    const protobuf_c_step = vendorProtobufC(b, grpc_step);

    const vendor_step = b.step("vendor", "Build vendored gRPC Core and protobuf-c for the host target");
    vendor_step.dependOn(&protobuf_c_step.step);

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

    const clean_vendor = sh(b,
        \\set -euo pipefail
        \\rm -rf .zig-vendor .zig-cache/cmake zig-out
    );
    const clean_step = b.step("clean", "Remove Zig and vendored CMake build outputs");
    clean_step.dependOn(&clean_vendor.step);

    _ = grpc_build_dir;
    _ = protobuf_c_build_dir;
}
