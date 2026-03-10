# Grace-OP v0.7 Design Tasks — Cross-Compilation

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.7_TASK01_build_targets_schema.md) | `build.targets` manifest schema + `op check` | — | — |
| 02 | [TASK02](./grace-op_v0.7_TASK02_build_target_flag.md) | `op build --target` CLI flag | TASK01 | — |
| 03 | [TASK03](./grace-op_v0.7_TASK03_cross_compile_go.md) | Go: gomobile (iOS/Android) + WASM | TASK02 | — |
| 04 | [TASK04](./grace-op_v0.7_TASK04_cross_compile_rust.md) | Rust: cargo targets + wasm-pack | TASK02 | — |
| 05 | [TASK05](./grace-op_v0.7_TASK05_sdk_mode_detection.md) | All SDKs: auto-detect mode + transport chain | TASK03, v0.6 | — |
| 07 | [TASK07](./grace-op_v0.7_TASK07_cross_compile_cpp.md) | C/C++: NDK + Emscripten | TASK02 | — |
| 08 | [TASK08](./grace-op_v0.7_TASK08_cross_compile_swift.md) | Swift: Xcode + SwiftWasm | TASK02 | — |
| 09 | [TASK09](./grace-op_v0.7_TASK09_cross_compile_kotlin.md) | Kotlin: Gradle + Kotlin/JS | TASK02 | — |
| 10 | [TASK10](./grace-op_v0.7_TASK10_cross_compile_dart.md) | Dart: Flutter + dart compile js | TASK02 | — |
| 11 | [TASK11](./grace-op_v0.7_TASK11_cross_compile_dotnet.md) | C#: .NET MAUI + Blazor WASM | TASK02 | — |
| 12 | [TASK12](./grace-op_v0.7_TASK12_cross_compile_node.md) | Node.js: WASI / bundler (WASM only) | TASK02 | — |
| 13 | [TASK13](./grace-op_v0.7_TASK13_cross_compile_python.md) | Python: Pyodide (WASM only) | TASK02 | — |
| 14 | [TASK14](./grace-op_v0.7_TASK14_cross_compile_desktop.md) | Desktop cross-compile: windows / linux / macos | TASK02 | — |
| 15 | [TASK15](./grace-op_v0.7_TASK15_cross_compile_docs.md) | Documentation (spec updates → output/ for review) | TASK01–14 | — |

Design document: [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md)
