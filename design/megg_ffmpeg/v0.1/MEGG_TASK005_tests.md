# MEGG TASK005 — Tests

## Goal

Validate every RPC with unit tests (struct/logic) and integration tests
(real FFmpeg calls with media fixtures). Catch regressions early.

## Test Architecture

```
tests/
├── test_media_types.cpp       — struct serialization roundtrips
├── test_probe.cpp             — Probe RPC with real files
├── test_extract_audio.cpp     — ExtractAudio + PCM validation
├── test_extract_segment.cpp   — ExtractSegment timing accuracy
├── test_transcode.cpp         — TranscodeAudio codec conversion
├── test_mux_subtitles.cpp     — MuxSubtitles with SRT/VTT/ASS
├── test_get_version.cpp       — Version + license mode
├── test_error_handling.cpp    — Missing files, bad codecs, corrupt data
└── fixtures/
    ├── test_1s_stereo.wav     — 1s stereo PCM, 44100Hz
    ├── test_5s_video.mp4      — 5s H.264+AAC, 720p
    ├── test_subtitle.srt      — 3-line SRT
    └── test_corrupt.mp4       — Truncated container
```

## Test Categories

### Unit Tests (no FFmpeg calls)

- JSON serialization roundtrips for all request/response types
- Error struct construction and formatting
- RAII wrapper lifecycle (mock allocations)

### Integration Tests (require FFmpeg build)

| Test | Validates |
|------|-----------|
| Probe wav/mp4 | Format, duration, stream count, codec names |
| ExtractAudio from mp4 | Correct sample count, sample rate, mono conversion |
| ExtractSegment (stream copy) | Output duration matches request |
| ExtractSegment (reencode) | Accurate cuts at non-keyframe boundaries |
| TranscodeAudio wav→aac | Output codec is AAC, duration preserved |
| MuxSubtitles | Output has subtitle stream, original streams intact |
| GetVersion | Version string non-empty, license matches build config |

### Error Tests

| Test | Expected |
|------|----------|
| Probe non-existent file | Graceful error, no crash |
| ExtractAudio from video-only | Error: no audio stream |
| TranscodeAudio to unsupported codec | Error: codec not found |
| MuxSubtitles with malformed SRT | Error or best-effort (no crash) |
| All RPCs with empty request | Error: missing required fields |

## Test Fixtures

Generate minimal fixtures via FFmpeg CLI (committed to repo):

```bash
# 1s stereo WAV
ffmpeg -f lavfi -i "sine=frequency=440:duration=1" -ac 2 -ar 44100 fixtures/test_1s_stereo.wav

# 5s video with audio
ffmpeg -f lavfi -i "testsrc2=duration=5:size=1280x720:rate=30" \
       -f lavfi -i "sine=frequency=440:duration=5" \
       -c:v libx264 -preset ultrafast -c:a aac fixtures/test_5s_video.mp4

# Simple SRT
echo -e "1\n00:00:00,000 --> 00:00:02,000\nHello World\n" > fixtures/test_subtitle.srt

# Corrupt file (truncated)
head -c 1024 fixtures/test_5s_video.mp4 > fixtures/test_corrupt.mp4
```

## Build Integration

```cmake
enable_testing()
add_test(NAME test_media_types COMMAND test_media_types)
add_test(NAME test_probe COMMAND test_probe)
# ... etc
```

## Acceptance Criteria

- [ ] All unit tests pass
- [ ] All integration tests pass with generated fixtures
- [ ] Error tests confirm graceful failure (no segfaults, no leaks)
- [ ] ASAN build passes all tests (no memory errors)
- [ ] Test fixtures are < 1 MB total (committed to repo)

## Dependencies

- TASK03 (service implementation to test)
