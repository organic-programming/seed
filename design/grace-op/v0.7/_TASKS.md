# Grace-OP v0.7 Design Tasks — Cross-Compilation

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.7_TASK01_build_targets_schema.md) | `build.targets` manifest schema + `op check` | — |
| 02 | [TASK02](./grace-op_v0.7_TASK02_build_target_flag.md) | `op build --target` CLI flag | TASK01 |
| 03 | [TASK03](./grace-op_v0.7_TASK03_cross_compile_go.md) | Go: gomobile (iOS/Android) + WASM | TASK02 |
| 04 | [TASK04](./grace-op_v0.7_TASK04_cross_compile_rust.md) | Rust: cargo targets + wasm-pack | TASK02 |
| 05 | [TASK05](./grace-op_v0.7_TASK05_sdk_mode_detection.md) | SDK auto-detect execution mode + transport chain | TASK03, v0.6 |
| 06 | [TASK06](./grace-op_v0.7_TASK06_cross_compile_docs.md) | Documentation (spec updates → output/ for review) | all |
| 07 | [TASK07](./grace-op_v0.7_TASK07_cross_compile_cpp.md) | C/C++: NDK + Emscripten | TASK02 |
| 08 | [TASK08](./grace-op_v0.7_TASK08_cross_compile_swift.md) | Swift: Xcode + SwiftWasm | TASK02 |
| 09 | [TASK09](./grace-op_v0.7_TASK09_cross_compile_kotlin.md) | Kotlin: Gradle + Kotlin/JS | TASK02 |
| 10 | [TASK10](./grace-op_v0.7_TASK10_cross_compile_dart.md) | Dart: Flutter + dart compile js | TASK02 |
| 11 | [TASK11](./grace-op_v0.7_TASK11_cross_compile_dotnet.md) | C#: .NET MAUI + Blazor WASM | TASK02 |

Design document: [DESIGN_cross_compilation.md](./DESIGN_cross_compilation.md)
