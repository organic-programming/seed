# `op sdk` - SDK Prebuilt Management

`op sdk` manages SDK distributions. Some carry native runtime dependencies;
all official language SDKs may carry proto codegen plugins used by
`build.codegen`:

- `c`
- `cpp`
- `csharp`
- `dart`
- `go`
- `java`
- `js`
- `js-web`
- `kotlin`
- `python`
- `ruby`
- `rust`
- `swift`
- `zig`

## Commands

```text
op sdk install <lang> [--target <triplet>] [--version <v>] [--source <url-or-file>]
op sdk build   <lang> [--target <triplet>] [--version <v>] [--jobs <n>] [--force] [--no-install]
op sdk list           [--installed | --available | --compilable] [--lang <lang>]
op sdk verify  <lang> [--target <triplet>] [--version <v>]
op sdk path    <lang> [--target <triplet>] [--version <v>]
op sdk uninstall <lang> [--target <triplet>] [--version <v>]
```

`install` without `--source` and without `--target` uses the host triplet.
With `--source`, an archive `manifest.json` can declare the target/version;
when the corresponding flags are omitted, those manifest values choose the
install path. Explicit `--target` or `--version` values must match the archive
manifest. This is useful for validating a PR artifact before it is promoted to
a GitHub Release.

`build` is the source-build counterpart of `install`: it invokes the per-SDK
script under `.github/scripts/build-prebuilt-<lang>.sh` to compile the prebuilt
from the local gRPC + per-SDK sources, then installs the resulting tarball into
`$OPPATH/sdk` via the same code path as `install`. They are explicit
alternatives — no silent fallback. `install` is cheap (~30 s download); `build`
is expensive (~30-60 min cold, fast on cache hit).

| Use case | Verb |
|---|---|
| You want a published release for your host | `install` |
| The release isn't published for your (lang, target) yet | `build` |
| You're hacking on a SDK and want to validate locally | `build --force` |
| You just want the tarball, not installation | `build --no-install` |

`list --available` reads the SDK GitHub Release manifest. When a
`release-manifest.json` asset is present, it is the source of truth for archive
URLs and SHA-256 values.

`list --compilable` reports which SDKs `op sdk build <lang>` can build right
now on this checkout. When a SDK is not buildable, the response carries
`blockers` per entry naming the missing pieces — submodule markers, prereq
binaries, or the build script itself.

### Prerequisites for `op sdk build`

| SDK | Required commands on PATH | Required submodules |
|---|---|---|
| `c` | `zig`, `cmake`, `ninja`, `xcrun` (darwin only) | `sdk/zig-holons/third_party/grpc`, `sdk/zig-holons/third_party/protobuf-c`, `sdk/cpp-holons/third_party/nlohmann-json` |
| `cpp` | `zig`, `cmake`, `ninja`, `xcrun` (darwin) | `sdk/zig-holons/third_party/grpc`, `sdk/cpp-holons/third_party/nlohmann-json` |
| `go`, `js-web` | `go` | none |
| `csharp`, `java`, `js`, `kotlin`, `python` | `go`, `curl`, `unzip` | none |
| `dart` | `dart` | none |
| `ruby` | `ruby` (3.1.x), `bundle` | none |
| `rust` | `cargo` | none |
| `swift` | `git`, `swift` | none |
| `zig` | `zig`, `cmake`, `ninja`, `xcrun` (darwin only) | `sdk/zig-holons/third_party/grpc`, `sdk/zig-holons/third_party/protobuf-c` |

Initialise missing submodules with `git submodule update --init --recursive`.

## Storage

Installed prebuilts live under:

```text
$OPPATH/sdk/<lang>/<version>/<target>/
```

Each install writes a local `manifest.json` with the archive SHA-256 and tree
SHA-256. `op sdk verify <lang>` recomputes the installed tree hash and fails if
the tree no longer matches the recorded metadata.

Distributions that provide proto generators also advertise them in the same
local manifest:

```json
{
  "codegen": {
    "plugins": [
      {
        "name": "go",
        "binary": "bin/protoc-gen-go",
        "out_subdir": "go"
      }
    ]
  }
}
```

`op build` resolves `build.codegen.languages` through this block and runs the
plugin binary from the installed distribution, so proto generation does not
depend on generators being present on `PATH`.

## Targets

The v1 target set is T0 + T1:

- `aarch64-apple-darwin`
- `x86_64-apple-darwin` (with per-SDK exceptions — see [sdk/PREBUILTS.md](../../sdk/PREBUILTS.md#per-sdk-suspended-pairs))
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

During `op build`, `op test`, `op run`, and local `op inspect`, preflight
locates every requested prebuilt for the host triplet. Missing prebuilts are
auto-resolved by default: release-matching SDK source uses the `op sdk install`
path, while diverged local SDK source uses the `op sdk build` path and installs
the result. On success it injects the corresponding environment variable into
the runner:

| SDK | Runner environment variable |
|---|---|
| `c` | `OP_SDK_C_PATH` |
| `cpp` | `OP_SDK_CPP_PATH` |
| `csharp` | `OP_SDK_CSHARP_PATH` |
| `dart` | `OP_SDK_DART_PATH` |
| `go` | `OP_SDK_GO_PATH` |
| `java` | `OP_SDK_JAVA_PATH` |
| `js` | `OP_SDK_JS_PATH` |
| `js-web` | `OP_SDK_JS_WEB_PATH` |
| `kotlin` | `OP_SDK_KOTLIN_PATH` |
| `python` | `OP_SDK_PYTHON_PATH` |
| `ruby` | `OP_SDK_RUBY_PATH` |
| `rust` | `OP_SDK_RUST_PATH` |
| `swift` | `OP_SDK_SWIFT_PATH` |
| `zig` | `OP_SDK_ZIG_PATH` |

Use `--no-auto-install` to restore the strict behavior where a missing prebuilt
fails before invoking the runner and points at `op sdk install <lang>`.

## Integrity

V0.x verifies SHA-256 before extraction. Cosign keyless signing is reserved for
v1.0+.
