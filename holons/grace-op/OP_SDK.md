# `op sdk` - SDK Prebuilt Management

`op sdk` manages native SDK prebuilts for the SDKs whose cold builds otherwise
compile native runtime dependencies from source:

- `c`
- `cpp`
- `ruby`
- `zig`

Other SDKs are intentionally out of scope for the v1 prebuilt pipeline. They
are pure-language at build time or already use upstream package managers for
their runtime/tooling dependencies.

## Commands

```text
op sdk install <lang> [--target <triplet>] [--version <v>] [--source <url-or-file>]
op sdk list [--installed | --available] [--lang <lang>]
op sdk verify <lang> [--target <triplet>] [--version <v>]
op sdk path <lang> [--target <triplet>] [--version <v>]
op sdk uninstall <lang> [--target <triplet>] [--version <v>]
```

`install` without `--target` uses the host triplet. `--source` installs a local
or explicit archive URL, which is useful for validating a PR artifact before it
is promoted to a GitHub Release.

`list --available` reads the SDK GitHub Release manifest. When a
`release-manifest.json` asset is present, it is the source of truth for archive
URLs and SHA-256 values.

## Storage

Installed prebuilts live under:

```text
$OPPATH/sdk/<lang>/<version>/<target>/
```

Each install writes a local `manifest.json` with the archive SHA-256 and tree
SHA-256. `op sdk verify <lang>` recomputes the installed tree hash and fails if
the tree no longer matches the recorded metadata.

## Targets

The v1 target set is T0 + T1:

- `aarch64-apple-darwin`
- `x86_64-apple-darwin`
- `x86_64-unknown-linux-gnu`
- `aarch64-unknown-linux-gnu`
- `x86_64-unknown-linux-musl`
- `aarch64-unknown-linux-musl`
- `x86_64-windows-gnu`

`x86_64-pc-windows-msvc` is deferred until the Windows self-hosted runner is
available.

## Build Integration

A holon that needs a native SDK prebuilt declares it in its manifest:

```protobuf
requires: {
  commands: ["zig"]
  sdk_prebuilts: ["zig"]
}
```

During `op build`, preflight locates every requested prebuilt for the host
triplet. On success it injects the corresponding environment variable into the
runner:

| SDK | Runner environment variable |
|---|---|
| `c` | `OP_SDK_C_PATH` |
| `cpp` | `OP_SDK_CPP_PATH` |
| `ruby` | `OP_SDK_RUBY_PATH` |
| `zig` | `OP_SDK_ZIG_PATH` |

On a miss, `op build` fails before invoking the runner and points at
`op sdk install <lang>`.

## Integrity

V0.x verifies SHA-256 before extraction. Cosign keyless signing is reserved for
v1.0+.
