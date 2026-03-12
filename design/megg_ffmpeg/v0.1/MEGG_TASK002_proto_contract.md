# MEGG TASK002 — Proto Contract

## Goal

Define the gRPC/JSON-RPC contract for the `media.v1.Media` service. This
proto is the single source of truth for all v0.1 RPCs — 6 task-oriented
operations plus `Execute` and `ExecuteProbe` pass-throughs that expose the
full FFmpeg + ffprobe capability surface. Shared with Go, Dart, Swift, and
Kotlin consumers via the monorepo `protos/` tree.

## Proto: `protos/media/v1/media.proto`

```protobuf
syntax = "proto3";
package media.v1;

service Media {
  // --- Task-oriented RPCs ---
  rpc Probe(ProbeRequest) returns (ProbeResponse);
  rpc ExtractAudio(ExtractAudioRequest) returns (ExtractAudioResponse);
  rpc ExtractSegment(ExtractSegmentRequest) returns (ExtractSegmentResponse);
  rpc TranscodeAudio(TranscodeAudioRequest) returns (TranscodeAudioResponse);
  rpc MuxSubtitles(MuxSubtitlesRequest) returns (MuxSubtitlesResponse);
  rpc GetVersion(GetVersionRequest) returns (GetVersionResponse);

  // --- Pass-through RPCs (full FFmpeg/ffprobe surface) ---
  rpc Execute(ExecuteRequest) returns (ExecuteResponse);
  rpc ExecuteProbe(ExecuteProbeRequest) returns (ExecuteProbeResponse);
}
```

### Execute — Full FFmpeg Pass-Through

```protobuf
message ExecuteRequest {
  repeated string args = 1;  // FFmpeg CLI-equivalent args
                              // e.g. ["-i", "in.mp4", "-vf", "scale=1920:1080", "out.mp4"]
}

message ExecuteResponse {
  int32 exit_code = 1;
  string stdout = 2;
  string stderr = 3;
  repeated string output_paths = 4;  // Files created by the operation
}
```

### ExecuteProbe — Full ffprobe Pass-Through

```protobuf
message ExecuteProbeRequest {
  string input_path = 1;             // Media file to probe
  repeated string extra_args = 2;    // Additional ffprobe args
                                      // e.g. ["-show_packets", "-select_streams", "a:0"]
  string output_format = 3;          // "json" (default), "xml", "csv", "flat", "ini"
}

message ExecuteProbeResponse {
  int32 exit_code = 1;
  string output = 2;                 // Full ffprobe output in requested format
  string stderr = 3;
}
```

> Both `Execute` and `ExecuteProbe` are implemented using `libav*` directly —
> they do NOT shell out to `ffmpeg`/`ffprobe` binaries. The args are parsed
> and mapped to the equivalent C API calls.

## Message Design Rules

- All file paths are **absolute local paths** — no URLs, no relative paths
- Duration/time fields are `double` in seconds
- Bitrate fields are `int32` in bps (not kbps)
- Codec names use FFmpeg canonical names (e.g. `aac`, `h264`, `pcm_f32le`)
- Language tags use BCP-47 format
- Zero values mean "use default" (e.g. `sample_rate = 0` → original)

## Stream Info Model

Each media stream is described by `StreamInfo`:

| Field | Type | Scope |
|-------|------|-------|
| `index` | int32 | All |
| `type` | string | All — `audio`, `video`, `subtitle`, `data` |
| `codec` | string | All |
| `sample_rate` | int32 | Audio only |
| `channels` | int32 | Audio only |
| `width` | int32 | Video only |
| `height` | int32 | Video only |
| `frame_rate` | double | Video only |
| `language` | string | BCP-47 if available |
| `bit_depth` | int32 | Audio only (16, 24, 32) |
| `channel_layout` | string | Audio only (`mono`, `stereo`, `5.1`) |
| `pixel_format` | string | Video only (`yuv420p`, `rgb24`) |

## Error Model

JSON-RPC errors use standard codes:
- `-32602` — invalid params (bad path, unsupported codec)
- `-32603` — internal error (FFmpeg returned non-zero)
- Custom: `-1` — file not found, `-2` — no suitable stream

Error `data` field carries the FFmpeg error string for diagnostics.

## Acceptance Criteria

- [ ] `protos/media/v1/media.proto` written and compilable
- [ ] All 8 service methods documented with field-level comments
- [ ] `Execute` accepts arbitrary FFmpeg-equivalent args
- [ ] `ExecuteProbe` returns full ffprobe output in JSON/XML/CSV
- [ ] `StreamInfo` covers audio, video, subtitle, and data streams
- [ ] Zero-value semantics documented for optional fields
- [ ] Error codes documented in proto file comments

## Dependencies

- TASK01 (repo structure exists)

