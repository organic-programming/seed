# Megg FFmpeg v0.1 — Bootstrap Tasks

> Build FFmpeg from source, wire it to `cpp-holons` JSON-RPC, expose 7
> RPCs (6 task-oriented + `Execute` pass-through). Everything file-based,
> stateless, LGPL by default. Full FFmpeg capability from day one.

## Execution Strategy

Linear — each task gates the next.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— Foundation —** | |
| 01 | [TASK01](./MEGG_TASK001_ffmpeg_bootstrap.md) | Repo init, FFmpeg submodule, CMake build system | — |
| 02 | [TASK02](./MEGG_TASK002_proto_contract.md) | Proto contract: `media.v1.Media` (6 RPCs) | TASK01 |
| 03 | [TASK03](./MEGG_TASK003_media_service.md) | C++ service implementation (Probe, ExtractAudio, etc.) | TASK02 |
| | | **— Integration —** | |
| 04 | [TASK04](./MEGG_TASK004_jsonrpc_cli.md) | JSON-RPC dispatcher + `cmd/megg-ffmpeg/main.cpp` | TASK03 |
| 05 | [TASK05](./MEGG_TASK005_tests.md) | Unit + integration tests | TASK03 |
| 06 | [TASK06](./MEGG_TASK006_holon_integration.md) | `holon.yaml`, submodule in videosteno, `op build` | TASK04, TASK05 |

## Dependency Graph

```
TASK01 → TASK02 → TASK03 → TASK04 → TASK06
                         └→ TASK05 → ┘
```
