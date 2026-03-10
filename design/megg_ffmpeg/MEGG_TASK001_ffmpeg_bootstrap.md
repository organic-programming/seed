# Codex Prompt — Bootstrap `megg-ffmpeg` Holon

## Goal

Initialize `organic-programming/megg-ffmpeg` as a **complete C++ holon** that wraps **FFmpeg** (as a git submodule) and exposes task-oriented media processing RPCs over JSON-RPC via the `cpp-holons` SDK.

Repository: `git@github.com:organic-programming/megg-ffmpeg.git`

FFmpeg must be compilable in **LGPL mode** (default) or **GPL mode** (opt-in) via the `OP_CONFIG` build config mechanism (see `OP.md` §4 Build configs).

---

## Reference: Organic Programming C++ Conventions

| Aspect | Convention |
|--------|-----------|
| Source | `src/` |
| Generated | `gen/cpp/` |
| Tests | `tests/` |
| Manifest | `CMakeLists.txt` |
| Build | `cmake --build .` |

Rules: `HOLON.md` at root, `protos/<pkg>/<ver>/`, `gen/` committed, `cmd/` for CLI, `.gitignore` for build artifacts.

---

## Reference: `cpp-holons` SDK API

The SDK is **header-only** at relative path `../../organic-programming/sdk/cpp-holons/include` (when installed at `holons/megg-ffmpeg/` in the monorepo).

```cpp
namespace holons {
  std::string parse_flags(const std::vector<std::string>& args);

  struct HolonIdentity { std::string uuid, given_name, family_name, motto, composer, clade, status, born, lang; };
  HolonIdentity parse_holon(const std::string& path);

  using listener = std::variant<tcp_listener, unix_listener, stdio_listener, mem_listener, ws_listener>;
  struct connection { int read_fd, write_fd; std::string scheme; };

  listener    listen(const std::string& uri);
  connection  accept(listener& lis);
  ssize_t     conn_read(const connection& conn, void* buf, size_t n);
  ssize_t     conn_write(const connection& conn, const void* buf, size_t n);
  void        close_connection(connection& conn);
  void        close_listener(listener& lis);
}
```

The SDK is cross-platform (macOS, Linux, Windows). Uses `nlohmann/json.hpp`.

---

## Step 1 — Init repo + FFmpeg submodule

```bash
git init
git remote add origin git@github.com:organic-programming/megg-ffmpeg.git

# FFmpeg source — pinned to latest stable release
git submodule add https://github.com/FFmpeg/FFmpeg.git third_party/FFmpeg
cd third_party/FFmpeg && git checkout <latest-stable-tag> && cd ../..
```

---

## Step 2 — Directory structure

```
megg-ffmpeg/
├── HOLON.md
├── .gitignore
├── CMakeLists.txt
├── protos/
│   └── media/v1/
│       └── media.proto
├── gen/cpp/                           ← empty for now
├── src/
│   ├── media_service.h
│   └── media_service.cpp
├── cmd/
│   └── megg-ffmpeg/
│       └── main.cpp
├── tests/
│   └── test_media_service.cpp
└── third_party/
    └── FFmpeg/                        ← submodule
```

---

## Step 3 — `HOLON.md`

```markdown
---
# Holon Identity v1
uuid: "a2b3c4d5-6e7f-8a9b-0c1d-2e3f4a5b6c7d"
given_name: "Megg"
family_name: "FFmpeg"
motto: "Transform any stream."
composer: "B. ALTER"
clade: "deterministic/perceptual"
status: draft
born: "2026-03-02"

# Lineage
parents: []
reproduction: "assisted"

# Optional
aliases: []

# Metadata
generated_by: "codex"
lang: "cpp"
proto_status: defined
---

# Megg FFmpeg

> *"Transform any stream."*

## Description

General-purpose media processing holon. Wraps FFmpeg's libav* libraries
and exposes task-oriented RPCs for probing, audio extraction, transcoding,
and subtitle muxing over JSON-RPC.

Built from FFmpeg source (git submodule) with LGPL (default) or GPL mode.

This is a **deterministic/perceptual** holon — same input always produces
the same output.

## Contract

- Proto: `protos/media/v1/media.proto`
- Service: `media.v1.Media`
- Transport: `stdio://` (default), `tcp://`, `unix://`

## Introspection Notes

- FFmpeg lives in `third_party/FFmpeg/` (git submodule).
- The `cpp-holons` SDK is in the parent monorepo: `../../organic-programming/sdk/cpp-holons/`.
- Build with `op build --config lgpl` (default) or `op build --config gpl`.
- LGPL mode disables GPL-only codecs (x264, x265, etc.).
```

---

## Step 4 — Proto contract: `protos/media/v1/media.proto`

```protobuf
syntax = "proto3";
package media.v1;

service Media {
  // Probe a media file: format, duration, streams, codecs.
  rpc Probe(ProbeRequest) returns (ProbeResponse);

  // Extract audio from a media file as raw PCM.
  rpc ExtractAudio(ExtractAudioRequest) returns (ExtractAudioResponse);

  // Extract a time segment from a media file.
  rpc ExtractSegment(ExtractSegmentRequest) returns (ExtractSegmentResponse);

  // Transcode audio between formats.
  rpc TranscodeAudio(TranscodeAudioRequest) returns (TranscodeAudioResponse);

  // Mux subtitles into a media container.
  rpc MuxSubtitles(MuxSubtitlesRequest) returns (MuxSubtitlesResponse);

  // Return FFmpeg version, build config, and license mode.
  rpc GetVersion(GetVersionRequest) returns (GetVersionResponse);
}

// --- Probe ---

message ProbeRequest {
  string input_path = 1;          // Path to media file
}

message ProbeResponse {
  string format = 1;              // Container format (e.g. "matroska", "mp4")
  double duration_s = 2;          // Duration in seconds
  int64 bitrate = 3;              // Overall bitrate (bps)
  repeated StreamInfo streams = 4;
}

message StreamInfo {
  int32 index = 1;
  string type = 2;               // "audio", "video", "subtitle", "data"
  string codec = 3;              // Codec name (e.g. "aac", "h264")
  int32 sample_rate = 4;         // Audio only
  int32 channels = 5;            // Audio only
  int32 width = 6;               // Video only
  int32 height = 7;              // Video only
  double frame_rate = 8;         // Video only
  string language = 9;           // BCP-47 if available
}

// --- ExtractAudio ---

message ExtractAudioRequest {
  string input_path = 1;
  int32 stream_index = 2;        // Audio stream index (-1 = first audio)
  int32 sample_rate = 3;         // Target sample rate (0 = original)
  int32 channels = 4;            // Target channels (0 = original, 1 = mono)
  string sample_format = 5;      // "f32le", "s16le", "f64le" (default: "f32le")
  double start_s = 6;            // Start time in seconds (0 = beginning)
  double duration_s = 7;         // Duration in seconds (0 = until end)
  string output_path = 8;        // Output file path (raw PCM)
}

message ExtractAudioResponse {
  int64 n_samples = 1;
  int32 sample_rate = 2;
  int32 channels = 3;
  string sample_format = 4;
  string output_path = 5;
}

// --- ExtractSegment ---

message ExtractSegmentRequest {
  string input_path = 1;
  double start_s = 2;
  double duration_s = 3;
  string output_path = 4;        // Output file (same format as input)
  bool reencode = 5;             // If true, reencode; if false, stream copy
}

message ExtractSegmentResponse {
  string output_path = 1;
  double actual_start_s = 2;
  double actual_duration_s = 3;
}

// --- TranscodeAudio ---

message TranscodeAudioRequest {
  string input_path = 1;
  string output_path = 2;
  string codec = 3;              // Target codec (e.g. "aac", "opus", "pcm_f32le")
  int32 sample_rate = 4;         // Target sample rate (0 = original)
  int32 channels = 5;            // Target channels (0 = original)
  int32 bitrate = 6;             // Target bitrate bps (0 = default)
}

message TranscodeAudioResponse {
  string output_path = 1;
  double duration_s = 2;
}

// --- MuxSubtitles ---

message MuxSubtitlesRequest {
  string input_path = 1;         // Media file
  string subtitle_path = 2;      // Subtitle file (.srt, .vtt, .ass)
  string output_path = 3;        // Output file
  string language = 4;           // Subtitle language (BCP-47)
  bool default_track = 5;        // Set as default subtitle track
}

message MuxSubtitlesResponse {
  string output_path = 1;
}

// --- GetVersion ---

message GetVersionRequest {}

message GetVersionResponse {
  string version = 1;            // FFmpeg version string
  string configuration = 2;      // Build configuration flags
  string license = 3;            // "LGPL" or "GPL"
}
```

---

## Step 5 — `CMakeLists.txt`

This is the most complex part. FFmpeg uses its own `configure` + `make` system, not CMake.

```cmake
cmake_minimum_required(VERSION 3.14)
project(megg-ffmpeg LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# License mode: driven by OP_CONFIG (lgpl or gpl)
set(MEGG_LICENSE "LGPL" CACHE STRING "FFmpeg license mode")
if(DEFINED OP_CONFIG AND OP_CONFIG STREQUAL "gpl")
    set(MEGG_LICENSE "GPL")
endif()

# cpp-holons SDK
set(CPP_HOLONS_INCLUDE_DIR ${CMAKE_CURRENT_SOURCE_DIR}/../../organic-programming/sdk/cpp-holons/include)
if (NOT EXISTS ${CPP_HOLONS_INCLUDE_DIR}/holons/holons.hpp)
    message(FATAL_ERROR "cpp-holons SDK not found at ${CPP_HOLONS_INCLUDE_DIR}")
endif()

# nlohmann/json
find_path(NLOHMANN_JSON_INCLUDE_DIR nlohmann/json.hpp
    HINTS /opt/homebrew/include /usr/local/include
    PATHS ${CMAKE_CURRENT_SOURCE_DIR}/third_party)

# --- Build FFmpeg from source via ExternalProject ---
include(ExternalProject)

set(FFMPEG_SOURCE_DIR ${CMAKE_CURRENT_SOURCE_DIR}/third_party/FFmpeg)
set(FFMPEG_INSTALL_DIR ${CMAKE_CURRENT_BINARY_DIR}/ffmpeg-install)

# Base configure flags (minimal, LGPL-safe)
set(FFMPEG_CONFIGURE_FLAGS
    --prefix=${FFMPEG_INSTALL_DIR}
    --disable-programs        # No ffmpeg/ffprobe binaries
    --disable-doc
    --disable-network
    --enable-static
    --disable-shared
    --enable-pic
    --disable-debug
    --disable-autodetect      # Explicit control over dependencies
    --enable-swresample
    --enable-avformat
    --enable-avcodec
    --enable-avutil
    --enable-avfilter
    --enable-swscale
)

# GPL mode: add GPL flag + optional GPL codecs
if(MEGG_LICENSE STREQUAL "GPL")
    list(APPEND FFMPEG_CONFIGURE_FLAGS --enable-gpl)
    # Enable GPL-only codecs if available on the system
    # --enable-libx264 --enable-libx265 (commented — require external libs)
endif()

# Platform-specific flags
if(WIN32)
    list(APPEND FFMPEG_CONFIGURE_FLAGS
        --toolchain=msvc
        --target-os=win64
    )
elseif(APPLE)
    list(APPEND FFMPEG_CONFIGURE_FLAGS
        --enable-videotoolbox
        --enable-coreimage
    )
endif()

# Build FFmpeg
ExternalProject_Add(ffmpeg_build
    SOURCE_DIR ${FFMPEG_SOURCE_DIR}
    CONFIGURE_COMMAND ${FFMPEG_SOURCE_DIR}/configure ${FFMPEG_CONFIGURE_FLAGS}
    BUILD_COMMAND make -j4
    INSTALL_COMMAND make install
    BUILD_IN_SOURCE OFF
    BINARY_DIR ${CMAKE_CURRENT_BINARY_DIR}/ffmpeg-build
)

# Import FFmpeg libraries after build
set(FFMPEG_INCLUDE_DIR ${FFMPEG_INSTALL_DIR}/include)
set(FFMPEG_LIB_DIR ${FFMPEG_INSTALL_DIR}/lib)

# Helper to import each libav* as a static library
function(import_ffmpeg_lib name)
    add_library(${name} STATIC IMPORTED)
    set_target_properties(${name} PROPERTIES
        IMPORTED_LOCATION ${FFMPEG_LIB_DIR}/lib${name}.a)
    add_dependencies(${name} ffmpeg_build)
endfunction()

import_ffmpeg_lib(avformat)
import_ffmpeg_lib(avcodec)
import_ffmpeg_lib(avutil)
import_ffmpeg_lib(swresample)
import_ffmpeg_lib(swscale)
import_ffmpeg_lib(avfilter)

# Platform system libs (FFmpeg deps)
if(APPLE)
    find_library(VIDEOTOOLBOX VideoToolbox)
    find_library(COREMEDIA CoreMedia)
    find_library(COREVIDEO CoreVideo)
    find_library(COREFOUNDATION CoreFoundation)
    find_library(SECURITY Security)
    find_library(AUDIOTOOLBOX AudioToolbox)
    find_library(IOKIT IOKit)
    set(PLATFORM_LIBS ${VIDEOTOOLBOX} ${COREMEDIA} ${COREVIDEO} ${COREFOUNDATION}
                      ${SECURITY} ${AUDIOTOOLBOX} ${IOKIT} z bz2 iconv)
elseif(UNIX)
    set(PLATFORM_LIBS z m pthread)
elseif(WIN32)
    set(PLATFORM_LIBS ws2_32 bcrypt)
endif()

# Service library
add_library(media_service src/media_service.cpp)
target_include_directories(media_service PUBLIC src/)
target_include_directories(media_service PRIVATE ${FFMPEG_INCLUDE_DIR})
target_link_libraries(media_service PRIVATE
    avformat avcodec swresample swscale avfilter avutil
    ${PLATFORM_LIBS})
add_dependencies(media_service ffmpeg_build)

# CLI executable
add_executable(megg-ffmpeg cmd/megg-ffmpeg/main.cpp)
target_include_directories(megg-ffmpeg PRIVATE ${CPP_HOLONS_INCLUDE_DIR})
target_include_directories(megg-ffmpeg PRIVATE ${FFMPEG_INCLUDE_DIR})
if(NLOHMANN_JSON_INCLUDE_DIR)
    target_include_directories(megg-ffmpeg PRIVATE ${NLOHMANN_JSON_INCLUDE_DIR})
endif()
target_link_libraries(megg-ffmpeg PRIVATE media_service
    avformat avcodec swresample swscale avfilter avutil
    ${PLATFORM_LIBS})
if(WIN32)
    target_link_libraries(megg-ffmpeg PRIVATE ws2_32)
endif()

# Tests
add_executable(test_media_service tests/test_media_service.cpp)
target_include_directories(test_media_service PRIVATE ${CPP_HOLONS_INCLUDE_DIR})
target_include_directories(test_media_service PRIVATE ${FFMPEG_INCLUDE_DIR})
if(NLOHMANN_JSON_INCLUDE_DIR)
    target_include_directories(test_media_service PRIVATE ${NLOHMANN_JSON_INCLUDE_DIR})
endif()
target_link_libraries(test_media_service PRIVATE media_service
    avformat avcodec swresample swscale avfilter avutil
    ${PLATFORM_LIBS})
```

---

## Step 6 — `src/media_service.h` + `src/media_service.cpp`

C++ wrapper mapping each RPC to the FFmpeg `libav*` C API.

**Key patterns:**
- Use `extern "C" { #include <libavformat/avformat.h> ... }` for C headers in C++ code.
- `Probe`: `avformat_open_input()` → `avformat_find_stream_info()` → iterate streams → `avformat_close_input()`.
- `ExtractAudio`: open input → find audio stream → `avcodec_open2()` → decode packets → `swr_convert()` (resample to target format) → write raw PCM to output file.
- `ExtractSegment`: `av_seek_frame()` → remux or reencode packets to output.
- `TranscodeAudio`: decode → resample → encode → mux to output.
- `MuxSubtitles`: open input + subtitle → copy all streams + add subtitle stream → remux.
- `GetVersion`: `av_version_info()` + `avformat_configuration()` + return MEGG_LICENSE.

Pass `MEGG_LICENSE` (derived from `OP_CONFIG`) as a compile definition:
```cmake
target_compile_definitions(media_service PRIVATE MEGG_LICENSE="${MEGG_LICENSE}")
```

---

## Step 7 — `cmd/megg-ffmpeg/main.cpp`

Same pattern as wisupaa-whisper:
1. `holons::parse_holon("HOLON.md")` → print motto to stderr.
2. `holons::parse_flags(args)` → default `stdio://`.
3. `holons::listen(uri)` → `accept()` → JSON-RPC loop.
4. Dispatch: `"probe"`, `"extract_audio"`, `"extract_segment"`, `"transcode_audio"`, `"mux_subtitles"`, `"get_version"`.
5. File paths in requests/responses — the holon works with local filesystem paths.

---

## Step 8 — `tests/test_media_service.cpp`

1. Struct construction tests.
2. `get_version()` returns valid FFmpeg version + license string.
3. `probe()` on a non-existent file returns an error gracefully.
4. All RPCs handle empty/invalid input without crashing.

---

## Step 9 — `.gitignore`

```gitignore
# OS
.DS_Store
Thumbs.db

# IDE
.idea/
.vscode/
*.swp
*~

# C++ build artifacts
build/
*.o
*.a
*.so
*.dylib
*.lib
*.obj

# Binaries
/megg-ffmpeg
/test_media_service
```

---

## Step 10 — Post-Codex: add as submodule in videosteno

```bash
cd <videosteno-root>
git submodule add git@github.com:organic-programming/megg-ffmpeg.git holons/megg-ffmpeg
```

---

## Platform Notes

| Platform | FFmpeg build | Notes |
|----------|-------------|-------|
| **macOS** | `./configure && make` | VideoToolbox/Accelerate auto-detected |
| **Linux** | `./configure && make` | Needs `nasm`/`yasm` for x86 assembly |
| **Windows** | MSYS2/MinGW or `--toolchain=msvc` | Needs MSYS2 environment for configure |

---

## Rules

- **Do NOT reformat code you did not write.**
- **Comments explain _why_, not _what_.**
- **DRY** — no duplication.
- `gen/cpp/` stays empty.
- FFmpeg is built from source via `ExternalProject_Add` — no system FFmpeg dependency.
- Default config is `lgpl`. Use `op build --config gpl` to enable GPL codecs.
- Use `holons::parse_flags()` for CLI args, `holons::parse_holon()` for identity.
- File I/O RPCs work with local paths — no streaming over JSON-RPC (files only).
- All RPC boundaries are stateless per call.
