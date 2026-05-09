#!/usr/bin/env python3
import json
import sys
from pathlib import Path


def parse_scalar(value):
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] == '"':
        return value[1:-1]
    if value in ("null", "~"):
        return None
    return value


def load_yaml_subset(path):
    root = {}
    stack = [(-1, root)]
    for raw in path.read_text().splitlines():
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        indent = len(raw) - len(raw.lstrip(" "))
        line = raw.strip()
        if ":" not in line:
            raise SystemExit(f"unsupported YAML line in {path}: {raw}")
        key, value = line.split(":", 1)
        key = key.strip()
        while stack and stack[-1][0] >= indent:
            stack.pop()
        parent = stack[-1][1]
        if value.strip() == "":
            node = {}
            parent[key] = node
            stack.append((indent, node))
        else:
            parent[key] = parse_scalar(value)
    return root


def seed_toolchain(repo_root):
    return load_yaml_subset(Path(repo_root) / "seed-toolchain.yaml")


def protoc_version(seed):
    protoc = seed.get("protoc", {})
    version = protoc.get("version") or protoc.get("upstream_tag", "")
    return str(version).removeprefix("v")


def sdk_requires_protoc(lang):
    return lang in {"csharp", "java", "js", "kotlin", "python", "ruby"}


def toolchain_manifest(seed, lang, target):
    entries = []
    if sdk_requires_protoc(lang):
        version = protoc_version(seed)
        sha = seed.get("protoc", {}).get("sha256_per_target", {}).get(target, "")
        entries.append({
            "name": "protoc",
            "version": version,
            "target": target,
            "sha256": sha,
        })
    for name, raw in sorted(seed.get("plugins", {}).get(lang, {}).items()):
        entry = {"name": name}
        if isinstance(raw, dict):
            if raw.get("version"):
                entry["version"] = raw["version"]
            per_target = raw.get("sha256_per_target")
            if isinstance(per_target, dict) and per_target.get(target):
                entry["target"] = target
                entry["sha256"] = per_target[target]
            elif raw.get("sha256"):
                entry["sha256"] = raw["sha256"]
        elif raw is not None:
            entry["version"] = str(raw)
        entries.append(entry)
    return entries


def plugin_version(seed, lang, name):
    raw = seed.get("plugins", {}).get(lang, {}).get(name)
    if isinstance(raw, dict):
        return raw.get("version", "")
    if raw is None:
        return ""
    return str(raw)


def main(argv):
    if len(argv) < 3:
        raise SystemExit("usage: seed_toolchain.py <command> <repo-root> [...]")
    command = argv[1]
    repo_root = argv[2]
    seed = seed_toolchain(repo_root)
    if command == "protoc-version":
        print(protoc_version(seed))
    elif command == "plugin-version":
        if len(argv) != 5:
            raise SystemExit("usage: seed_toolchain.py plugin-version <repo-root> <lang> <plugin>")
        print(plugin_version(seed, argv[3], argv[4]))
    elif command == "manifest-json":
        if len(argv) != 5:
            raise SystemExit("usage: seed_toolchain.py manifest-json <repo-root> <lang> <target>")
        print(json.dumps(toolchain_manifest(seed, argv[3], argv[4]), indent=4))
    else:
        raise SystemExit(f"unknown command: {command}")


if __name__ == "__main__":
    main(sys.argv)
