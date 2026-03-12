# MEGG TASK003 — Media Service Implementation

## Goal

Implement the 6 `media.v1.Media` RPCs as a C++ library (`media_service`).
Each RPC maps directly to FFmpeg's `libav*` C API with RAII wrappers for
resource management.

## Architecture

```
media_service.h     — public API (6 functions, one per RPC)
media_service.cpp   — implementation (libav* calls)
media_types.h       — shared request/response structs
```

The service is a stateless library — no global state, no caching, no threads.
Each function takes a request struct and returns a response struct or an error.

## RPC → libav* Mapping

| RPC | Primary API | Flow |
|-----|------------|------|
| `Probe` | avformat | `avformat_open_input` → `avformat_find_stream_info` → iterate streams → close |
| `ExtractAudio` | avcodec + swresample | Open → find audio stream → `avcodec_open2` → decode loop → `swr_convert` → write PCM |
| `ExtractSegment` | avformat | `av_seek_frame` → remux packets (or reencode if requested) → close |
| `TranscodeAudio` | avcodec + swresample | Decode → resample → encode to target codec → mux |
| `MuxSubtitles` | avformat | Open input + subtitle → copy all streams + add subtitle → remux |
| `GetVersion` | avutil | `av_version_info()` + `avformat_configuration()` + compile-time license |

## RAII Wrappers

All `libav*` resources must be wrapped in RAII helpers to prevent leaks.
Required wrappers:

```cpp
struct AVFormatContextDeleter {
    void operator()(AVFormatContext* ctx) const;
};
using FormatContextPtr = std::unique_ptr<AVFormatContext, AVFormatContextDeleter>;

// Similarly for:
// - AVCodecContext (avcodec_free_context)
// - AVFrame (av_frame_free)
// - AVPacket (av_packet_free)
// - SwrContext (swr_free)
// - SwsContext (sws_freeContext)
// - AVFilterGraph (avfilter_graph_free)
```

## Error Handling

- **Never throw.** Return `std::expected<Response, MediaError>` (C++23) or
  `std::variant<Response, MediaError>` (C++20 fallback).
- `MediaError` carries: error code, FFmpeg error string, context string.
- All FFmpeg return codes are checked — no silent failures.

## Key Implementation Notes

- Use `extern "C" { #include <libavformat/avformat.h> ... }` for C headers
- Pass `MEGG_LICENSE` as a compile definition for `GetVersion`
- `ExtractAudio` defaults: f32le, original sample rate, original channels
- `ExtractSegment` with `reencode=false`: stream copy (fast), with
  `reencode=true`: full decode→encode (accurate cuts)
- `MuxSubtitles` supports SRT, VTT, ASS input formats

## Acceptance Criteria

- [ ] All 6 RPCs implemented and callable
- [ ] RAII wrappers for all libav* resource types
- [ ] No raw `av_*_free()` calls outside RAII destructors
- [ ] `GetVersion` returns correct license mode (LGPL or GPL)
- [ ] `Probe` correctly reports all stream types
- [ ] `ExtractAudio` produces valid PCM at requested sample rate
- [ ] Graceful error on missing files, unsupported codecs, corrupt media

## Dependencies

- TASK01 (FFmpeg builds), TASK02 (proto defines the API shape)
