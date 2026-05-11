#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "sdk_ci_paths.py"
SPEC = importlib.util.spec_from_file_location("sdk_ci_paths", SCRIPT)
assert SPEC and SPEC.loader
paths = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(paths)


class SDKCIPathsTests(unittest.TestCase):
    def test_sdk_source_excludes_sdk_docs_and_assets(self) -> None:
        self.assertFalse(paths.is_sdk_source_path("sdk/cpp-holons/README.md"))
        self.assertFalse(paths.is_sdk_source_path("sdk/cpp-holons/docs/build.md"))
        self.assertFalse(paths.is_sdk_source_path("sdk/cpp-holons/license"))
        self.assertFalse(paths.is_sdk_source_path("sdk/cpp-holons/logo.svg"))
        self.assertTrue(paths.is_sdk_source_path("sdk/cpp-holons/src/runtime.cpp"))

    def test_sdk_source_includes_foundation_files(self) -> None:
        self.assertTrue(paths.is_sdk_source_path("seed-toolchain.yaml"))
        self.assertTrue(paths.is_sdk_source_path(".gitmodules"))
        self.assertTrue(paths.is_sdk_source_path(".github/scripts/build-prebuilt-cpp.sh"))
        self.assertTrue(paths.is_sdk_source_path("holons/grace-op/internal/sdkprebuilts/build.go"))
        self.assertTrue(paths.is_sdk_source_path("holons/grace-op/cmd/protoc-gen-op-noop/main.go"))
        self.assertFalse(paths.is_sdk_source_path("holons/grace-op/internal/sdkprebuilts/README.md"))

    def test_publish_set_republishes_all_for_central_pin_and_tooling(self) -> None:
        self.assertEqual(paths.publish_set(["seed-toolchain.yaml"]), paths.SDKS)
        self.assertEqual(paths.publish_set([".github/scripts/lib-codegen-prebuilt.sh"]), paths.SDKS)
        self.assertEqual(paths.publish_set([".github/scripts/seed_release_bump.py"]), paths.SDKS)
        self.assertEqual(paths.publish_set([".github/scripts/seed_toolchain.py"]), paths.SDKS)
        self.assertEqual(paths.publish_set(["holons/grace-op/cmd/protoc-gen-op-noop/main.go"]), paths.SDKS)

    def test_publish_set_republishes_cpp_and_downstream(self) -> None:
        self.assertEqual(paths.publish_set(["sdk/cpp-holons/src/runtime.cpp"]), paths.CPP_DOWNSTREAM)

    def test_publish_set_republishes_zig_grpc_foundation(self) -> None:
        self.assertEqual(
            paths.publish_set(["sdk/zig-holons/third_party/grpc/CMakeLists.txt"]),
            paths.ZIG_GRPC_DOWNSTREAM,
        )

    def test_publish_set_maps_per_sdk_paths(self) -> None:
        self.assertEqual(paths.publish_set(["sdk/python-holons/holons/__init__.py"]), ["python"])
        self.assertEqual(paths.publish_set([".github/scripts/build-prebuilt-js-web.sh"]), ["js-web"])
        self.assertEqual(paths.publish_set(["sdk/cpp-holons/README.md"]), [])

    def test_publish_set_preserves_order_and_deduplicates(self) -> None:
        self.assertEqual(
            paths.publish_set(["sdk/python-holons/a.py", "sdk/cpp-holons/b.cpp", "sdk/python-holons/c.py"]),
            ["python", "cpp", "c", "ruby", "csharp", "kotlin", "java", "js"],
        )


if __name__ == "__main__":
    unittest.main()
