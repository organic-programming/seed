# relay-cascade

Cross-language cascade testing for the observability relay primitive.

## What this is

A development tool, not a stable feature. Each `relay-cascade-<lang>` is a
composite holon that spawns a 4-deep chain of `cascade-node-<lang>` instances
and verifies that logs, metrics, and events propagate from leaf to root across
mixed-language boundaries.

## Why

Holons relay observability up to their parent via `serve.Options.MemberEndpoints`
+ `startMemberRelays`. Cross-language drift is invisible to unit tests because
each SDK validates against itself. This cascade exercises the wire protocol with
every supported language pair, exposing primitive bugs that single-language
tests miss.

## Status

Pre-alpha. Covers Go ↔ Dart ↔ Rust today (180/180 assertions PASS). Other
languages (Python, Ruby, Node, Java, Kotlin, C#, Swift, Zig, C++, C) are
pending — same pattern, no scaffolder yet.

## Layout

```
_protos/relay/v1/relay.proto    shared Tick contract
cascade-node-<lang>/            atomic node, one per language
relay-cascade-<lang>/           composite orchestrator, one per language
```

## Architecture

**Atomic node (`cascade-node-<lang>`).** Implements `RelayService.Tick`: emits a
log line and increments counter `cascade_ticks_total`. Accepts repeated
`--member <slug>=<address>` flags to declare its downstream children.

**Composite (`relay-cascade-<lang>`).** Spawns a 4-deep chain
(root → mid1 → mid2 → leaf), wires `MemberEndpoints` so each parent relays
observability from its children, issues `Tick` RPCs, then reads the root's
observability streams and asserts that each phase's records surface with the
correct chain.

## Modes

| Mode | Purpose |
|---|---|
| *default* | Single 4-node chain, all nodes in the composite's own language. |
| `--live-stream` | Long-lived `Follow:true` streams; verifies records surface as they happen, not only on drain. |
| `--multi-pattern` | Runs three patterns back-to-back covering depth-2/depth-3 alter-language splits. |

## Patterns (`--multi-pattern`)

| Composite | Patterns |
|---|---|
| `relay-cascade-go` | go-go-go-go · go-go-dart-go · go-go-dart-dart |
| `relay-cascade-dart` | dart-dart-dart-dart · dart-dart-go-dart · dart-dart-go-go |
| `relay-cascade-rust` | rust-rust-rust-rust · rust-rust-go-rust · rust-rust-go-go |

Go is the alter language for the Dart and Rust composites; Dart is the alter
language for the Go composite. Every pair is exercised in both roles.

## Running

```bash
op build cascade-node-go --install
op build cascade-node-dart --install
op build cascade-node-rust --install
op build relay-cascade-go --install
op build relay-cascade-dart --install
op build relay-cascade-rust --install

op run relay-cascade-go                       # default
op run relay-cascade-go -- --live-stream      # live-stream
op run relay-cascade-go -- --multi-pattern    # full matrix
```

Each invocation prints `PASS` / `FAIL` per phase and exits non-zero on any
failure.

## Chain enrichment convention

When a parent relays a child's observability record, it prepends its own slug
to `chain` — but excludes the slug performing the read. The originator stays at
the head. A leaf log read by `root` looks like:

```
chain = [leaf, mid2, mid1]
```

Not `[root, mid1, mid2, leaf]`. Every SDK must honor this; the cascade asserts
it.

## What this exposes — and what it doesn't

Surfaces:
- MemberEndpoint UID resolution (parent must learn the UID from the child's
  `INSTANCE_READY` event, not assume it).
- Subscribe-before-drain races in stream consumers.
- Member identity refresh across reconnects.

Does not yet cover:
- Flutter app layer. The cascade rules out the SDK; if a Flutter app loses
  observability, the bug is in the kit or app, not the primitive.

## Adding a new language

No scaffolder yet. Copy `gabriel-greeting-<lang>` as a starting point, strip to
the minimum (single RPC + observability emission), and follow the existing
`cascade-node-rust` / `relay-cascade-rust` as the closest reference for the
composite half. Then port the assertions one phase at a time until all three
modes pass.
