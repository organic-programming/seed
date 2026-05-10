#!/usr/bin/env python3
from __future__ import annotations

import argparse
import fnmatch
import json
import sys
from pathlib import PurePosixPath


SDKS = [
    "c",
    "cpp",
    "csharp",
    "dart",
    "go",
    "java",
    "js",
    "js-web",
    "kotlin",
    "python",
    "ruby",
    "rust",
    "swift",
    "zig",
]

CPP_DOWNSTREAM = ["cpp", "c", "ruby", "python", "csharp", "kotlin", "java", "js"]
ZIG_GRPC_FOUNDATION = ["zig", "cpp", "c"]

SDK_DIR_TO_LANG = {f"sdk/{lang}-holons/": lang for lang in SDKS}
SCRIPT_TO_LANG = {
    ".github/scripts/build-prebuilt-c.sh": "c",
    ".github/scripts/build-prebuilt-cpp.sh": "cpp",
    ".github/scripts/build-prebuilt-csharp.sh": "csharp",
    ".github/scripts/build-prebuilt-dart.sh": "dart",
    ".github/scripts/build-prebuilt-go.sh": "go",
    ".github/scripts/build-prebuilt-java.sh": "java",
    ".github/scripts/build-prebuilt-js.sh": "js",
    ".github/scripts/build-prebuilt-js-web.sh": "js-web",
    ".github/scripts/build-prebuilt-kotlin.sh": "kotlin",
    ".github/scripts/build-prebuilt-python.sh": "python",
    ".github/scripts/build-prebuilt-ruby.sh": "ruby",
    ".github/scripts/build-prebuilt-rust.sh": "rust",
    ".github/scripts/build-prebuilt-swift.sh": "swift",
    ".github/scripts/build-prebuilt-zig.sh": "zig",
}

REPUBLISH_ALL_PATHS = {
    "seed-toolchain.yaml",
    ".github/scripts/lib-codegen-prebuilt.sh",
    ".github/scripts/seed_toolchain.py",
    ".github/scripts/build-prebuilt-codegen-light.sh",
    ".github/scripts/promote-sdk-prebuilts.sh",
    ".github/scripts/sdk_ci_paths.py",
    ".github/workflows/_sdk-prebuilt-target.yml",
    ".github/workflows/sdk-prebuilts.yml",
    ".github/workflows/sdk-source-pipeline.yml",
}

SDK_SOURCE_EXCLUDE_EXTENSIONS = {
    ".md",
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".svg",
    ".webp",
    ".ico",
}


def normalize(path: str) -> str:
    path = path.strip().replace("\\", "/")
    while path.startswith("./"):
        path = path[2:]
    return path


def ordered_union(*groups: list[str]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for group in groups:
        for item in group:
            if item not in seen:
                seen.add(item)
                out.append(item)
    return out


def is_sdk_doc_or_asset(path: str) -> bool:
    parts = PurePosixPath(path).parts
    name = parts[-1] if parts else ""
    suffix = PurePosixPath(path).suffix.lower()
    if suffix in SDK_SOURCE_EXCLUDE_EXTENSIONS:
        return True
    if name.startswith(("README", "LICENSE", "CHANGELOG")):
        return True
    return "docs" in parts


def is_sdk_source_path(path: str) -> bool:
    path = normalize(path)
    if not path:
        return False
    if path.startswith("sdk/"):
        return not is_sdk_doc_or_asset(path)
    if path in {"seed-toolchain.yaml", ".gitmodules"}:
        return True
    if fnmatch.fnmatch(path, ".github/scripts/build-prebuilt-*.sh"):
        return True
    if path in {
        ".github/scripts/lib-codegen-prebuilt.sh",
        ".github/scripts/seed_toolchain.py",
        ".github/scripts/sdk_ci_paths.py",
    }:
        return True
    if path.startswith("holons/grace-op/internal/sdkprebuilts/"):
        return not path.lower().endswith(".md")
    if path.startswith("holons/grace-op/cmd/protoc-gen-op-adapter/"):
        return not path.lower().endswith(".md")
    return False


def sdk_for_path(path: str) -> str | None:
    path = normalize(path)
    if path in SCRIPT_TO_LANG:
        return SCRIPT_TO_LANG[path]
    for prefix, lang in SDK_DIR_TO_LANG.items():
        if path.startswith(prefix):
            return lang
    return None


def publish_set(paths: list[str]) -> list[str]:
    normalized = [normalize(path) for path in paths if normalize(path)]
    if not normalized:
        return []

    for path in normalized:
        if path in REPUBLISH_ALL_PATHS:
            return SDKS.copy()
        if path.startswith("holons/grace-op/internal/sdkprebuilts/"):
            return SDKS.copy()
        if path.startswith("holons/grace-op/cmd/protoc-gen-op-adapter/"):
            return SDKS.copy()

    groups: list[list[str]] = []
    for path in normalized:
        if path.startswith("sdk/") and is_sdk_doc_or_asset(path):
            continue
        if path.startswith("sdk/cpp-holons/"):
            groups.append(CPP_DOWNSTREAM)
        elif path.startswith("sdk/zig-holons/third_party/grpc"):
            groups.append(ZIG_GRPC_FOUNDATION)
        else:
            lang = sdk_for_path(path)
            if lang:
                groups.append([lang])
    return ordered_union(*groups)


def read_files_arg(path: str | None) -> list[str]:
    if not path:
        return [line.strip() for line in sys.stdin if line.strip()]
    with open(path, encoding="utf-8") as handle:
        return [line.strip() for line in handle if line.strip()]


def cmd_classify(args: argparse.Namespace) -> int:
    files = read_files_arg(args.files)
    sdk_source = any(is_sdk_source_path(path) for path in files)
    print(f"sdk_source={'true' if sdk_source else 'false'}")
    print("sdk_source_json=" + json.dumps(sdk_source))
    return 0


def cmd_publish_set(args: argparse.Namespace) -> int:
    files = read_files_arg(args.files)
    print(json.dumps(publish_set(files)))
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)

    classify = sub.add_parser("classify")
    classify.add_argument("--files")
    classify.set_defaults(func=cmd_classify)

    publish = sub.add_parser("publish-set")
    publish.add_argument("--files")
    publish.set_defaults(func=cmd_publish_set)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
