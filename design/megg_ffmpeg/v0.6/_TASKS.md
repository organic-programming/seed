# Megg FFmpeg v0.6 — Packaging Tasks

> Adaptive streaming output, encoding ladders, burn-in subtitles,
> DRM preparation.

## Execution Strategy

TASK01 (encoding ladder) is the foundation. HLS/DASH depend on it.
Burn-in subtitles is independent.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— Encoding —** | |
| 01 | [TASK01](./MEGG_TASK001_encoding_ladder.md) | Multi-bitrate encoding ladder from source | v0.4 |
| 02 | [TASK02](./MEGG_TASK002_preview_proxy.md) | Low-bitrate preview proxy generation | v0.4 |
| | | **— Adaptive Streaming —** | |
| 03 | [TASK03](./MEGG_TASK003_hls.md) | HLS output (master playlist + segments) | TASK01 |
| 04 | [TASK04](./MEGG_TASK004_dash.md) | MPEG-DASH MPD + segments | TASK01 |
| | | **— Subtitles & DRM —** | |
| 05 | [TASK05](./MEGG_TASK005_burn_in_subs.md) | Hardcode subtitles onto video (ASS/SRT/VTT) | v0.3 |
| 06 | [TASK06](./MEGG_TASK006_drm_prep.md) | DRM keyinfo / CPIX manifest preparation | TASK03, TASK04 |
| | | **— Proto & Tests —** | |
| 07 | [TASK07](./MEGG_TASK007_proto_packaging.md) | `media.v3.packaging` proto definitions | TASK01–06 |
| 08 | [TASK08](./MEGG_TASK008_tests.md) | Packaging validation + ABR playback tests | TASK01–07 |

## Dependency Graph

```
v0.4 → TASK01 → TASK03 → TASK06 → TASK07 → TASK08
              → TASK04 ──┘
     → TASK02 ─────────────────→ TASK07
v0.3 → TASK05 ─────────────────→ TASK07
```
