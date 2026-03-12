# Megg FFmpeg — Codex Implementation Strategy

> Analysis of what Codex needs to implement megg-ffmpeg v0.1, plus
> recommendations for prompt structure and supporting materials.

---

## Executive Summary

Megg v0.1 is a **feasible Codex target** but needs careful preparation.
The main challenges are:

1. **FFmpeg build from source** — CMake `ExternalProject_Add` is complex
2. **C API ↔ C++ bridge** — RAII wrappers for every `libav*` type
3. **No gRPC needed** — v0.1 uses JSON-RPC over stdio (simpler than whisper)
4. **Execute pass-through** — requires parsing FFmpeg CLI args programmatically

---

## Reference Architecture: wisupaa-whisper

The wisupaa-whisper holon is the closest existing pattern. Megg should
replicate its structure:

```
wisupaa-whisper/
├── holon.yaml                    ← identical structure
├── HOLON.md                      ← motto, identity
├── protos/whisper/v1/whisper.proto
├── third_party/whisper.cpp/      ← git submodule (→ FFmpeg for megg)
├── CMakeLists.txt                ← builds third_party + service + CLI
├── src/
│   ├── whisper_service.h
│   ├── whisper_service.cpp
│   └── ...
├── cmd/wisupaa-whisper/main.cpp  ← JSON-RPC dispatcher
└── tests/
    └── test_whisper_service.cpp
```

### Key Difference: Transport

- wisupaa-whisper → gRPC via `holons::serve::serve()` (uses `serve.hpp`)
- megg-ffmpeg v0.1 → **JSON-RPC over stdio** (uses `holons.hpp` directly)

This is actually **simpler** — Codex doesn't need protoc-generated stubs.
The JSON-RPC dispatcher parses `nlohmann::json` and dispatches manually.

---

## What Codex Needs to Succeed

### 1. Pre-Built Supporting Files (we create, not Codex)

These files require domain knowledge Codex won't have:

| File | Why we create it | Codex risk if we don't |
|------|-----------------|----------------------|
| `holon.yaml` | OP-specific manifest format | Codex will invent wrong schema |
| `HOLON.md` | Identity/motto conventions | Codex will write generic README |
| `protos/media/v1/media.proto` | Full proto with all messages | Codex will miss field semantics |
| `CMakeLists.txt` (skeleton) | FFmpeg ExternalProject is tricky | Codex will likely get build wrong |
| `tests/fixtures/` | Test media files via `ffmpeg` CLI | Codex cannot generate binary files |

### 2. Codex-Generated Files

These are the core implementation Codex is well-suited for:

| File | Codex difficulty | Notes |
|------|-----------------|-------|
| `src/media_types.h` | Easy | Struct definitions matching proto |
| `src/raii_wrappers.h` | Medium | RAII for AVFormatContext, AVCodecContext, etc. |
| `src/media_service.h` | Easy | Function declarations |
| `src/media_service.cpp` | **Hard** | Core implementation — libav* API calls |
| `src/execute.h/cpp` | **Hard** | FFmpeg CLI arg parser + libav* mapping |
| `cmd/megg-ffmpeg/main.cpp` | Medium | JSON-RPC dispatcher loop |
| `tests/test_media_service.cpp` | Medium | Assumes fixtures exist |

### 3. Context Files to Include in Prompt

Codex performs best when given concrete reference code. Include:

| Context | Path | Why |
|---------|------|-----|
| cpp-holons echo server | `sdk/cpp-holons/examples/echo_server.cpp` | Shows the serve pattern |
| cpp-holons holons.hpp | `sdk/cpp-holons/include/holons/holons.hpp` (lines 648–800) | JSON-RPC client/dispatch |
| Proto contract | `design/megg_ffmpeg/v0.1/MEGG_TASK002_proto_contract.md` | Message shapes |
| Media service design | `design/megg_ffmpeg/v0.1/MEGG_TASK003_media_service.md` | RAII patterns, error model |

---

## FFmpeg 8.0.1 API Notes for Codex

Critical API facts Codex must know:

### Mandatory Modern APIs (old ones removed)

```cpp
// ✅ CORRECT (FFmpeg 8.0+)
AVChannelLayout layout = AV_CHANNEL_LAYOUT_STEREO;
av_channel_layout_copy(&layout, &codec_ctx->ch_layout);

// ❌ REMOVED (was deprecated since FFmpeg 5.1)
// codec_ctx->channels = 2;
// codec_ctx->channel_layout = AV_CH_LAYOUT_STEREO;
```

### Decode/Encode Pattern

```cpp
// ✅ CORRECT — send/receive API (since FFmpeg 3.1, mandatory since 7.0)
avcodec_send_packet(codec_ctx, packet);
avcodec_receive_frame(codec_ctx, frame);

// ❌ REMOVED
// avcodec_decode_audio4(codec_ctx, frame, &got_frame, packet);
```

### C11 Requirement

FFmpeg 8.0 requires a C11 compiler. CMake must set:
```cmake
set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
```

### Key New Features Available

- **Vulkan compute codecs** (FFv1, ProRes RAW) — available via `avcodec`
- **Whisper filter** (`avfilter`) — built-in ASR for live subtitles
- **WHIP muxer** — sub-second latency WebRTC streaming
- **AV1 RTP** — packetizer/depacketizer

---

## Recommended Prompt Strategy

### Don't: One Monolithic Prompt

A single prompt asking Codex to build everything will fail because:
- FFmpeg ExternalProject is finicky
- The Execute RPC requires deep ffmpeg CLI knowledge
- Test fixtures need to exist before tests run

### Do: Phased Prompts

| Prompt | Scope | Depends on |
|--------|-------|------------|
| **P0** (us) | Bootstrap files: `holon.yaml`, `HOLON.md`, proto, CMakeLists skeleton, fixtures | — |
| **P1** | RAII wrappers + `media_types.h` + `Probe` RPC only | P0 |
| **P2** | Remaining 5 task-oriented RPCs | P1 |
| **P3** | `Execute` + `ExecuteProbe` pass-throughs | P2 |
| **P4** | JSON-RPC dispatcher (`main.cpp`) | P1 |
| **P5** | Tests | P2, P4 |

### Prompt P0 — What We Build Before Codex

```bash
# 1. Create repo structure
mkdir -p holons/megg-ffmpeg/{src,cmd/megg-ffmpeg,tests/fixtures,protos/media/v1}

# 2. Add FFmpeg submodule
cd holons/megg-ffmpeg
git submodule add https://github.com/FFmpeg/FFmpeg.git third_party/FFmpeg
cd third_party/FFmpeg && git checkout n8.0.1 && cd ../..

# 3. Generate test fixtures
ffmpeg -f lavfi -i "sine=frequency=440:duration=1" -ac 2 -ar 44100 \
  tests/fixtures/test_1s_stereo.wav
ffmpeg -f lavfi -i "testsrc2=duration=5:size=1280x720:rate=30" \
  -f lavfi -i "sine=frequency=440:duration=5" \
  -c:v libx264 -preset ultrafast -c:a aac \
  tests/fixtures/test_5s_video.mp4
echo -e "1\n00:00:00,000 --> 00:00:02,000\nHello World\n" \
  > tests/fixtures/test_subtitle.srt
head -c 1024 tests/fixtures/test_5s_video.mp4 \
  > tests/fixtures/test_corrupt.mp4

# 4. Write proto, holon.yaml, HOLON.md, CMakeLists.txt
# (these require our domain knowledge)
```

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Codex uses deprecated FFmpeg APIs | High | Include FFmpeg 8.0 API notes in every prompt |
| CMake ExternalProject fails | High | We write CMakeLists.txt ourselves (P0) |
| Execute RPC too complex for one prompt | Medium | Split into P3, accept it may need iteration |
| JSON serialization mismatches | Low | Include exact struct definitions in prompt |
| Test fixtures missing | High | We generate them in P0 |
| Codex hallucinate gRPC pattern instead of JSON-RPC | Medium | Explicitly state "no gRPC in v0.1" |

---

## Recommendation

**We should do P0 manually** (or in this session) before sending anything to
Codex. This means creating:

1. `holons/megg-ffmpeg/holon.yaml`
2. `holons/megg-ffmpeg/HOLON.md`
3. `holons/megg-ffmpeg/protos/media/v1/media.proto`
4. `holons/megg-ffmpeg/CMakeLists.txt` (with FFmpeg ExternalProject)
5. Test fixture generation script
6. Codex prompts P1–P5

This gives Codex a working build system and a complete proto contract to
implement against. It's the difference between "implement these RPCs" (good)
and "create a C++ project from scratch that builds FFmpeg" (destined to fail).
