# Megg FFmpeg — Compatibility & Maintenance Strategy

> How Megg tracks FFmpeg releases and maintains full capability coverage.

---

## Principle: Full FFmpeg, Not a Subset

Megg does **not** cherry-pick FFmpeg features. Every codec, muxer, demuxer,
filter, and protocol that FFmpeg's current stable release supports is
available in Megg — gated only by the LGPL/GPL license boundary.

| Build mode | `op build` flag | What's enabled |
|------------|----------------|----------------|
| **LGPL** (default) | `--config lgpl` | All LGPL-compatible codecs, filters, muxers, protocols |
| **GPL** | `--config gpl` | Everything in LGPL + GPL-only codecs (x264, x265, etc.) |

The configure step uses `--enable-everything` (or an equivalent maximal set)
and disables only what the license mode forbids.

---

## FFmpeg Version Tracking Policy

### Submodule Pin Strategy

FFmpeg lives in `third_party/FFmpeg/` as a git submodule, pinned to a **stable
release tag**. The initial pin is `n8.0.1`. We do not track `master`.

### Update Cadence

| Event | Action |
|-------|--------|
| FFmpeg stable release (roughly every 6 months) | Update submodule pin within 2 weeks |
| FFmpeg security patch | Update submodule pin within 48 hours |
| Breaking API change in FFmpeg | Branch, adapt, test, then merge |

### The Update Procedure

```
1. Update the submodule pin
   cd third_party/FFmpeg && git fetch --tags && git checkout n<new> && cd ../..

2. Build
   cmake -B build && cmake --build build -j$(nproc)

3. Run the compatibility test suite (TASK005)
   ctest --test-dir build --output-on-failure

4. Check for deprecation warnings
   cmake --build build 2>&1 | grep -i deprecated

5. If any test fails → fix the wrapper code, do NOT patch FFmpeg

6. Update FFMPEG_VERSION in CMakeLists.txt
   set(MEGG_FFMPEG_VERSION "n8.0.1" CACHE STRING "Pinned FFmpeg version")

7. Commit: "chore: bump FFmpeg to n8.0.1"
```

---

## API Stability Contract

### Megg API ≠ FFmpeg API

Megg exposes **task-oriented RPCs** (Probe, ExtractAudio, etc.), not raw
`libav*` function calls. When FFmpeg changes its internal C API:

- **Megg's RPC contract stays stable** — callers never see the change.
- **The wrapper adapts** — `media_service.cpp` maps new FFmpeg API to the
  same request/response structs.

### Deprecation Handling

FFmpeg marks APIs deprecated before removing them (usually 2 major versions).
Megg's build system detects and adapts:

```cmake
include(CheckSymbolExists)
check_symbol_exists(avcodec_decode_audio4 "libavcodec/avcodec.h" HAS_LEGACY_DECODE)
if(HAS_LEGACY_DECODE)
    target_compile_definitions(media_service PRIVATE MEGG_HAS_LEGACY_DECODE)
endif()
```

In C++ code:
```cpp
#ifdef MEGG_HAS_LEGACY_DECODE
    // Use avcodec_decode_audio4 (deprecated but present)
#else
    // Use avcodec_send_packet / avcodec_receive_frame (current API)
#endif
```

---

## Full Capability Exposure Strategy

### Phase 1 (v0.1): Task-Oriented RPCs

High-level RPCs for common operations. These are stable and never change shape.

### Phase 2 (v0.2+): Pass-Through RPC

A generic `Execute` RPC that accepts **FFmpeg CLI-equivalent arguments** and
runs them through the `libav*` pipeline:

```protobuf
rpc Execute(ExecuteRequest) returns (ExecuteResponse);

message ExecuteRequest {
  repeated string args = 1;  // e.g. ["-i", "input.mp4", "-vf", "scale=1920:1080", "output.mp4"]
}

message ExecuteResponse {
  int32 exit_code = 1;
  string stdout = 2;
  string stderr = 3;
  string output_path = 4;
}
```

This ensures **anything FFmpeg can do, Megg can do** — without needing a
dedicated RPC for every operation.

### Phase 3 (v0.3+): Filter Graph RPC

Exposes `avfilter` directly. Any valid FFmpeg filter string works:

```protobuf
rpc ApplyFilterGraph(ApplyFilterGraphRequest) returns (ApplyFilterGraphResponse);
```

---

## What We Explicitly Do NOT Do

- **We do not patch FFmpeg.** Megg uses upstream FFmpeg unmodified.
- **We do not fork FFmpeg.** Submodule only.
- **We do not expose `libav*` headers** to Megg consumers. The proto
  contract is the only interface.
- **We do not limit codecs** beyond the LGPL/GPL boundary.

---

## Build-Time Feature Detection

On every build, the CMake system introspects the FFmpeg install and generates
a capability report:

```cmake
# Auto-detect what the linked FFmpeg supports
execute_process(
    COMMAND ${FFMPEG_INSTALL_DIR}/bin/ffmpeg -hide_banner -codecs
    OUTPUT_VARIABLE FFMPEG_CODECS)
execute_process(
    COMMAND ${FFMPEG_INSTALL_DIR}/bin/ffmpeg -hide_banner -filters
    OUTPUT_VARIABLE FFMPEG_FILTERS)
```

The `GetVersion` RPC returns this capability set at runtime, so callers can
discover what's available:

```json
{
  "version": "8.0.1",
  "license": "LGPL",
  "codecs": ["aac", "mp3", "opus", "flac", "pcm_f32le", ...],
  "filters": ["loudnorm", "silencedetect", "scale", ...],
  "muxers": ["mp4", "mkv", "wav", "flac", "ogg", ...],
  "protocols": ["file", "pipe", "tcp", "udp", ...]
}
```

---

## License Audit Checklist

Run before every release:

- [ ] `ffmpeg -version` shows correct license (LGPL or GPL)
- [ ] No GPL-only codec linked in LGPL mode
- [ ] `avformat_configuration()` output matches expected flags
- [ ] LGPL build tested: x264/x265 codecs are NOT available
- [ ] GPL build tested: x264/x265 codecs ARE available
- [ ] All third-party license files present in `third_party/LICENSES/`
