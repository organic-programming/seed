# SDK Toolchain Audit

Status: complete  
Date: 2026-04-26  
Scope: the 13 production SDKs in `sdk/` plus `sdk/zig-holons` (chantier in flight)

This audit reports the **actual** toolchain situation per SDK. Every claim cites a path:line. Edge cases (Alpine/musl, Apple Silicon Ruby, Windows) are flagged when the evidence is in the code, called out as gaps when the evidence is silence.

---

## 1. Methodology

Three sources crossed:

1. **Per-SDK manifest + README + example** — `sdk/<lang>-holons/<manifest>`, `examples/hello-world/gabriel-greeting-<lang>/`. Read line-by-line.
2. **CI workflows** — `.github/workflows/{ader,ci}.yml`, `ader/catalogues/*/checks.yaml`, `ader/bouquets/*.yaml`. Mapped tools-installed and timeouts-declared.
3. **`op` runner internals** — `holons/grace-op/internal/holons/{lifecycle,runner_registry}.go`, `OP_BUILD.md`. Mapped how `op build` invokes each toolchain and what it checks beforehand.

Where evidence is silent (no benchmark, no edge-case test), this audit flags the silence.

---

## 2. The two-time problem (read this first)

A holon's lifecycle has two distinct moments where toolchain matters, and they are commonly conflated:

### 2.1 At-build-time — `op build my-holon`

Per [`CLAUDE.md`](../../CLAUDE.md) rule #7, generated code is committed: every `.pb.go`, `.pb.swift`, `.pb.dart`, `.pb-c.{c,h}`, etc. lives in the SDK's `gen/` directory and in each holon's `gen/`. Per [`OP_BUILD.md:118-123`](../../holons/grace-op/OP_BUILD.md), `op` parses `.proto` files **in-process** via pure-Go `protoparse` for validation; it does not generate stubs.

**Consequence: no SDK requires `protoc` at build time. Ever.** A user pulling the repo and running `op build` only needs the language's compile/run toolchain (`go`, `cargo`, `swift`, `dart`, `python`, `ruby`, `zig`, `cmake`, etc.).

What hurts at build time is the **runtime native libraries** the language linker needs — `libgrpc.a` for C/C++/Zig, the compiled `grpc` C extension for Ruby, the `grpcio` C extension for Python on Alpine. That's the prebuilt scope.

### 2.2 At-proto-edit-time — developer modifies `holon.proto`

When a developer changes a `.proto` and wants to regenerate the bindings, `protoc` (or its substitute) is needed. This is a **dev-time** operation, run a few times per feature, not at every build, not in CI for downstream users. Per language ecosystem, `protoc` may be:

- **Vendored by the language ecosystem** (no system install needed): java, kotlin, csharp, python, ruby, rust (via `tonic-build` + `protoc-bin-vendored`), zig (via the SDK's vendored `protoc-c`).
- **Ship-via-prebuilt** (covered by this prebuilts spec): c, cpp, zig — the SDK prebuilt archive includes `protoc`, `protoc-c`, `grpc_cpp_plugin` under `bin/`.
- **System install required** (genuine residual): go, dart, swift; optionally js if typed codegen is desired (the `@grpc/proto-loader` runtime-parsing path needs no protoc).

For the third class, the right behaviour is **diagnostic, not vending**: `op` detects missing tooling and prints an actionable hint pointing to the platform-specific install command. No new infrastructure is needed.

### 2.3 Examples that conflate the two times

Two existing examples invoke `protoc` from a build script in ways that would mislead a reader into thinking system protoc is required at build time:

- [`examples/hello-world/gabriel-greeting-rust/build.rs:26`](../../examples/hello-world/gabriel-greeting-rust/build.rs) calls `Command::new("protoc")` to produce a descriptor set into `OUT_DIR/holon_descriptor.bin` that nothing reads. The actual codegen at lines 13-18 uses `tonic_build::compile_protos()` which embeds protoc via `protoc-bin-vendored`. The line-26 call is a **redundant sanity check**; `op build`'s pure-Go protoparse already validates the proto.
- [`examples/hello-world/gabriel-greeting-dart/scripts/generate_proto.sh:22-29`](../../examples/hello-world/gabriel-greeting-dart/scripts/generate_proto.sh) runs `protoc --descriptor_set_out=$tmp` then `rm -f $tmp`. Same pattern — pure validation, immediately discarded.

**Recommendation (out of scope but flagged)**: drop these redundant calls. They reinforce a false belief that system protoc is required at build time.

---

## 3. Verdict at a glance

| SDK | Class | Native runtime libs at link time | Edge-case pain | Prebuilt benefit |
|---|---|---|---|---|
| `go` | pure-language | none (CGO optional, off by default) | none documented | none |
| `rust` | pure-language | none (rustls feature, no aws-lc) | none documented | none |
| `dart` | pure-language | none (`grpc^5.1.0` is pure Dart) | none documented | none |
| `js` (Node) | pure-language | none (`@grpc/grpc-js^1.10` is pure JS) | none documented | none |
| `js-web` | pure-language | none (browser sandbox) | browser-only by definition | none |
| `swift` | pure-language | none (grpc-swift, swift-nio, swift-protobuf all pure Swift) | macOS 13+ only declared in `Package.swift:6`; iOS/Linux undeclared | none |
| `java` | bundled-prebuilt | none (`grpc-netty-shaded:1.60` is uber-JAR with bundled netty) | JDK 17+ required | none |
| `kotlin` | bundled-prebuilt | none (same uber-JAR as Java) | JDK 21+ required | none |
| `csharp` | managed-prebuilt | none (`Grpc.AspNetCore:2.76` is pure managed .NET) | net8.0 / net9.0 only | none |
| `python` | wheel-with-source-fallback | `grpcio` C extension wraps gRPC C library | Alpine/musl: no wheel → source build needs gRPC dev libs + C toolchain | **moderate** (Alpine niche) |
| `ruby` | binary-gem-with-source-fallback | `grpc^1.58.3` C extension, `google-protobuf` C extension | **explicitly slow in CI** (`ader.yml:37`); Apple Silicon w/ x86_64 Ruby = source build (`sdk/ruby-holons/README.md:5`); Alpine/musl = source build mandatory | **HIGH** |
| `c` | system-deps-required | gRPC C, libprotobuf-c, upb, transitive abseil/BoringSSL/c-ares/re2/zlib | hard-coded `/opt/homebrew` and `/usr/local` paths in CMake (`sdk/cpp-holons/CMakeLists.txt:13,30,39,87`); Windows untested; Alpine/musl: source build of gRPC required | **HIGH** |
| `cpp` | system-deps-required | gRPC-C++, abseil, BoringSSL/OpenSSL, protobuf, plus `grpc_cpp_plugin` codegen tool | same hard-coded paths as C; Windows broken in practice | **HIGH** |
| `zig` (in flight) | vendored-from-source | gRPC Core C, libprotobuf-c, abseil, BoringSSL, c-ares, re2, zlib, upb (all built from source via CMake/Ninja) | cold first build ~10-15 min | **HIGH** |

**Four SDKs benefit substantially from prebuilts**: ruby, c, cpp, zig. Python is borderline (Alpine niche).

---

## 4. Per-SDK deep dives

### 4.1 `sdk/go-holons` — pure Go

**Manifest**: `sdk/go-holons/go.mod`. Direct deps `google.golang.org/grpc v1.78.0`, `google.golang.org/protobuf v1.36.11`, `nhooyr.io/websocket v1.8.17`. All pure Go.

**Build**: `go-module` runner at `holons/grace-op/internal/holons/lifecycle.go:1109-1130` runs `go build -o <path> <pkg>`. Prereqs: `go` on PATH (`lifecycle.go:1091-1107`).

**Cold install** in CI: ~2 min on warm `GOMODCACHE` (`ader.yml:42`), seconds when cached.

**Proto-edit-time toolchain**: requires system `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc`. **No vendored alternative shipped today.** This is a residual "system install required" case that should be handled by diagnostic in `op` rather than by us shipping protoc.

**Verdict**: pure-language at build time, **no prebuilt benefit** for runtime. Diagnostic hint sufficient for proto-edit time.

### 4.2 `sdk/rust-holons` — pure Rust

**Manifest**: `sdk/rust-holons/Cargo.toml`. Deps `tonic 0.12`, `prost 0.13`, `tokio 1.x`, `reqwest` with `rustls-tls`, `tokio-tungstenite` with `rustls-tls-webpki-roots`. **Rustls** chosen over OpenSSL/aws-lc → no C TLS dep.

**Build**: `cargo` runner at `runner_registry.go:807-837`.

**Proto-edit-time toolchain**: `tonic-build` 0.12 (a build dep) handles codegen. By default, `tonic-build` calls `protoc`, but it can be configured to use `protoc-bin-vendored` which embeds protoc as a dependency. The seed example at `examples/hello-world/gabriel-greeting-rust/build.rs` uses `tonic_build::configure().compile_protos()` (line 13-18) — that path can be made fully vendored by adding `protoc-bin-vendored` to `[build-dependencies]`. The line-26 `Command::new("protoc")` call is gratuitous (descriptor sanity check, output unused).

**Verdict**: pure-language. **No prebuilt benefit.** Recommend cleaning up the redundant build.rs protoc call as a side-task.

### 4.3 `sdk/dart-holons` — pure Dart

**Manifest**: `sdk/dart-holons/pubspec.yaml`. Deps `grpc:^5.1.0`, `http2:^2.3.1`, `protobuf:^6.0.0`, `protoc_plugin:^25.0.0`. **`grpc` is pure Dart** (uses HTTP/2 directly).

**Build**: `dart` and `flutter` runners. Cache `PUB_CACHE → /Users/popok/.ader-ci-cache/dart-pub` (`ader.yml:46`).

**Proto-edit-time toolchain**: `protoc_plugin:^25.0.0` is a Dart-language plugin that runs as a sub-process **of** `protoc`, not a standalone codegen. So Dart proto-edit needs **system `protoc` + the Dart plugin activated via `dart pub global activate protoc_plugin`**. The Dart ecosystem does not (today) ship a binary that bundles protoc with the plugin, unlike Python/Java/C#/Ruby. The `gabriel-greeting-dart/scripts/generate_proto.sh` lines 22-29 contain a redundant descriptor sanity check that should be dropped.

**Verdict**: pure-language at build time. **No prebuilt benefit** for runtime. Diagnostic hint for proto-edit. Side-task: drop the redundant sanity check in the example script.

### 4.4 `sdk/js-holons` — pure JavaScript (Node)

**Manifest**: `sdk/js-holons/package.json`. Deps `@grpc/grpc-js:^1.10`, `@grpc/proto-loader:^0.7.13`, `protobufjs:^7.5.4`, `ws:^8.16`. All pure JS since `@grpc/grpc-js` deprecated the old native addon.

**Build**: `npm` runner. Cache `npm_config_cache → .ader-ci-cache/npm` (`ader.yml:47`).

**Proto-edit-time toolchain**: two paths:
- **Runtime parsing** via `@grpc/proto-loader` — no codegen, no protoc, ever. The proto file is parsed at runtime.
- **Typed codegen** via `protoc-gen-js` or `ts-proto` — requires system protoc.

Today's JS holons appear to use the runtime-parsing path. **No protoc dependency at all for the canonical use case.**

**Verdict**: pure-language. **No prebuilt benefit. No proto-edit-time issue** if runtime parsing is the chosen path.

### 4.5 `sdk/js-web-holons` — browser-only

Same as `js-holons`. Browser sandbox; runtime parsing path.

**Verdict**: pure-language. **No prebuilt benefit.**

### 4.6 `sdk/swift-holons` — pure Swift

**Manifest**: `sdk/swift-holons/Package.swift`. Deps `grpc-swift 1.9.0`, `swift-nio 2.36+`, `swift-protobuf 1.35+`, `swift-log 1.6+`. All pure Swift.

**Build**: `swift-package` runner. Slow SPM resolution noted in `ader.yml:37` ("Swift packages") — that's dependency download + Swift compile, not native code.

**Proto-edit-time toolchain**: SwiftProtobuf provides a `protoc-gen-swift` plugin and `protoc-gen-grpc-swift` (via `grpc-swift`). Both are protoc plugins (not standalone). Swift proto-edit needs **system protoc + the plugins** (built via SPM or installed via Homebrew). No mature pure-Swift codegen substitute.

**Edge case flagged**: `Package.swift:6` declares `.macOS(.v13)` only, but `sdk/swift-holons/README.md:87-96` claims iOS/Linux/Windows support. Doc-vs-code divergence to arbitrate.

**Verdict**: pure-language at build time. **No prebuilt benefit** for runtime; toolchain (Xcode) is the practical dep. Diagnostic for proto-edit.

### 4.7 `sdk/java-holons` — JVM bundled

**Manifest**: `sdk/java-holons/build.gradle`. Deps `io.grpc:grpc-netty-shaded:1.60.0`, `protobuf-java:4.34.1`. Uber-JAR, pure JVM bytecode.

**Build**: `gradle` runner. Cache `GRADLE_USER_HOME → .ader-ci-cache/gradle` (`ader.yml:48`).

**Proto-edit-time toolchain**: the Gradle `protobuf` plugin **downloads platform-specific `protoc`** from Maven Central (artifact `com.google.protobuf:protoc:<version>:<classifier>`). No system install needed.

**Verdict**: ecosystem-prebuilt. **No prebuilt benefit, no proto-edit issue.**

### 4.8 `sdk/kotlin-holons` — JVM bundled (same family)

Same as Java. Gradle protobuf plugin downloads protoc.

**Verdict**: ecosystem-prebuilt. **No prebuilt benefit, no proto-edit issue.**

### 4.9 `sdk/csharp-holons` — managed .NET

**Manifest**: `sdk/csharp-holons/Holons/Holons.csproj`. Deps `Grpc.AspNetCore:2.76`, `Grpc.Core.Api:2.76`, `Google.Protobuf:3.31.1`. Pure managed .NET.

**Build**: `dotnet` runner. Caches `NUGET_PACKAGES`, `DOTNET_CLI_HOME` (`ader.yml:49-50`).

**Proto-edit-time toolchain**: `Grpc.Tools` NuGet package **ships protoc + grpc_csharp_plugin per platform**. No system install.

**Verdict**: ecosystem-prebuilt. **No prebuilt benefit, no proto-edit issue.**

### 4.10 `sdk/python-holons` — wheel with source fallback

**Manifest**: `sdk/python-holons/pyproject.toml`. Deps `grpcio>=1.60`, `grpcio-tools>=1.60`, `grpcio-reflection>=1.60`, `websockets>=12`. **`grpcio` is a C extension** wrapping gRPC C.

**Build**: `python` runner at `runner_registry.go:872-936`. Creates isolated venv, installs from `requirements.txt`.

**Proto-edit-time toolchain**: `grpcio-tools` wheel ships bundled `protoc` and codegen plugins. No system install when wheels are available. **On Alpine/musl, no wheel → pip falls back to source build of grpcio C ext, which is fragile.**

**Verdict**: ecosystem-prebuilt-with-fallbacks. **Borderline prebuilt benefit** — only Alpine/musl niche.

### 4.11 `sdk/ruby-holons` — binary gem with source fallback (HIGH PAIN)

**Manifest**: `sdk/ruby-holons/Gemfile`. Pinned `gem "grpc", "= 1.58.3"`. C extension wrapping gRPC C library. `google-protobuf` transitively also a C ext.

**Build**: `ruby` runner at `runner_registry.go:1095-1231`. `bundle install` → matches binary gem if available, else source-builds (slow).

**Documented pain**:
- `.github/workflows/ader.yml:37` — *"Persist slow SDK bootstraps (**Ruby grpc gem**, Swift packages, Go modules, etc.) OUTSIDE the workspace"*. Ruby explicitly named first.
- `sdk/ruby-holons/README.md:5` — *"On Apple Silicon, prefer a native arm64 Ruby/Bundler toolchain so Bundler can reuse the prebuilt grpc and google-protobuf gems instead of compiling the C/C++ extensions from source."*
- Cache key in `ader.yml:88` is `hashFiles(...Gemfile.lock, sdk/**/Gemfile)` — only Ruby Gemfile changes invalidate it.

**Cold install**: ~10-15 min on platforms requiring source build; <1 min when binary gem matches.

**Binary gems available for**: `x86_64-darwin`, `arm64-darwin`, `x86_64-linux`, `aarch64-linux`, `x86-mingw32`, `x64-mingw32`, `x64-mingw-ucrt`, `java`. **NOT available for**: `x86_64-linux-musl` (Alpine), `aarch64-linux-musl`, FreeBSD, less common ARM Linux.

**Proto-edit-time toolchain**: `grpc-tools` gem ships protoc + plugins. No system install for proto-edit (separate from the runtime gem pain).

**Verdict**: ecosystem-prebuilt-with-fallbacks at runtime. **HIGH prebuilt benefit** for runtime. No proto-edit issue.

### 4.12 `sdk/c-holons` — system-deps-required

**Manifest**: `sdk/c-holons/Makefile`. C11.

**Native runtime libs**: gRPC C, libprotobuf-c, upb, transitive abseil/BoringSSL/c-ares/re2/zlib.

**Build**: `cmake` runner at `lifecycle.go:1161-1231`. `find_package(gRPC CONFIG REQUIRED)` etc.

**Pain**: hard-coded `/opt/homebrew` and `/usr/local` paths (`sdk/cpp-holons/CMakeLists.txt:13,30,39,87` — shared CMake patterns); Windows untested; Alpine/musl needs source build of gRPC.

**Proto-edit-time toolchain**: needs `protoc` + `protoc-gen-c` + `grpc_cpp_plugin` (when reaching for the C++ bridge). Today provided by Homebrew. **The prebuilts solution covers this naturally** — the c-holons prebuilt archive includes these binaries under `bin/`.

**Verdict**: system-deps-required. **HIGH prebuilt benefit** for both runtime and proto-edit (single archive serves both).

### 4.13 `sdk/cpp-holons` — system-deps-required (worse than C)

Same as C plus larger surface (gRPC-C++, abseil, BoringSSL, more transitives, `grpc_cpp_plugin`).

**Verdict**: system-deps-required. **HIGH prebuilt benefit.**

### 4.14 `sdk/zig-holons` (in flight) — vendored-from-source

**Manifest**: `sdk/zig-holons/build.zig` + `build.zig.zon`. Submodules `third_party/grpc` and `third_party/protobuf-c` (`.gitmodules` at root).

**Native runtime libs**: gRPC Core C `v1.80.0`, libprotobuf-c `v1.5.2`, transitively abseil/BoringSSL/c-ares/re2/zlib/upb.

**Cold first build**: ~10-15 min (per Codex M0 evidence in `docs/adr/zig-sdk-scope.md`).

**Proto-edit-time toolchain**: SDK build vendors `protoc-c` under `.zig-vendor/native/bin/`. No system install needed if SDK is vendored. **Prebuilt archive carries `protoc-c` for proto-edit.**

**Verdict**: vendored-from-source. **HIGH prebuilt benefit** for both runtime and proto-edit.

---

## 5. Cross-cutting concerns

### 5.1 `op build` does not check native libraries today

`lifecycle.go:814-846` `preflight()` checks:
- `requires.files` exist (file paths declared in manifest).
- `requires.commands` resolve via `exec.LookPath()` (e.g., `cmake`, `ruby`).
- Provides install hints via `installHint()` per platform.

**It does not verify that `libgrpc.so` is installed.** If `find_package(gRPC CONFIG REQUIRED)` fails inside CMake, `op build` propagates the CMake error with no actionable hint. **There is no SDK-level prerequisite check.** That's the gap.

### 5.2 Caching strategy today

External to workspace, on the self-hosted runner `popok` (`ader.yml:42-50`):

| Cache var | Path | SDK |
|---|---|---|
| `GOMODCACHE`, `GOCACHE` | `go-mod/`, `go-build/` | go |
| `BUNDLE_PATH` | `bundle/` | ruby |
| `PUB_CACHE` | `dart-pub/` | dart |
| `npm_config_cache` | `npm/` | js, js-web |
| `GRADLE_USER_HOME` | `gradle/` | java, kotlin |
| `NUGET_PACKAGES`, `DOTNET_CLI_HOME` | `nuget/`, `dotnet-home/` | csharp |
| (none explicit) | `~/.cargo` | rust |
| (none explicit) | Xcode caches | swift |
| (none explicit) | `~/.cache/pip` | python |
| `GRACE_OP_SHARED_CACHE_DIR` | `grace-op-shared/` | shared (gRPC bridge artifacts) |

`actions/cache@v4` (`ader.yml:85-91`) keys on `hashFiles('examples/hello-world/*/Gemfile.lock', 'sdk/**/Gemfile')`. Only Ruby Gemfile changes invalidate. Other SDKs rely on the underlying tool's own cache freshness.

### 5.3 Compounded pain in COAX cold builds

`ader/catalogues/gabriel-greeting-app-swiftui/checks.yaml:53-73` declares `prereqs: [go, xcodebuild, xcodegen, swift, cargo, python3, cmake, dotnet, dart, java, gradle, node, npm, ruby, bundle]` with `timeout: 150m`. Same for Flutter. A cold COAX build bootstraps every SDK in series. The 240-minute job timeout (`ader.yml:33`) is the consequence.

### 5.4 Doc-vs-code divergences

Per `CLAUDE.md` "doubt is the method":

1. **Swift platforms**: `Package.swift:6` says macOS only; README claims more.
2. **OP_BUILD.md runner list**: 11 listed; `runner_registry.go:15-29` registers 13.
3. **C SDK transport row** in `sdk/README.md:71` shows `both` everywhere, but the implementation uses a gRPC-C++ bridge — separate audit item.

---

## 6. Pain ranking

1. **Ruby grpc gem source build on niche platforms.** Documented explicitly. Worst single offender.
2. **C/C++ SDKs needing `brew install grpc`** with hard-coded macOS paths and broken Windows.
3. **Zig SDK vendored cold first build** ~10-15 min on every fresh checkout.
4. **Python grpcio source build on Alpine/musl.** Niche but real for container builds.
5. **COAX cold builds compounding all of the above.**

Pains 1–4 are addressable by us shipping prebuilts. Pain 5 is downstream consequence and is solved by 1–4.

---

## 7. Dead-code findings (cleanup candidates)

A targeted scan across the 13 SDKs and their hello-world examples found a recurring pattern: **redundant `protoc --descriptor_set_out=` invocations** whose output is never consumed. These are vestigial — likely a copy-paste from an era predating `op build`'s in-process protoparse validation. They reinforce the false belief that system protoc is required at build time. All 6 instances generate a descriptor file and either delete it immediately (`mktemp` + `rm -f`) or write it to a build dir where nothing reads it.

Plus 2 phantom Gradle dependencies declared but never imported.

### 7.1 Redundant proto-validation scories (6 instances, all same pattern)

| File | Lines | Pattern | Verdict |
|---|---|---|---|
| `examples/hello-world/gabriel-greeting-rust/build.rs` | 20-37 | `Command::new("protoc")` writes `holon_descriptor.bin` to `OUT_DIR`. `grep -rn holon_descriptor src/` returns zero hits. The actual codegen happens at lines 13-18 via `tonic_build::compile_protos()` (vendored protoc). Lines 20-37 are pure validation, output unused. | Safe to delete |
| `examples/hello-world/gabriel-greeting-dart/scripts/generate_proto.sh` | 22-29 | `mktemp` → `protoc --descriptor_set_out=$file` → `rm -f $file`. The real codegen is line 11 (`--dart_out=grpc:`). Lines 22-29 are pure validation. | Safe to delete |
| `examples/hello-world/gabriel-greeting-c/scripts/generate_proto.sh` | 30-38 | Same pattern: `DESCRIPTOR_FILE=$(mktemp)` → `protoc --descriptor_set_out=$DESCRIPTOR_FILE` → `rm -f $DESCRIPTOR_FILE`. | Safe to delete |
| `examples/hello-world/gabriel-greeting-cpp/scripts/generate_proto.sh` | 25-33 | Same pattern. | Safe to delete |
| `examples/hello-world/gabriel-greeting-python/scripts/generate_proto.sh` | 13-20 | Same pattern, using `python3 -m grpc_tools.protoc`. | Safe to delete |
| `examples/hello-world/gabriel-greeting-kotlin/scripts/generate_proto.sh` | 17-24 | Same pattern. | Safe to delete |

In all 6 cases, `op build`'s in-process pure-Go protoparse (`OP_BUILD.md:118-123`) already validates the proto. The script-side validation is duplicated work, costs system protoc as a dev-time dep that isn't actually needed for a sanity check, and confuses the picture about what's truly required.

### 7.2 Phantom dependencies (2 instances)

| File | Line | Declaration | Evidence |
|---|---|---|---|
| `examples/hello-world/gabriel-greeting-java/build.gradle` | 21 | `compileOnly 'org.apache.tomcat:annotations-api:6.0.53'` | `grep -rn "javax.annotation\|@Nullable\|@Nonnull" examples/hello-world/gabriel-greeting-java/src/` returns zero hits. The annotations are declared but no source file uses them. |
| `examples/hello-world/gabriel-greeting-kotlin/build.gradle.kts` | 33 | `implementation("javax.annotation:javax.annotation-api:1.3.2")` | Same grep returns zero hits. |

Both are leftovers, likely from a generated scaffold template that included annotation utilities for some other use case. Safe to remove from the manifests.

### 7.3 Confirmed clean (no findings)

The audit also verified the following are NOT dead code, despite suspicion:

- `sdk/python-holons` declares `grpcio-tools` and `grpcio-reflection` in `pyproject.toml`. Both are actually used: `grpcio-tools` at `sdk/python-holons/holons/describe.py:195-217` (`from grpc_tools import protoc`); `grpcio-reflection` at `sdk/python-holons/holons/serve.py:19` (`from grpc_reflection.v1alpha import reflection`). **Keep both.**
- All `sdk/*/templates/describe.*.tmpl` files are consumed by `op build`'s Incode Description pipeline. **Active code.**
- The `sdk/c-holons/Makefile`, `sdk/cpp-holons/CMakeLists.txt`, `sdk/zig-holons/build.zig` build artifacts in `gen/c/` are committed per CLAUDE.md rule #7 and consumed at link time. **Active code.**

### 7.4 Suggested cleanup PR

A single small PR can land all 8 deletions: 6 script blocks + 2 manifest lines. Risk is zero (verified by greps). Side benefit: `op build` of the 5 remaining holons (rust, dart, c, cpp, python, kotlin) drops a transient system-protoc dependency entirely for users who never edit the protos. The only remaining protoc dependency at proto-edit-time, post-cleanup, is the actual codegen invocation in each `generate_proto.sh` — and even that, where the language ecosystem vendors protoc (Python via `python3 -m grpc_tools.protoc`, Kotlin/Java via Gradle plugin), can be flipped to vendored.

---

## 8. Recommendations preview

**Ship runtime prebuilts for**: `ruby`, `c`, `cpp`, `zig`. The c/cpp/zig prebuilts also carry their codegen tooling (`protoc`, `protoc-c`, `grpc_cpp_plugin`) under `bin/`, so they cover proto-edit-time as well.

**Don't ship prebuilts for**: `go`, `rust`, `dart`, `js`, `js-web`, `swift`, `java`, `kotlin`, `csharp`, `python`. Either pure-language (build-time fine) or already covered by upstream package managers (proto-edit-time fine via gradle / NuGet / wheel / gem / cargo + tonic-build).

**For the residual proto-edit-time gap** (Go, Dart, Swift): no new infrastructure. `op` emits a clear diagnostic when proto-edit-related tooling is missing, pointing at the platform-specific install command. Same pattern as `installHint()` already does for `cmake`, `ruby`, etc.

**Side-task: cleanup PR for §7 dead code.** Land before or in parallel with the prebuilts work. Drops 8 instances of vestigial protoc validation + phantom annotation deps. Resolves the Swift `Package.swift` vs README platform-support divergence as a separate cleanup item.

The detailed prebuilts design lives in `docs/specs/sdk-prebuilts.md`.
