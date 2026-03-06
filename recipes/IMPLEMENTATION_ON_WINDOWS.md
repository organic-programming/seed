# Recipe Implementation Guide — Windows

This document describes how to create all recipe combinations that can
be built on Windows. Hand it to Codex or any AI agent as a self-contained
implementation prompt.

---

## Context

Read `IMPLEMENTATION_ON_MAC_OS.md` first — it contains the shared proto
contract, naming conventions, composite `holon.yaml` template, and
workspace layout. This document only covers Windows-specific differences.

**Key differences from macOS:**

- No SwiftUI/Xcode — `go-swift-holons` and `rust-swift-holons` cannot be
  built on Windows.
- WinUI 3 / .NET MAUI is the **primary** platform — these recipes should
  be fully developed on Windows.
- Flutter builds use `flutter build windows`.
- Qt builds use Visual Studio or MinGW.
- Go cross-compile from Windows to Linux is trivial.
- Rust cross-compile uses `rustup target add`.

---

## Recipes Buildable on Windows

Windows has access to: `go`, `rustc`/`cargo`, `flutter`, Android SDK +
`gradle`, `npm`/`node`, `cmake`/`qt`, `dotnet`, Visual Studio.

### Available Recipes

| Recipe | Frontend | Status |
|--------|----------|--------|
| `go-dart-holons` | Flutter | ✅ exists, build Windows target |
| `go-swift-holons` | SwiftUI | ❌ not buildable on Windows |
| `go-kotlin-holons` | Jetpack Compose | buildable (JVM desktop) |
| `go-web-holons` | TypeScript | buildable |
| `go-dotnet-holons` | **WinUI 3** | **primary platform** |
| `go-qt-holons` | Qt/C++ | buildable (MSVC or MinGW) |
| `rust-dart-holons` | Flutter | buildable |
| `rust-swift-holons` | SwiftUI | ❌ not buildable on Windows |
| `rust-kotlin-holons` | Jetpack Compose | buildable (JVM desktop) |
| `rust-web-holons` | TypeScript | buildable |
| `rust-dotnet-holons` | **WinUI 3** | **primary platform** |
| `rust-qt-holons` | Qt/C++ | buildable (MSVC or MinGW) |

---

## Windows-Specific Build Details

### 1. `go-dotnet-holons` — Go + WinUI 3 (PRIMARY)

This is the recipe that **must** be developed on Windows. WinUI 3 is
Windows-only.

#### Daemon (Go)

Copy from `go-dart-holons`. Binary name: `gudule-daemon-greeting-godotnet.exe`.

**Transport:** Named Pipes (`\\.\pipe\gudule-greeting`). The Go daemon
should accept `--listen npipe://gudule-greeting` in addition to
`tcp://` and `stdio://`. The `go-holons` SDK already supports named
pipes on Windows.

#### Frontend (WinUI 3)

```
greeting-godotnet/
├── GreetingDotnet.sln
├── GreetingDotnet/
│   ├── GreetingDotnet.csproj
│   ├── App.xaml / .cs
│   ├── MainWindow.xaml / .cs    # language list + greeting panel
│   ├── Services/
│   │   ├── DaemonProcess.cs     # System.Diagnostics.Process
│   │   └── GreetingClient.cs    # Grpc.Net.Client
│   └── Protos/
│       └── greeting.proto
├── Package.appxmanifest
└── Properties/
    └── launchSettings.json
```

**Project type:** WinUI 3 (Windows App SDK).
**Target framework:** `net8.0-windows10.0.19041.0`.

**NuGet dependencies:**
- `Grpc.Net.Client` (gRPC client)
- `Google.Protobuf` (proto stubs)
- `Grpc.Tools` (proto codegen via MSBuild)
- `Microsoft.WindowsAppSDK` (WinUI 3)

**Recipe steps (Windows):**
```yaml
targets:
  windows:
    steps:
      - build_member: daemon
      - exec:
          cwd: greeting-godotnet
          argv: ["dotnet", "restore"]
      - exec:
          cwd: greeting-godotnet
          argv: ["dotnet", "build", "-c", "Debug"]
      - assert_file:
          path: greeting-godotnet/bin/Debug/net8.0-windows10.0.19041.0/GreetingDotnet.exe
```

**Required commands:** `go`, `dotnet`

**holon.yaml:**
```yaml
kind: composite
build:
  runner: recipe
  defaults:
    target: windows
    mode: debug
  members:
    - id: daemon
      path: greeting-daemon
      type: holon
    - id: app
      path: greeting-godotnet
      type: component
  targets:
    windows:
      steps:
        - build_member: daemon
        - exec:
            cwd: greeting-godotnet
            argv: ["dotnet", "build", "-c", "Debug"]
        - copy:
            from: greeting-daemon/gudule-daemon-greeting-godotnet.exe
            to: greeting-godotnet/bin/Debug/net8.0-windows10.0.19041.0/gudule-daemon-greeting-godotnet.exe
        - assert_file:
            path: greeting-godotnet/bin/Debug/net8.0-windows10.0.19041.0/GreetingDotnet.exe
platforms: [windows]
requires:
  commands: [go, dotnet]
  files: [greeting-daemon/go.mod, greeting-godotnet/GreetingDotnet.csproj]
artifacts:
  primary_by_target:
    windows:
      debug: greeting-godotnet/bin/Debug/net8.0-windows10.0.19041.0/GreetingDotnet.exe
  binary: gudule-greeting-godotnet
```

---

### 2. `rust-dotnet-holons` — Rust + WinUI 3

Same as `go-dotnet-holons` but with Rust daemon (`cargo build`).
Binary: `gudule-daemon-greeting-rustdotnet.exe`.

---

### 3. `go-dart-holons` — Windows Target

The recipe already exists. On Windows, add a `windows` target:

```yaml
windows:
  steps:
    - build_member: daemon
    - exec:
        cwd: greeting-godart
        argv: ["flutter", "pub", "get"]
    - exec:
        cwd: greeting-godart
        argv: ["flutter", "build", "windows", "--debug"]
    - copy:
        from: greeting-daemon/gudule-daemon-greeting-godart.exe
        to: greeting-godart/build/windows/x64/runner/Debug/gudule-daemon-greeting-godart.exe
    - assert_file:
        path: greeting-godart/build/windows/x64/runner/Debug/gudule-greeting-godart.exe
```

---

### 4. `go-kotlin-holons` — Windows (Compose Desktop)

Compose Desktop runs on JVM and works identically on Windows.
The `gradlew.bat` wrapper handles Windows.

```yaml
windows:
  steps:
    - build_member: daemon
    - exec:
        cwd: greeting-gokotlin
        argv: ["gradlew.bat", "createDistributable"]
    - assert_file:
        path: greeting-gokotlin/build/compose/binaries/main/app/gudule-greeting-gokotlin/gudule-greeting-gokotlin.exe
```

---

### 5. `go-web-holons` — Windows

Identical to macOS. `node`, `npm`, `vite` work the same.

```yaml
windows:
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

---

### 6. `go-qt-holons` — Windows (MSVC or MinGW)

Qt on Windows uses Visual Studio generator or MinGW.

```yaml
windows:
  steps:
    - build_member: daemon
    - exec:
        cwd: greeting-goqt
        argv: ["cmake", "-B", "build", "-G", "Visual Studio 17 2022",
               "-DCMAKE_PREFIX_PATH=C:/Qt/6.7.0/msvc2019_64"]
    - exec:
        cwd: greeting-goqt
        argv: ["cmake", "--build", "build", "--config", "Debug"]
    - assert_file:
        path: greeting-goqt/build/Debug/gudule-greeting-goqt.exe
```

**Required:** Visual Studio 2022, Qt 6 (MSVC build), CMake.

---

### 7–10. Rust Variants

Each Rust variant replaces `build_member: daemon` with:

```yaml
- exec:
    cwd: greeting-daemon
    argv: ["cargo", "build"]
```

And uses the Rust binary name (`gudule-daemon-greeting-rust<frontend>.exe`).

---

## Windows-Specific Considerations

### Binary Extensions

All Windows binaries must end in `.exe`. The daemon `holon.yaml` must
declare `artifacts.binary: gudule-daemon-greeting-<pattern>.exe` on
Windows.

### Transport

| Transport | Go support | Rust support |
|-----------|-----------|-------------|
| `tcp://` | ✅ | ✅ |
| `stdio://` | ✅ | ✅ |
| `npipe://` (Named Pipes) | ✅ (`go-holons`) | ✅ (`tokio::net::windows`) |

Named Pipes are the Windows-native IPC mechanism. Use them instead of
Unix sockets. The daemon should accept `--listen npipe://gudule-greeting`.

### Path Separators

All paths in `holon.yaml` use forward slashes (`/`). `grace-op` handles
the conversion to backslashes internally via `filepath.FromSlash`.

### Long Path Support

Enable long paths in the Windows registry or use `\\?\` prefix for
paths exceeding 260 characters (common with `node_modules` and `.gradle`).

---

## GitHub Repo Creation

Same as macOS:

```powershell
gh repo create organic-programming/<recipe-name> `
  --public `
  --description "<Go|Rust> backend + <Frontend> — gRPC <transport>"
```

## Verification

```powershell
cd organic-programming\holons\grace-op
go run .\cmd\op check ..\..\recipes\<recipe-name>\examples\greeting
go run .\cmd\op build ..\..\recipes\<recipe-name>\examples\greeting --dry-run
```

## Order of Execution (Windows Priority)

1. `go-dotnet-holons` — **Windows-primary**, must be built here
2. `rust-dotnet-holons` — Rust variant of above
3. `go-dart-holons` — add `windows` target to existing recipe
4. `rust-dart-holons` — Windows target
5. `go-kotlin-holons` — Compose Desktop on Windows
6. `go-web-holons` — identical to macOS
7. `go-qt-holons` — Qt with MSVC
8. Remaining Rust variants
