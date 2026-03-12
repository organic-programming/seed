# Megg FFmpeg v0.5 — Analysis Tasks

> Content-aware media analysis: scene detection, loudness measurement,
> spectrum analysis, keyframe extraction.

## Execution Strategy

All tasks depend on v0.3 (filter engine). TASK01–05 are independent of each
other and can run in parallel.

## Tasks

| # | File | Summary | Depends on |
|---|------|---------|------------|
| 01 | [TASK01](./MEGG_TASK001_scene_detect.md) | Scene change detection with confidence scores | v0.3 |
| 02 | [TASK02](./MEGG_TASK002_loudness.md) | EBU R128 integrated/short-term/momentary loudness | v0.3 |
| 03 | [TASK03](./MEGG_TASK003_spectrum.md) | Frequency-domain analysis (voice activity detection) | v0.3 |
| 04 | [TASK04](./MEGG_TASK004_keyframes.md) | I-frame extraction with timestamps | v0.3 |
| 05 | [TASK05](./MEGG_TASK005_black_detect.md) | Black frame and silence interval detection | v0.3 |
| 06 | [TASK06](./MEGG_TASK006_bitrate.md) | Per-second bitrate profile | v0.3 |
| 07 | [TASK07](./MEGG_TASK007_proto_analysis.md) | `media.v3.analysis` proto definitions | TASK01–06 |
| 08 | [TASK08](./MEGG_TASK008_tests.md) | Analysis accuracy tests + benchmarks | TASK01–07 |

## Dependency Graph

```
v0.3 → TASK01 ──┐
     → TASK02 ──┤
     → TASK03 ──┤→ TASK07 → TASK08
     → TASK04 ──┤
     → TASK05 ──┤
     → TASK06 ──┘
```
