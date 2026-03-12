# Megg FFmpeg — Design Documents

## Philosophy

Megg makes FFmpeg **fully actionable via RPC** in the Organic Programming
ecosystem. Every codec, filter, muxer, and protocol that FFmpeg supports is
available through Megg — gated only by the LGPL/GPL license boundary.

**Initial FFmpeg pin:** `n8.0.1`

## Root Documents

- [ROADMAP.md](./ROADMAP.md) — versioned milestones (v0.1 → v1.0)
- [COMPATIBILITY.md](./COMPATIBILITY.md) — FFmpeg version tracking, full
  coverage strategy, license audit

## Tasks

- [v0.1/](./v0.1/_TASKS.md) — Bootstrap: build system + 8 RPCs (6 task-oriented + Execute + ExecuteProbe)
- [v0.2/](./v0.2/_TASKS.md) — Upstream: segmenter, audio prep, silence detection (7 tasks)
- [v0.3/](./v0.3/_TASKS.md) — Filters: avfilter graph, thumbnails, waveforms, HW accel (7 tasks)
- [v0.4/](./v0.4/_TASKS.md) — Streaming: gRPC transport, live ingest, pipe transcode (7 tasks)
- [v0.5/](./v0.5/_TASKS.md) — Analysis: scene detect, loudness, spectrum, keyframes (8 tasks)
- [v0.6/](./v0.6/_TASKS.md) — Packaging: HLS, DASH, encoding ladder, burn-in subs (8 tasks)
- [v1.0/](./v1.0/_TASKS.md) — Production: stability, CI matrix, API freeze, license audit (9 tasks)

## Key Design Decisions

1. **Full Coverage:** Megg does not cherry-pick FFmpeg features. Everything
   FFmpeg can do is accessible via `Execute` + `ExecuteProbe` from v0.1.
2. **Task-Oriented First:** Common operations (Probe, Extract, Transcode)
   get dedicated RPCs with typed request/response. Pass-throughs are the
   universal fallback.
3. **LGPL Default:** `op build --config lgpl` (default) or `--config gpl`.
4. **FFmpeg 8.0.1 from Source:** Built as git submodule, no system dependency.
5. **Stateless RPCs:** Each call is independent — no sessions, no state.
