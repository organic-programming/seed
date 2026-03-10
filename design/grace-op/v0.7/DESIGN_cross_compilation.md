# Cross-Compilation & Platform Targets

## Problem

`op build` always builds for the current host. To run holons on
mobile (iOS, Android) or browser (WASM), we need cross-compilation:
building a framework or WASM module from a desktop machine.

## Execution Modes

A holon's output format depends on the target platform. The same
source can produce three different artifacts:

| Mode | Output | Use case |
|---|---|---|
| **binary** | Standalone executable | Desktop, server |
| **framework** | Shared library (`.framework`, `.so`, `.aar`) | Mobile apps |
| **wasm** | WebAssembly module (`.wasm`) | Browser, edge |

The execution mode determines which transports are available.
See [PLATFORM_MATRIX.md](../../PLATFORM_MATRIX.md) for the full
transport × mode × platform matrix.

## CLI Interface

```bash
# Default: build for current host as binary
op build

# Cross-compile for a target platform
op build --target ios
op build --target android
op build --target wasm
op build --target windows    # cross-compile from macOS/Linux
```

The `--target` flag selects both the platform and the appropriate
execution mode (e.g., `--target ios` implies framework mode).

## Manifest: `build.targets`

Each holon declares how it builds for each platform:

```yaml
build:
  runner: go-module
  targets:
    default:                # op build (no --target)
      mode: binary
    ios:
      mode: framework
      tool: gomobile bind
      flags: [-target, ios]
    android:
      mode: framework
      tool: gomobile bind
      flags: [-target, android]
    wasm:
      mode: wasm
      env:
        GOOS: js
        GOARCH: wasm
```

If a target is not declared, `op build --target <x>` fails with a
clear error: `holon "foo" does not declare a build target for "ios"`.

## Per-Language Tooling

| Language | Framework (mobile) | WASM (browser) |
|---|---|---|
| **Go** | `gomobile bind` | `GOOS=js GOARCH=wasm` |
| **Rust** | `cargo build --target aarch64-apple-ios` | `wasm-pack build` |
| **C/C++** | NDK (Android), Xcode (iOS) | Emscripten |
| **Swift** | Xcode framework | SwiftWasm (experimental) |
| **Kotlin** | Gradle (Android native) | Kotlin/JS (limited) |
| **Dart** | Flutter | `dart compile js` |
| **C#** | .NET MAUI | Blazor WASM |

| **Node.js** | N/A (interpreted) | WASI / bundler |
| **Python** | N/A (interpreted) | Pyodide / CPython WASM |

## Runner Adaptation

Each runner learns how to cross-compile. The recipe runner
(`kind: composite`) passes `--target` to each member's runner:

```
op build --target ios
  → runner(greeting-daemon-go, target=ios)    → gomobile bind
  → runner(greeting-swiftui, target=ios)      → xcodebuild
  → assemble composite artifact
```

## Transport Selection

The connect chain adapts based on the execution mode of the
built artifact:

| Mode | Connect chain |
|---|---|
| binary | `mem → stdio → unix → tcp → rest+sse` |
| framework | `mem → tcp → rest+sse` |
| wasm | `mem → rest+sse` |

The SDK detects its own execution mode at startup and configures
the connect chain accordingly. No manual transport configuration
needed.

## Dependency

- **Requires v0.6** — REST+SSE transport must exist before
  framework/WASM targets make sense (they depend on HTTP transport)
- **Independent of v0.9** — Mesh is network topology, not build
  targeting
