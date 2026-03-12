# Megg FFmpeg v0.4 — Streaming Tasks

> Network protocols, gRPC transport, bidirectional media streaming.

## Execution Strategy

Linear — gRPC transport (TASK01) gates the streaming RPCs.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— Transport —** | |
| 01 | [TASK01](./MEGG_TASK001_grpc_transport.md) | Add gRPC transport (alongside JSON-RPC/stdio) | v0.3 |
| 02 | [TASK02](./MEGG_TASK002_proto_streaming.md) | Streaming proto definitions (bidirectional) | TASK01 |
| | | **— Network —** | |
| 03 | [TASK03](./MEGG_TASK003_ingest_stream.md) | Live stream ingest (RTMP, SRT, RTSP → file/chunks) | TASK01 |
| 04 | [TASK04](./MEGG_TASK004_stream_probe.md) | Probe live URLs without full download | v0.3 |
| 05 | [TASK05](./MEGG_TASK005_pipe_transcode.md) | Bidirectional streaming transcode (stdin→stdout) | TASK02 |
| | | **— Infrastructure —** | |
| 06 | [TASK06](./MEGG_TASK006_security.md) | Protocol whitelist + input validation | TASK03 |
| 07 | [TASK07](./MEGG_TASK007_tests.md) | Streaming integration tests | TASK01–06 |

## Dependency Graph

```
v0.3 → TASK01 → TASK02 → TASK05 → TASK07
              → TASK03 → TASK06 → TASK07
     → TASK04 ─────────────────→ TASK07
```
