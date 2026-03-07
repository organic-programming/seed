# Recipe Implementation Guide — macOS

This document describes how to create all recipe combinations that can
be built on macOS. Hand it to Codex or any AI agent as a self-contained
implementation prompt.

---

## Failure Handling Policy

> **IMPORTANT:** If you are blocked on a recipe after **3 serious
> attempts** (e.g. dependency resolution fails, build tool incompatibility,
> proto codegen errors), you MUST:
>
> 1. **Write a failure report** in the recipe's `BLOCKED.md` file.
>    Include: what you tried, exact error messages, suspected root cause,
>    and what would be needed to unblock.
> 2. **Commit and push** the partial work + `BLOCKED.md` so nothing is lost.
> 3. **Move on to the next recipe** in the execution order.
>
> Do NOT spend more than 3 attempts on a single recipe. The goal is to
> make as much progress as possible across ALL recipes, not to get stuck
> on one. A well-written `BLOCKED.md` is more valuable than an incomplete
> fix loop.

---

## Context

The Organic Programming ecosystem organizes cross-language applications
as **recipes**: a backend daemon (Go or Rust) paired with a frontend UI
framework. All recipes share the same `greeting.proto` contract and
produce a "Gudule Greeting" sample app that greets users in 56 languages.

Two recipes already exist and serve as **reference implementations**:

- `go-dart-holons/` — Go daemon + Flutter UI (all platforms)
- `go-swift-holons/` — Go daemon + SwiftUI app (macOS)

Study them before creating new recipes. Their structure, naming, and
`holon.yaml` format are the template for everything below.

### Workspace Layout

```
organic-programming/
├── sdk/
│   ├── go-holons/           # Go SDK (transport, serving, identity)
│   └── rust-holons/         # Rust SDK (tonic serving, transport)
├── holons/
│   └── grace-op/            # op CLI (check, build, test, clean)
└── recipes/
    ├── README.md
    ├── go-dart-holons/      # ✅ reference — Go + Flutter
    ├── go-swift-holons/     # ✅ reference — Go + SwiftUI
    └── <new recipes here>
```

### Proto Contract (shared by ALL recipes)

```protobuf
syntax = "proto3";
package greeting.v1;

service GreetingService {
  rpc ListLanguages(ListLanguagesRequest) returns (ListLanguagesResponse);
  rpc SayHello(SayHelloRequest) returns (SayHelloResponse);
}

message ListLanguagesRequest {}
message ListLanguagesResponse { repeated Language languages = 1; }
message Language {
  string code = 1;    // ISO 639-1: "fr"
  string name = 2;    // "French"
  string native = 3;  // "Français"
}
message SayHelloRequest {
  string name = 1;
  string lang_code = 2;
}
message SayHelloResponse {
  string greeting = 1;
  string language = 2;
  string lang_code = 3;
}
```

The daemon hardcodes 56 languages (see `go-dart-holons/examples/greeting/
greeting-daemon/internal/greetings.go` for the full list).

---

## Recipe Structure Convention

Every recipe must follow this layout:

```
<backend>-<frontend>-holons/
├── README.md
├── APPS.md                        # architecture + transport details
├── examples/
│   └── greeting/
│       ├── holon.yaml             # kind: composite, runner: recipe
│       ├── greeting-daemon/       # backend daemon
│       │   ├── holon.yaml         # kind: native
│       │   └── <source>
│       └── greeting-<name>/       # frontend app
│           └── <source>
└── .gitignore
```

### Naming Conventions

| Field | Pattern | Example |
|-------|---------|---------|
| Repo name | `<backend>-<frontend>-holons` | `rust-dart-holons` |
| Frontend dir | `greeting-<shortname>` | `greeting-rustdart` |
| Composite family_name | `Greeting-<Pattern>` | `Greeting-Rustdart` |
| Daemon binary | `gudule-daemon-greeting-<pattern>` | `gudule-daemon-greeting-rustdart` |
| App artifact | `gudule-greeting-<pattern>` | `gudule-greeting-rustdart` |

### Composite `holon.yaml` Template

```yaml
schema: holon/v0
given_name: gudule
family_name: Greeting-<Pattern>
kind: composite
build:
  runner: recipe
  defaults:
    target: macos
    mode: debug
  members:
    - id: daemon
      path: greeting-daemon
      type: holon
    - id: app
      path: greeting-<name>
      type: component
  targets:
    macos:
      steps:
        - build_member: daemon
        - exec:
            cwd: greeting-<name>
            argv: [<build command>]
        - assert_file:
            path: <artifact path>
requires:
  commands: [<required commands>]
  files: [<required files>]
platforms: [<supported platforms>]
artifacts:
  primary_by_target:
    macos:
      debug: <artifact path>
  binary: gudule-greeting-<pattern>
```

---

## Recipes Buildable on macOS

macOS has access to: `go`, `rustc`/`cargo`, `flutter`, `swift`/`xcodebuild`,
Android SDK + `gradle`, `npm`/`node`, `cmake`/`qt`, and `dotnet` (limited).

### 1. `rust-dart-holons` — Rust + Flutter

**Priority: HIGH** — proves the cross-backend story.

#### Daemon (Rust)

Create a Rust project that implements `GreetingService` using `tonic`:

```
greeting-daemon/
├── Cargo.toml
├── build.rs           # tonic-build for proto codegen
├── holon.yaml         # kind: native, runner: ??? (see note)
├── proto/
│   └── greeting.proto
└── src/
    ├── main.rs        # tonic server, listen on tcp://
    ├── greetings.rs   # 56 hardcoded languages (port from Go)
    └── service.rs     # GreetingService impl
```

**Cargo.toml dependencies:**
- `tonic` (gRPC server)
- `prost` (protobuf)
- `tokio` (async runtime)
- `tonic-build` (build dependency)

**Important:** `grace-op` does not have a `cargo` runner yet. The daemon
`holon.yaml` should use the recipe runner at the composite level. The
daemon `holon.yaml` can declare `kind: native` with `build.runner: go-module`
replaced by a future `cargo` runner. For now, the composite recipe can
use `exec` steps with `cargo build` instead of `build_member`.

Alternatively, to make `build_member` work, add a minimal `cargo` runner
to `grace-op` that calls `cargo build --release -o <binary>`.

**Recipe `holon.yaml` (macOS target):**
```yaml
targets:
  macos:
    steps:
      - exec:
          cwd: greeting-daemon
          argv: ["cargo", "build"]
      - exec:
          cwd: greeting-rustdart
          argv: ["flutter", "pub", "get"]
      - exec:
          cwd: greeting-rustdart
          argv: ["flutter", "build", "macos", "--debug"]
      - assert_file:
          path: greeting-rustdart/build/macos/Build/Products/Debug/gudule-greeting-rustdart.app
```

#### Frontend (Flutter)

Copy from `go-dart-holons/examples/greeting/greeting-godart/`. Change:
- Directory name → `greeting-rustdart`
- Bundle name → `gudule-greeting-rustdart`
- Daemon binary name → `gudule-daemon-greeting-rustdart`
- The daemon launch code to point to the new binary name

The Flutter code is almost identical — only the daemon binary name changes
in the process launch logic.

---

### 2. `go-kotlin-holons` — Go + Jetpack Compose

**Priority: HIGH** — completes the native mobile story.

#### Daemon (Go)

Copy from `go-dart-holons`. Update module path, binary name to
`gudule-daemon-greeting-gokotlin`.

#### Frontend (Jetpack Compose)

```
greeting-gokotlin/
├── build.gradle.kts
├── settings.gradle.kts
├── gradle/
├── src/main/kotlin/
│   ├── Main.kt                 # Compose Desktop app entry
│   ├── ui/
│   │   ├── ContentView.kt     # language list + greeting panel
│   │   └── Theme.kt
│   └── grpc/
│       ├── DaemonProcess.kt   # ProcessBuilder to launch daemon
│       └── GreetingClient.kt  # gRPC stub (io.grpc)
└── proto/
    └── greeting.proto
```

**Build tool:** Gradle with `org.jetbrains.compose` plugin for desktop.

**Dependencies:**
- `io.grpc:grpc-kotlin-stub` (gRPC client)
- `com.google.protobuf:protobuf-kotlin` (proto stubs)
- `org.jetbrains.compose.desktop:desktop` (Compose Desktop)

**Recipe steps (macOS):**
```yaml
steps:
  - build_member: daemon
  - exec:
      cwd: greeting-gokotlin
      argv: ["./gradlew", "createDistributable"]
  - assert_file:
      path: greeting-gokotlin/build/compose/binaries/main/app/gudule-greeting-gokotlin.app
```

**Required commands:** `go`, `java` (JDK 17+), `gradle` (via wrapper)

---

### 3. `go-web-holons` — Go + TypeScript (Web)

**Priority: HIGH** — most universal platform.

#### Daemon (Go)

Copy from `go-dart-holons`. Add **gRPC-Web** or **Connect** support
to the daemon so browsers can call it. The simplest approach:

- Use [Connect-Go](https://connectrpc.com/docs/go/getting-started/)
  which serves both gRPC and Connect (HTTP/1.1+JSON) on the same port.
- Or use [grpcwebproxy](https://github.com/nicholasgasior/grpc-web-proxy) as a sidecar.

Update binary name to `gudule-daemon-greeting-goweb`.

#### Frontend (TypeScript/Web)

```
greeting-goweb/
├── package.json
├── tsconfig.json
├── index.html
├── src/
│   ├── main.ts
│   ├── grpc-client.ts    # @connectrpc/connect-web client
│   └── ui.ts             # DOM: language list + greeting display
└── proto/
    └── greeting.proto
```

**Build tool:** `vite` (dev server + bundler).

**Dependencies:**
- `@connectrpc/connect-web` (Connect/gRPC-Web client)
- `@bufbuild/protobuf` (proto stubs)
- `vite` (build)

**Recipe steps (macOS):**
```yaml
steps:
  - build_member: daemon
  - exec:
      cwd: greeting-goweb
      argv: ["npm", "install"]
  - exec:
      cwd: greeting-goweb
      argv: ["npm", "run", "build"]
  - assert_file:
      path: greeting-goweb/dist/index.html
```

**Required commands:** `go`, `node`, `npm`

---

### 4. `rust-swift-holons` — Rust + SwiftUI

#### Daemon (Rust)

Same as `rust-dart-holons` daemon. Copy or symlink.
Binary name: `gudule-daemon-greeting-rustswift`.

#### Frontend (SwiftUI)

Copy from `go-swift-holons/examples/greeting/greeting-swiftui/`.
Change daemon binary name to `gudule-daemon-greeting-rustswift`.

**Recipe steps:** Same as `go-swift-holons` but with `cargo build`
instead of `build_member daemon`.

---

### 5. `rust-kotlin-holons` — Rust + Jetpack Compose

#### Daemon (Rust)

Same Rust daemon. Binary name: `gudule-daemon-greeting-rustkotlin`.

#### Frontend (Compose)

Copy from `go-kotlin-holons`. Change daemon binary name.

---

### 6. `rust-web-holons` — Rust + TypeScript (Web)

#### Daemon (Rust)

Same Rust daemon. Add Connect/gRPC-Web support via `tonic-web`.
Binary name: `gudule-daemon-greeting-rustweb`.

#### Frontend (Web)

Copy from `go-web-holons`. Change daemon binary name.

---

### 7. `go-qt-holons` — Go + Qt/C++

#### Daemon (Go)

Copy from `go-dart-holons`. Binary name: `gudule-daemon-greeting-goqt`.

#### Frontend (Qt)

```
greeting-goqt/
├── CMakeLists.txt
├── src/
│   ├── main.cpp
│   ├── MainWindow.h / .cpp     # QMainWindow with QListWidget + QLabel
│   ├── DaemonProcess.h / .cpp  # QProcess to launch daemon
│   └── GreetingClient.h / .cpp # gRPC client (grpc++ C++ API)
└── proto/
    └── greeting.proto
```

**Dependencies:** Qt6 (Widgets or Quick), gRPC C++, Protobuf.
**Build tool:** CMake.

**Recipe steps (macOS):**
```yaml
steps:
  - build_member: daemon
  - exec:
      cwd: greeting-goqt
      argv: ["cmake", "-B", "build", "-DCMAKE_BUILD_TYPE=Debug"]
  - exec:
      cwd: greeting-goqt
      argv: ["cmake", "--build", "build"]
  - assert_file:
      path: greeting-goqt/build/gudule-greeting-goqt
```

**Required commands:** `go`, `cmake`, `qt6` (from Homebrew)

---

### 8. `rust-qt-holons` — Rust + Qt/C++

Same as `go-qt-holons` but with Rust daemon.

---

### 9. `go-dotnet-holons` — Go + .NET MAUI

**Limited on macOS.** .NET MAUI supports macOS (via Mac Catalyst),
but WinUI 3 (the primary target) requires Windows. On macOS, build the
Mac Catalyst variant.

#### Daemon (Go)

Copy from `go-dart-holons`. Binary name: `gudule-daemon-greeting-godotnet`.

#### Frontend (.NET MAUI)

```
greeting-godotnet/
├── GreetingDotnet.csproj
├── Platforms/
│   ├── MacCatalyst/
│   └── Windows/
├── MainPage.xaml / .cs
├── Services/
│   ├── DaemonProcess.cs    # Process.Start for daemon
│   └── GreetingClient.cs   # Grpc.Net.Client
└── Protos/
    └── greeting.proto
```

**Build tool:** `dotnet build` / `dotnet publish`.
**Required:** .NET 8 SDK.

---

### 10. `rust-dotnet-holons` — Rust + .NET MAUI

Same as `go-dotnet-holons` but with Rust daemon.

---

## GitHub Repo Creation

For each recipe, create the GitHub repository in the `organic-programming`
organization:

```bash
gh repo create organic-programming/<recipe-name> \
  --public \
  --description "<Go|Rust> backend + <Frontend> — gRPC <transport>"
```

Then add it as a submodule in `organic-programming/recipes/`:

```bash
cd organic-programming
git submodule add git@github.com:organic-programming/<recipe-name>.git recipes/<recipe-name>
```

## Verification

After creating each recipe, verify with `op`:

```bash
# From grace-op directory
go run ./cmd/op check ../../recipes/<recipe-name>/examples/greeting
go run ./cmd/op build ../../recipes/<recipe-name>/examples/greeting --dry-run
```

Both commands must succeed. The dry-run should show the full build plan
including child daemon builds.

## Order of Execution

Build recipes in this order (each depends on the previous):

1. `rust-dart-holons` — first Rust recipe, reuses Flutter
2. `go-kotlin-holons` — first Compose recipe
3. `go-web-holons` — first web recipe
4. `rust-swift-holons` — clone of go-swift-holons with Rust daemon
5. `rust-kotlin-holons` — clone of go-kotlin-holons with Rust daemon
6. `rust-web-holons` — clone of go-web-holons with Rust daemon
7. `go-qt-holons` — Qt/C++ recipe
8. `rust-qt-holons` — clone with Rust daemon
9. `go-dotnet-holons` — .NET recipe (Mac Catalyst variant)
10. `rust-dotnet-holons` — clone with Rust daemon
