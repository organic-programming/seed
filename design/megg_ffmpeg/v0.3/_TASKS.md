# Megg FFmpeg v0.3 — Filter Tasks

> avfilter graph engine, hardware acceleration, thumbnails, waveforms.

## Execution Strategy

TASK01 is the foundation — all other tasks depend on the filter graph engine.
TASK04 (HW accel) is independent and can run in parallel.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— Filter Engine —** | |
| 01 | [TASK01](./MEGG_TASK001_filter_graph.md) | Generic filter graph RPC | v0.2 |
| 02 | [TASK02](./MEGG_TASK002_thumbnail.md) | Thumbnail extraction (frame → PNG/JPEG) | TASK01 |
| 03 | [TASK03](./MEGG_TASK003_waveform.md) | Audio waveform image generation | TASK01 |
| | | **— Advanced —** | |
| 04 | [TASK04](./MEGG_TASK004_hwaccel.md) | Hardware acceleration (VideoToolbox, VAAPI, NVENC) | v0.2 |
| 05 | [TASK05](./MEGG_TASK005_crop_scale.md) | Crop detection + video scaling | TASK01 |
| 06 | [TASK06](./MEGG_TASK006_presets.md) | Safe filter preset library (named presets) | TASK01 |
| 07 | [TASK07](./MEGG_TASK007_tests.md) | Filter tests + HW accel benchmarks | TASK01–06 |

## Dependency Graph

```
v0.2 → TASK01 → TASK02 → TASK07
              → TASK03 → TASK07
              → TASK05 → TASK07
              → TASK06 → TASK07
     → TASK04 ─────────→ TASK07
```
