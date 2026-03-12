# Megg FFmpeg v0.2 — Upstream Tasks

> Segmenter, alignment audio prep, silence detection, loudness normalization.
> Supports any pipeline that splits media and extracts audio for ASR.

## Execution Strategy

Mostly linear. TASK01 and TASK02 can run in parallel after v0.1 is done.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| | | **— Segmentation —** | |
| 01 | [TASK01](./MEGG_TASK001_segmenter.md) | Temporal segmentation with overlap | v0.1 |
| 02 | [TASK02](./MEGG_TASK002_silence_detect.md) | Silence detection for intelligent split points | v0.1 |
| 03 | [TASK03](./MEGG_TASK003_smart_segment.md) | Smart segmenter (silence-aware splits) | TASK01, TASK02 |
| | | **— Audio Prep —** | |
| 04 | [TASK04](./MEGG_TASK004_audio_alignment.md) | ASR-optimized audio extraction | v0.1 |
| 05 | [TASK05](./MEGG_TASK005_loudnorm.md) | EBU R128 loudness normalization | v0.1 |
| | | **— Format —** | |
| 06 | [TASK06](./MEGG_TASK006_convert_format.md) | Container conversion (remux without reencode) | v0.1 |
| 07 | [TASK07](./MEGG_TASK007_integration_tests.md) | End-to-end integration tests with real media | TASK01–06 |

## Dependency Graph

```
v0.1 → TASK01 → TASK03 → TASK07
     → TASK02 ──┘
     → TASK04 ──────────→ TASK07
     → TASK05 ──────────→ TASK07
     → TASK06 ──────────→ TASK07
```
