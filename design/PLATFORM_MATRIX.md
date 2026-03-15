# Platform Matrix

How Organic Programming SDKs, transports, and platforms interact.

---

## Execution Modes

A holon can run in three modes. **The mode determines which
transports are available**, not the language.

| Mode | Where | How it starts |
|---|---|---|
| **Binary** | Desktop, server | `op run` spawns a standalone process |
| **Framework** | Mobile (iOS, Android) | Embedded as a library in a host app |
| **WASM** | Browser, edge runtimes | Loaded as a WebAssembly module |

---

## Transports by Execution Mode

| Transport | Binary | Framework | WASM |
|---|---|---|---|
| `mem://` | ✅ | ✅ | ✅ |
| `stdio://` | ✅ | ❌ no process | ❌ no process |
| `unix://` | ✅ (POSIX) | ❌ sandboxed | ❌ no sockets |
| `tcp://` | ✅ | ✅ | ❌ no sockets |
<!--- @bpds pourquoi ne pas mettre  . ws, wss c'est très important -->  
| `rest+sse://` (v0.4) | ✅ | ✅ | ✅ |

**Key insight:** `mem://` and `rest+sse://` are the only universal
transports. Everything else depends on OS capabilities and
sandboxing constraints.

---

## Connect Chain by Mode + Platform

The SDK selects the best transport automatically, trying each in
order until one succeeds.

| Platform | Mode | Connect chain |
|---|---|---|
| macOS | binary | `mem → stdio → unix → tcp → rest+sse` |
| Linux | binary | `mem → stdio → unix → tcp → rest+sse` |
| Windows | binary | `mem → stdio → tcp → rest+sse` |
| iOS | framework | `mem → tcp → rest+sse` |
| Android | framework | `mem → tcp → rest+sse` |
| Browser | WASM | `mem → rest+sse` |

---

## Language × Execution Mode

Almost every language can target all three modes. The build
tooling differs, but the resulting holon is functionally identical.

| Language | Binary | Framework | WASM |
|---|---|---|---|
| **Go** | `go build` | `gomobile` (iOS, Android) | `GOOS=js GOARCH=wasm` |
| **Rust** | `cargo build` | `cdylib` target | `wasm-pack` |
| **C/C++** | `cmake` / `make` | `.so` / `.a` / `.framework` | Emscripten |
| **Swift** | `swift build` | Xcode framework (iOS) | SwiftWasm (experimental) |
| **Kotlin** | Gradle (JVM) | Android native | Kotlin/JS (limited) |
| **Dart** | `dart compile` | Flutter (iOS, Android) | `dart compile js` |
| **C#** | `dotnet build` | .NET MAUI (iOS, Android) | Blazor WASM |
| **Python** | `python` / PyInstaller | ❌ | Pyodide |
| **Java** | Gradle (JVM) | Android native | ❌ |
| **Node.js** | `node` | ❌ | N/A (already JS) |
| **JS Web** | N/A | N/A | Native |
| **Ruby** | `ruby` | ❌ | ruby.wasm (experimental) |

---

## Language × Platform (where holons can run)

Combining execution modes, here is where each language can
produce a running holon:

| Language | macOS | Linux | Windows | Android | iOS | Browser |
|---|---|---|---|---|---|---|
| **Go** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Rust** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **C/C++** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Swift** | ✅ | ✅ | ❌ | ❌ | ✅ | ⚠️ |
| **Kotlin** | ✅ | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| **Dart** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **C#** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Python** | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
| **Java** | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| **Node.js** | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| **JS Web** | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **Ruby** | ✅ | ✅ | ✅ | ❌ | ❌ | ⚠️ |

⚠️ = experimental / limited support

---

## Implications for Recipes

An assembly like `go-swift` (Go daemon + SwiftUI HostUI) works
differently depending on the platform:

| Platform | Go daemon mode | SwiftUI mode | Transport |
|---|---|---|---|
| macOS | binary | binary | `stdio` or `unix` |
| iOS | framework (gomobile) | native app | `mem` or `tcp` |

The **assembly manifest** (`holon.yaml`) doesn't change — the
**runner** adapts the build and transport automatically based on
the target platform.

---

## Summary

1. **Execution mode determines transports**, not language
2. **`mem` and `rest+sse` are universal** — they work everywhere
3. **Most languages target all platforms** via binary, framework, or WASM
4. **The connect chain is platform-aware** — the SDK picks the
   best available transport automatically
