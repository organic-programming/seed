# Wisupaa Whisper — Design Documents

## Philosophy

Wisupaa makes `whisper.cpp` **fully actionable via RPC** in the Organic
Programming ecosystem. Precision ASR — transcription, alignment, language
detection — exposed as stateless RPCs.

## Root Documents

- [ROADMAP.md](./ROADMAP.md) — versioned milestones (v0.1 → v1.0)

## Tasks

- [v0.1/](./v0.1/_TASKS.md) — Bootstrap: 4 RPCs, CMake + whisper.cpp build (6 tasks)

## Sister Holon

Wisupaa pairs with [megg-ffmpeg](../megg_ffmpeg/INDEX.md) — megg handles
media processing (format conversion, audio extraction), wisupaa handles ASR.

See [CPP_HOLONS_STRATEGY.md](../cpp/CPP_HOLONS_STRATEGY.md) for the joint plan.
