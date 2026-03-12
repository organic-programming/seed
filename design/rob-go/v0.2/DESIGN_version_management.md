# Rob-Go v0.2 — Version Management

Rob-Go v0.1 pins a single Go version in `holon.yaml`. If a
newer Go release ships, the operator must manually edit the
manifest and reprovision. v0.2 adds a first-class mechanism
to **replace** the active Go version at runtime.

---

## Problem

1. **Manual upgrade friction**: Changing the Go version requires
   editing `holon.yaml`, restarting Rob, and hoping the new
   version works. No rollback path.
2. **No visibility**: Callers cannot query which Go version Rob
   is currently running.
3. **Stale versions accumulate**: Old toolchain directories in
   `$OPPATH/toolchains/go/versions/` are never cleaned up.

## Solution

A new `UpdateToolchain` RPC and a `ToolchainInfo` RPC, plus
automatic pruning of the replaced version.

### Single Active Version

Rob-Go always has exactly **one** active Go version. Updating
replaces the previous one — this is not multi-version hosting.

### Update Flow

```
UpdateToolchain(target: "1.25.0" | "latest")
  → fetch https://go.dev/dl/?mode=json
  → if target == "latest": resolve to newest stable release
  → if target == current: return no-op
  → Ensure(target)           // download + verify + extract
  → update `current` symlink
  → prune old version directory
  → update holon.yaml delegates.toolchain.version
  → return {previous_version, current_version, changelog_url}
```

### Restart Semantics

The update takes effect **on next restart**. In-flight RPCs
continue using the version that was active when Rob started.
This avoids mid-request binary swaps.

### Version Query

```
ToolchainInfo()
  → return {version, goroot, goos, goarch, cgo_enabled}
```

### Pruning

After a successful update, the old version directory is removed:
```
rm -rf $OPPATH/toolchains/go/versions/go<old_version>/
```

`cache/` and `modcache/` are shared and preserved.

---

## Proto Changes

```protobuf
message UpdateToolchainRequest {
  string target_version = 1;  // semver or "latest"
}

message UpdateToolchainResponse {
  string previous_version = 1;
  string current_version = 2;
  string changelog_url = 3;
  bool   was_noop = 4;
}

message ToolchainInfoRequest {}

message ToolchainInfoResponse {
  string version = 1;
  string goroot = 2;
  string goos = 3;
  string goarch = 4;
  bool   cgo_enabled = 5;
}
```

---

## What Does Not Change

- **Hermetic environment** — same mechanism, just points at new GOROOT.
- **Exec / Library mode** — unchanged, driven by `current` symlink.
- **Manifest schema** — `delegates.toolchain.version` field already exists.
