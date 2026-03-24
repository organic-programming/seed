# Codex SDK Cleanup — Execution Plan

## Overview

Execute [SDK_CLEANUP.md](SDK_CLEANUP.md) across all 13 SDKs via Codex.

| Phase | Status |
|-------|--------|
| **0 — Prep** (`op build` Incode Description) | ✅ Done |
| **1A — Go** (reference implementation) | ✅ Done |
| **1B — 12 remaining SDKs** (parallel) | ☐ Next |
| **2 — Finalize** (SwiftUI organism + audit) | ☐ After 1B |

---

## Phase 0 — ✅ Done

`op build` generates Incode Description via `sdk/<lang>-holons/templates/describe.<ext>.tmpl`. Implemented in `holons/grace-op/internal/holons/describe_source.go`.

## Phase 1A — ✅ Go SDK Done

Static-only Describe, fail-fast HolonMeta, REST+SSE, WSS, full transport matrix. See `sdk/go-holons/README.md`.

Key design decisions (binding for all other SDKs):
- **Static-only runtime** — `describe.Register` serves the generated static response only. No proto fallback at runtime.
- **Missing Incode → startup failure** — if no static response is registered, serve fails with: `"no Incode Description registered — run op build"`.
- **`BuildResponse` is build-time only** — stays in SDK as a library for `op build`, not called at runtime.
- **REST+SSE** — native HTTP+SSE JSON-RPC binding per `PROTOCOL.md` §5.3.2. POST for unary, POST/GET with `Accept: text/event-stream` for server-streaming.

---

## Phase 1B — 12 SDKs in Parallel

### Codex prompt — Per-SDK

> Replace `LANG`, `SDK_DIR`, `EXAMPLE_DIR` at the top before pasting.

```text
You are working in the organic-programming monorepo.

LANG        = dart
SDK_DIR     = sdk/dart-holons
EXAMPLE_DIR = examples/hello-world/gabriel-greeting-dart

Reference implementation (completed): sdk/go-holons — the canonical pattern for every step.

Key design rule: Describe is STATIC-ONLY at runtime.
- The SDK must register a generated static response at startup. No runtime proto parsing.
- If no static response is registered, serve must fail with a clear error and prevent startup.
- pkg/describe.BuildResponse (or equivalent) exists only as a build-time utility for op build. It is NOT a runtime code path.
- See sdk/go-holons/pkg/describe/static.go for the registration hook pattern.

Context files to read first:
- sdk/README.md — Incode Description spec, transport matrix, per-SDK README spec
- sdk/go-holons/ — reference implementation for all modules
- PROTOCOL.md §5.3.2 — HTTP+SSE transport spec (Content-Type, status codes, SSE event format, CORS)

Execute the following steps IN ORDER. After each step, commit with `fix(LANG-holons):` or `feat(LANG-holons):`.

### Step 1 — Incode Description Template
Create `SDK_DIR/templates/describe.<ext>.tmpl`.
This Go template receives the same data model as the Go reference and produces a native source file that exports a static DescribeResponse equivalent.

### Step 2 — Explicit Describe Registration Errors
In the SDK's serve module, make HolonMeta registration fail loudly.
If no static response is registered → fail with "no Incode Description registered — run op build".
If registration errors for any other reason → log clearly and abort startup.

### Step 3 — Standard Describe / Identity / Discover
Align the SDK's modules to match go-holons:
- `describe`: Static-only at runtime. Accepts the generated response via a registration hook. No proto fallback.
- `identity`: Reads holon.proto and exposes the resolved manifest (build/dev-time utility).
- `discover`: Scans the filesystem for nearby holons by slug.

### Step 4 — Proto-less Describe
Test that a built binary with NO adjacent .proto files returns the static describe response.
This is the ONLY describe path, not a special case.

### Step 5 — Transport Matrix Audit
For each transport (`tcp://`, `unix://`, `stdio://`, `ws://`, `wss://`, `rest+sse://`):
- Read source code to determine Dial and/or Serve support.
- Write or verify a test that proves the capability.
- Update sdk/README.md's transport matrix row, replacing `?` with proven values.
Do NOT assume — only mark what you can prove with a passing test.

### Step 6 — Implement Required Transports
Compare current state with the expected v0.6 matrix in sdk/README.md.
Implement missing transports. Each must have a test.
For `rest+sse`: implement the HTTP+SSE JSON-RPC binding per PROTOCOL.md §5.3.2.

### Step 7 — Test Suite Cleanup
Run the full test suite. Fix broken tests, remove irrelevant ones, ensure all pass.

### Step 8 — Rebuild Hello-World Example
Rebuild EXAMPLE_DIR via `op build`. Wire the static describe registration.
Fix any issues that surface. The generated describe file should be committed.

### Step 9 — Per-SDK README
Create or rewrite `SDK_DIR/README.md` per sdk/README.md § "Per-SDK Documentation":
- serve — one working snippet
- transport — how to specify the listen URI
- identity / describe — one line to wire generated Incode Description
- discover — one call to find a holon
- connect — one call to get a ready channel

### Step 10 — Final Check
Run all tests. Ensure no regressions.
```

### Execution checklist

| LANG | SDK_DIR | EXAMPLE_DIR | Status |
|------|---------|-------------|--------|
| `c` | `sdk/c-holons` | `examples/hello-world/gabriel-greeting-c` | ☐ |
| `cpp` | `sdk/cpp-holons` | `examples/hello-world/gabriel-greeting-cpp` | ☐ |
| `csharp` | `sdk/csharp-holons` | `examples/hello-world/gabriel-greeting-csharp` | ☐ |
| `dart` | `sdk/dart-holons` | `examples/hello-world/gabriel-greeting-dart` | ☐ |
| `java` | `sdk/java-holons` | `examples/hello-world/gabriel-greeting-java` | ☐ |
| `js` | `sdk/js-holons` | `examples/hello-world/gabriel-greeting-node` | ☐ |
| `js-web` | `sdk/js-web-holons` | `examples/hello-world/gabriel-greeting-node`[^1] | ☐ |
| `kotlin` | `sdk/kotlin-holons` | `examples/hello-world/gabriel-greeting-kotlin` | ☐ |
| `python` | `sdk/python-holons` | `examples/hello-world/gabriel-greeting-python` | ☐ |
| `ruby` | `sdk/ruby-holons` | `examples/hello-world/gabriel-greeting-ruby` | ☐ |
| `rust` | `sdk/rust-holons` | `examples/hello-world/gabriel-greeting-rust` | ☐ |
| `swift` | `sdk/swift-holons` | `examples/hello-world/gabriel-greeting-swift` | ☐ |

[^1]: `js-web-holons` is browser-only (dial only). Shares the Node hello-world example.

---

## Phase 2 — Finalize

After all SDKs pass:

```text
You are working in the organic-programming monorepo.
All 13 SDKs have been cleaned up.

Final tasks:
1. Verify the SwiftUI Organism (examples/hello-world/gabriel-greeting-app-swiftui) builds and runs via `op build` + `op run`.
2. Run the full test suite across ALL SDKs.
3. Verify the transport matrix in sdk/README.md has NO remaining `?` entries.
4. Verify every SDK has a README.md matching the per-SDK docs spec.
```

---

## Tips

1. **Parallel** — launch all 12 Codex sessions simultaneously.
2. **Static-only** — no proto fallback. Missing Incode Description = startup failure.
3. **REST+SSE** — implement per PROTOCOL.md §5.3.2, not gRPC-over-HTTP.
4. **Commit after each step** — rollback-friendly.
5. **Don't trust `?`** — prove with tests, not assumptions.
