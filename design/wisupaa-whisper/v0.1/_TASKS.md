# Wisupaa Whisper v0.1 — Bootstrap Tasks

> Build whisper.cpp from source, wire it to `cpp-holons` JSON-RPC, expose
> 4 core ASR RPCs. Model loaded at runtime. Audio input: WAV only.

## Execution Strategy

Linear — same pattern as megg-ffmpeg v0.1.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— Foundation —** | |
| 01 | [TASK01](./WISUPAA_TASK001_whisper_bootstrap.md) | Repo init, whisper.cpp submodule, CMake build | — |
| 02 | [TASK02](./WISUPAA_TASK002_proto_contract.md) | Proto contract: `whisper.v1.Whisper` (4 RPCs) | TASK01 |
| 03 | [TASK03](./WISUPAA_TASK003_whisper_service.md) | C++ service (Transcribe, DetectLanguage, etc.) | TASK02 |
| | | **— Integration —** | |
| 04 | [TASK04](./WISUPAA_TASK004_jsonrpc_cli.md) | JSON-RPC dispatcher + `cmd/wisupaa-whisper/main.cpp` | TASK03 |
| 05 | [TASK05](./WISUPAA_TASK005_tests.md) | Unit + integration tests (requires model file) | TASK03 |
| 06 | [TASK06](./WISUPAA_TASK006_holon_integration.md) | `holon.yaml`, submodule, `op build` | TASK04, TASK05 |

## Dependency Graph

```
TASK01 → TASK02 → TASK03 → TASK04 → TASK06
                         └→ TASK05 → ┘
```

## Model Fixture Strategy

Integration tests require a Whisper model. Options:

| Model | Size | WER | Test use |
|-------|------|-----|----------|
| `ggml-tiny.bin` | 75 MB | ~32% | ✅ Fast, sufficient for CI |
| `ggml-base.bin` | 142 MB | ~22% | ✅ Default for local dev |

Models are **not committed** — downloaded on first test run via script.
