# MEGG TASK004 — JSON-RPC CLI Dispatcher

## Goal

Wire `media_service` to the `cpp-holons` SDK's JSON-RPC transport. The CLI
binary `megg-ffmpeg` listens on stdio (default) or tcp, receives JSON-RPC
requests, dispatches to the service, and returns JSON-RPC responses.

## Entry Point: `cmd/megg-ffmpeg/main.cpp`

Same pattern as `wisupaa-whisper`:

1. `holons::parse_holon("HOLON.md")` → print motto to stderr
2. `holons::parse_flags(args)` → extract transport URI (default `stdio://`)
3. `holons::listen(uri)` → `accept()` → JSON-RPC read loop
4. Dispatch by method name
5. Return result or error

## Method Dispatch Table

| JSON-RPC method | Service function |
|----------------|-----------------|
| `"probe"` | `media_service::probe(req)` |
| `"extract_audio"` | `media_service::extract_audio(req)` |
| `"extract_segment"` | `media_service::extract_segment(req)` |
| `"transcode_audio"` | `media_service::transcode_audio(req)` |
| `"mux_subtitles"` | `media_service::mux_subtitles(req)` |
| `"get_version"` | `media_service::get_version()` |
| `"execute"` | `media_service::execute(req)` |
| `"execute_probe"` | `media_service::execute_probe(req)` |

## JSON Serialization

Use `nlohmann/json.hpp` for request/response serialization:

```cpp
// Request parsing
auto req = j["params"].get<ProbeRequest>();

// Response serialization
nlohmann::json result = media_service::probe(req);
```

Define `to_json` / `from_json` for all request and response types.

## Transport Support

| Transport | URI | Status |
|-----------|-----|--------|
| stdio | `stdio://` | Default — used by `connect(slug)` |
| tcp | `tcp://localhost:PORT` | For standalone debugging |
| unix | `unix:///path/to/socket` | Linux/macOS |

## Acceptance Criteria

- [ ] `megg-ffmpeg` binary compiles and runs
- [ ] Prints HOLON.md motto to stderr on startup
- [ ] Accepts JSON-RPC over stdio and returns valid responses
- [ ] All 6 methods dispatch correctly
- [ ] Unknown methods return `-32601` (method not found)
- [ ] Malformed JSON returns `-32700` (parse error)
- [ ] TCP transport works for debugging

## Dependencies

- TASK03 (service library exists)
