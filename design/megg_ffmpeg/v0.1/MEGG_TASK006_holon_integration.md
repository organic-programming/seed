# MEGG TASK006 — Holon Integration

## Goal

Wire `megg-ffmpeg` into the Organic Programming ecosystem: `holon.yaml`
manifest, submodule registration in the monorepo, and `op build` validation.

## holon.yaml

```yaml
schema: holon/v0
given_name: megg
family_name: FFmpeg
kind: daemon
transport: stdio
platforms: [macos, linux]
build:
  runner: cmake
  defaults:
    target: macos
    mode: debug
  targets:
    macos:
      steps:
      - exec:
          cwd: .
          argv: ["cmake", "-B", "build", "-DCMAKE_BUILD_TYPE=Release"]
      - exec:
          cwd: .
          argv: ["cmake", "--build", "build", "-j4"]
      - assert_file:
          path: build/megg-ffmpeg
    linux:
      steps:
      - exec:
          cwd: .
          argv: ["cmake", "-B", "build", "-DCMAKE_BUILD_TYPE=Release"]
      - exec:
          cwd: .
          argv: ["cmake", "--build", "build", "-j$(nproc)"]
      - assert_file:
          path: build/megg-ffmpeg
requires:
  commands: [cmake, make, gcc]
artifacts:
  primary: build/megg-ffmpeg
```

## Submodule Registration

```bash
cd <monorepo-root>
git submodule add git@github.com:organic-programming/megg-ffmpeg.git holons/megg-ffmpeg
```

## op build Validation

After registration, these must work:

```bash
op build holons/megg-ffmpeg                # builds FFmpeg + megg-ffmpeg
op run holons/megg-ffmpeg                  # starts on stdio
echo '{"jsonrpc":"2.0","id":1,"method":"get_version","params":{}}' | op run --no-build holons/megg-ffmpeg
```

## OP_CONFIG Integration

LGPL/GPL mode via build config:

```bash
op build --config lgpl holons/megg-ffmpeg   # default
op build --config gpl  holons/megg-ffmpeg   # enables GPL codecs
```

The CMakeLists.txt reads `OP_CONFIG` and sets `MEGG_LICENSE` accordingly.

## Acceptance Criteria

- [ ] `holon.yaml` present and valid
- [ ] `op build` compiles FFmpeg + binary on macOS
- [ ] `op run` starts the holon, `get_version` returns valid response
- [ ] `--config gpl` builds with GPL flag enabled
- [ ] Submodule added to monorepo at `holons/megg-ffmpeg`
- [ ] testmatrix (if applicable) includes megg-ffmpeg as a target

## Dependencies

- TASK04 (CLI binary works), TASK05 (tests pass)
