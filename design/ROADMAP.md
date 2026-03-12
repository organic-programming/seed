# Organic Programming — Design Roadmap

Unified timeline orchestrating all design components. Individual
component roadmaps live in their respective directories.

**Strategic priority**: Grace (op) → Rob Go → Jack Middle → Line
Git → Phill Files → Wisupaa Whisper → Megg FFmpeg.

---

## Phase 0 — Foundation ✅

_Completed. The OP ecosystem has a working SDK, CLI, and recipe system._

| Component | Version | Status |
|---|---|:---:|
| [Grace OP](./grace-op/ROADMAP.md) | v0.3 (Core Maturity) | ✅ |
| [Grace OP](./grace-op/ROADMAP.md) | v0.4.1 (Go+Dart PoC) | ✅ |
| [Grace OP](./grace-op/ROADMAP.md) | v0.4.2 (Matrix Extraction) | ✅ |
| [Grace OP](./grace-op/ROADMAP.md) | v0.4.3 (Assembly & Composition) | ✅ |

---

## Phase 1 — Consolidation

_Finish the recipe ecosystem, start Rob-go, begin sessions._

| Component | Version | Theme | Depends on |
|---|---|---|---|
| [Grace OP](./grace-op/ROADMAP.md) | v0.4.4 | Bundle auto-signing | Grace v0.4.3 ✅ |
| [Grace OP](./grace-op/ROADMAP.md) | v0.5 | Extensibility (configs, MVS, sequences) | Grace v0.4.4 |
| [Rob Go](./rob-go/ROADMAP.md) | v0.1 | Toolchain holon (hermetic Go distro) | Grace v0.5 |
| [Sessions](./features/sessions/ROADMAP.md) | v0.1 | Specification & protocol | — |
| [Sessions](./features/sessions/ROADMAP.md) | v0.2 | Go SDK reference implementation | Sessions v0.1 |

---

## Phase 2 — Core Holons

_Rob-go matures. Line and Phill provide foundational utilities.
Jack enters as the observability layer._

| Component | Version | Theme | Depends on |
|---|---|---|---|
| [Rob Go](./rob-go/ROADMAP.md) | v0.2 | Version management | Rob v0.1 |
| [Rob Go](./rob-go/ROADMAP.md) | v0.3 | CGO toolchain | Rob v0.2 |
| [Jack Middle](./jack-middle/ROADMAP.md) | v0.1 | Transparent gRPC proxy + middleware | Grace v0.5 |
| [Jack Middle](./jack-middle/ROADMAP.md) | v0.2 | `op` injection (`--via jack`) | Jack v0.1 |
| Line Git | — | Git operations holon | Grace v0.5 |
| Phill Files | — | File system operations holon | Grace v0.5 |
| [Sessions](./features/sessions/ROADMAP.md) | v0.3 | Metrics (4-phase decomposition) | Sessions v0.2 |
| [Sessions](./features/sessions/ROADMAP.md) | v0.4 | `op` CLI + recipe integration | Sessions v0.3, Grace v0.5 |
| [Sessions](./features/sessions/ROADMAP.md) | v0.5 | Jack integration + Prometheus export | Sessions v0.3, Jack v0.1 |

---

## Phase 3 — C++ Holons

_Wisupaa first (simpler, teaches patterns), then Megg (last, most
complex). Megg benefits from every lesson learned before it._

| Component | Version | Theme | Depends on |
|---|---|---|---|
| [Wisupaa Whisper](./wisupaa-whisper/ROADMAP.md) | v0.1 | Bootstrap (Transcribe, DetectLanguage) | Grace v0.5 |
| [Wisupaa Whisper](./wisupaa-whisper/ROADMAP.md) | v0.2 | Precision (word timestamps, alignment) | Wisupaa v0.1 |
| [Megg FFmpeg](./megg_ffmpeg/ROADMAP.md) | v0.1 | Bootstrap (Probe, Extract, Transcode) | Grace v0.5, Wisupaa v0.1 lessons |
| [Megg FFmpeg](./megg_ffmpeg/ROADMAP.md) | v0.2 | Upstream (Segmenter, alignment prep) | Megg v0.1, Wisupaa v0.2 |

**Why Wisupaa before Megg:**
- Simpler codebase — whisper.cpp is a single library, FFmpeg is 100+ libs
- MIT license — no GPL gating complexity
- Faster build — seconds vs 5–15 minutes for FFmpeg
- Same CMake + submodule pattern — lessons learned carry over to Megg

**Why Megg is last:** Megg is the most complex holon in the
ecosystem. By the time it starts, Grace, Rob, Jack, Line,
Phill, and Wisupaa have all proven the patterns.

---

## Phase 4 — Cross-SDK & Distribution

_Port sessions to all SDKs. Wrapper holons for provisioning.
Start the distribution pipeline._

| Component | Version | Theme | Depends on |
|---|---|---|---|
| [Sessions](./features/sessions/ROADMAP.md) | v0.6 | Cross-SDK ports | Sessions v0.2 |
| [Grace OP](./grace-op/ROADMAP.md) | v0.6 | REST + SSE transport | Grace v0.5 |
| [Grace OP](./grace-op/ROADMAP.md) | v0.7 | Cross-compilation | Grace v0.6 |
| [Grace OP](./grace-op/ROADMAP.md) | v0.8 | Release pipeline + signing | Grace v0.7 |
| [Wisupaa Whisper](./wisupaa-whisper/ROADMAP.md) | v0.3 | Streaming transcription | Wisupaa v0.2 |
| [Megg FFmpeg](./megg_ffmpeg/ROADMAP.md) | v0.3 | Filters + HW accel | Megg v0.1 |
| Al Brew | — | Homebrew wrapper (macOS) | Grace v0.5 |
| Jess NPM | — | NPM/Node.js wrapper | Grace v0.5 |
| Marvin Winget | — | WinGet wrapper (Windows) | Grace v0.5 |
| Gertrude Apt | — | APT wrapper (Debian/Ubuntu) | Grace v0.5 |

**Wrapper holons** (Al, Jess, Marvin, Gertrude) are the building
blocks for Grace v0.11 (`op setup`).

---

## Phase 5 — Distributed & Production

_Mesh networking, public holons, advanced session tracking across
hosts, production hardening of C++ holons._

| Component | Version | Theme | Depends on |
|---|---|---|---|
| [Grace OP](./grace-op/ROADMAP.md) | v0.9 | Mesh networking (mTLS) | Grace v0.8 |
| [Grace OP](./grace-op/ROADMAP.md) | v0.10 | Public holons (auth, multi-listener) | Grace v0.9 |
| [Grace OP](./grace-op/ROADMAP.md) | v0.11 | Setup (provisioning from zero) | Grace v0.10, Al + Jess + Marvin + Gertrude |
| [Sessions](./features/sessions/ROADMAP.md) | v0.7 | Advanced transports (REST+SSE, mesh, public) | Sessions v0.6, Grace v0.9+ |
| [Wisupaa Whisper](./wisupaa-whisper/ROADMAP.md) | v0.4–v1.0 | Intelligence → Performance → Production | Wisupaa v0.3 |
| [Megg FFmpeg](./megg_ffmpeg/ROADMAP.md) | v0.4–v1.0 | Streaming → Analysis → Packaging → Production | Megg v0.3 |

---

## Visual Timeline

```
Phase 0 ✅          Phase 1               Phase 2              Phase 3              Phase 4              Phase 5
──────────          ───────               ───────              ───────              ───────              ───────

Grace v0.3 ✅ ───→  v0.4.4 ──→ v0.5 ─────────────────────────→ v0.6 → v0.7 → v0.8 → v0.9 → v0.10 → v0.11
Grace v0.4.1-3 ✅

Rob-go                          v0.1 ──→ v0.2 → v0.3

Jack                                      v0.1 → v0.2

Line Git                                  v0.1
Phill Files                               v0.1

Sessions            v0.1 → v0.2 ─────→ v0.3 → v0.4 → v0.5 ──→ v0.6 ──────────────→ v0.7
                    (spec)  (Go ref)    (metrics)(CLI)(Jack)    (all SDKs)            (advanced)

Wisupaa                                                        v0.1 → v0.2 ────────→ v0.3 → v1.0

Megg                                                           v0.1 → v0.2 → v0.3 → v0.4 → v1.0

Al / Jess / Marvin / Gertrude                                  v0.1
```

---

## Planned Holons (No Design Yet)

| Holon | Family | Purpose | Strategic phase |
|---|---|---|---|
| **Line** Git | Utility | Git operations via RPC | Phase 2 |
| **Phill** Files | Utility | File system operations via RPC | Phase 2 |
| **Al** Brew | Wrapper | Homebrew wrapper (macOS) | Phase 4 |
| **Jess** NPM | Wrapper | NPM/Node.js wrapper | Phase 4 |
| **Marvin** Winget | Wrapper | WinGet wrapper (Windows) | Phase 4 |
| **Gertrude** Apt | Wrapper | APT wrapper (Debian/Ubuntu) | Phase 4 |

---

## Cross-Cutting Tracks

These are not tied to a single phase — they evolve continuously
alongside the holons and features they validate.

| Track | Scope | Evolves with |
|---|---|---|
| **Recipes** | Assembly manifests, composition patterns, testmatrix | Every new holon and transport |
| **SDKs** | go-holons, rust-holons, swift-holons, dart-holons, etc. | Sessions, transports, connect, serve |
| **Examples** | Hello-world per SDK, recipe demos, integration cookbook | Each holon milestone |

- **Recipes** grow each time a new holon or transport is added —
  new assembly manifests, new composition patterns, new testmatrix
  entries.
- **SDKs** absorb sessions (v0.2/v0.6), new transports (REST+SSE,
  mesh), and connect/serve improvements across phases.
- **Examples** validate each milestone — if a holon ships without
  a working hello-world, it isn't done.

---

## Parallel Tracks

Work within each phase can be parallelized:

- **Phase 1**: Grace v0.4.4 ∥ Grace v0.5 ∥ Rob v0.1 ∥ Sessions v0.1–v0.2
- **Phase 2**: Rob v0.2–v0.3 ∥ Jack v0.1–v0.2 ∥ Line ∥ Phill ∥ Sessions v0.3–v0.5
- **Phase 3**: Wisupaa v0.1–v0.2 ∥ Megg v0.1
- **Phase 4**: Grace v0.6–v0.8 ∥ Sessions v0.6 ∥ Al ∥ Jess ∥ Marvin ∥ Gertrude
- **Phase 5**: Grace v0.9–v0.11 ∥ Sessions v0.7 ∥ production hardening
