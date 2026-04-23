# Holon Instance Spec

Status: FROZEN v1
⚠️ This specification describes how `Discover` targets running instances,
how external callers enumerate them, and how instance identity propagates
into session tracing ([SESSIONS.md](SESSIONS.md)) and observability
signals ([OBSERVABILITY.md](OBSERVABILITY.md)).

---

## Concept

A **holon instance** is a running process of a holon.
Multiple instances of the same holon can coexist.

Discovery (`DISCOVERY.md`) finds *holons* (packages).
This spec governs running *processes* of those holons.

---

## Identity

To target a specific running process, append the instance UID to the base Address (slug, alias, or path) using a colon `:`.

```
<address>:<uid>

gabriel-greeting-go               // default volatile instance 
gabriel-greeting-go:ea346efb      // specific instance (UID prefix)
op:ea346efb                       // specific instance by alias
```

The `uid` is an identifier (typically a UUID) assigned at spawn time.

| Form | Meaning |
|---|---|
| `gabriel-greeting-go` | default instance |
| `gabriel-greeting-go:ea346efb` | specific instance by UID prefix |

A "default" instance is the one marked `default: true` in the registry,
or the sole running instance, or a fresh volatile one if none exists.

---

## Registry

Instances are tracked in `<run_root>/<address>/<uid>/`, where `run_root`
is resolved per [§Path Resolution](#path-resolution) below:

```
<run_root>/
  gabriel-greeting-go/
    ea346efb1123.../
      pid            // OS process ID
      socket         // unix socket or tcp port binding
      stdio          // fifo handles if kept alive (persistent mode)
      meta.json      // started_at, last_call, ttl, mode, default,
                     // metrics_addr, log_path, log_bytes_rotated
      stdout.log     // see §Log Contract
      stderr.log     // see §Log Contract
      events.jsonl   // see §Log Contract
      stdout.log.1   // rotated (when present)
      stdout.log.2
      ...
```

The holon package itself is never mutated by instance state.

### Path Resolution

The root directory for the registry is resolved with this priority:

1. `$OP_RUN_DIR` — explicit override; honoured verbatim when set.
2. `$OPPATH/run` — user-local default when `OPPATH` is set.
3. `<OPROOT>/.op/run` — project-local, when inside an `OPROOT`.
4. `./.op/run` — fallback, relative to the current working directory.

`op run` writes to the first path that resolves to a writable directory
it can create. The chosen path is recorded in the child's environment as
`OP_RUN_DIR` so downstream tooling (e.g. `HolonInstance.List` callers,
`op ps`) can find the registry without re-deriving it.

### Log Contract

When the launcher sets `OP_OBS=logs` (or `events`), the SDK writes
structured signals to files in `<run_root>/<address>/<uid>/`:

| File | Format | Emitter |
|---|---|---|
| `stdout.log` | JSON lines (one LogEntry per line) | SDK logger |
| `stderr.log` | raw bytes | child process stderr |
| `events.jsonl` | JSON lines (one EventInfo per line) | SDK event bus + parent |

Rotation policy (default, tunable via `holon.proto`):

- Rotate at **16 MB** per file.
- Keep a **ring of 4** rotated files (`stdout.log.1`–`.4`;
  `.4` is oldest and evicted on next rotation).
- `meta.json` records `log_path` (absolute path to the current
  `stdout.log`) and `log_bytes_rotated` (cumulative bytes evicted).

When `OP_OBS` does not contain `logs`, the SDK may still write
`stdout.log` and `stderr.log` as raw captures of the child's
standard streams (useful for crash forensics). Rotation policy
applies identically.

See [OBSERVABILITY.md §Log Model](OBSERVABILITY.md#log-model) for
the structured-log schema.

---

## Runtime Discovery

The registry on disk is the source of truth. Direct filesystem access
is fine for local tooling; anything else goes through a dedicated
gRPC service so that remote callers and cross-host operators can
enumerate instances without shared filesystem semantics.

```protobuf
syntax = "proto3";
package holoninstance.v1;

import "google/protobuf/timestamp.proto";

// HolonInstance is auto-registered by the parent supervisor (typically
// `op`). It is NOT registered by individual holons — listing instances
// is a supervisor concern, not a per-holon one.
service HolonInstance {
  rpc List(ListInstancesRequest) returns (ListInstancesResponse);
  rpc Get(GetInstanceRequest) returns (InstanceInfo);
}

message ListInstancesRequest {
  // Filter by slug. Empty = all slugs.
  repeated string slugs = 1;
  // Include instances whose PID liveness check failed (stale entries).
  bool include_stale = 2;
}

message ListInstancesResponse {
  repeated InstanceInfo instances = 1;
}

message GetInstanceRequest {
  // Full or prefix UID.
  string uid = 1;
}

message InstanceInfo {
  string slug = 1;
  string uid = 2;
  int32 pid = 3;
  google.protobuf.Timestamp started_at = 4;
  string mode = 5;              // "volatile" | "persistent" | "attached"
  string transport = 6;         // "stdio" | "tcp" | "unix" | ...
  string address = 7;           // concrete endpoint
  string metrics_addr = 8;      // Prometheus /metrics bind, if any
  string log_path = 9;          // current stdout.log absolute path
  bool default = 10;            // marked default in registry
  bool stale = 11;              // PID liveness check failed
}
```

The service is **auto-registered by `op`** (not by holons). A holon
running under `op` inherits access to `HolonInstance` through `op`'s
supervisor listener; manually launched holons can still be enumerated
via direct filesystem scan (`.op/run/` per §Path Resolution).

---

## CLI — `op ps` and `op instances`

```
$ op ps
SLUG                  UID       PID    STARTED              MODE        TRANSPORT  ADDRESS                METRICS_ADDR
gabriel-greeting-go   ea346efb  12341  2026-04-23 18:42     persistent  tcp        tcp://127.0.0.1:9090   http://127.0.0.1:9091
grace-op              3f08b5c3  12287  2026-04-23 18:40     attached    stdio      stdio://               —

2 instances
```

Flags:

| Flag | Purpose |
|---|---|
| `--all` | Traverse every `<run_root>` candidate (see §Path Resolution), not just the current one. |
| `--slug <slug>` | Filter to one slug; shorthand for `op instances <slug>`. |
| `--stale` | Include entries whose PID liveness check failed. |
| `--json` | Emit one JSON object per instance (machine-readable). |

`op instances <slug>` is equivalent to `op ps --slug <slug>`.

---

## UID Return Contract

Scripts launching holons with `op run` need the UID back without
having to parse log output. Two forms, both honoured by every
launcher:

### Default (text)

`op run <slug>` writes a **dedicated first line** to stdout before
any application output, prefixed `uid:`:

```
$ op run gabriel-greeting-go:9090
uid: ea346efb1123c4d5e6f7a8b9c0d1e2f3
<application output begins here>
```

Scripts consume this with `read` / `awk` / `head -n1`.

### JSON

`op run --json <slug>` emits **exactly one JSON object** on stdout
and nothing else (application stdout is redirected to
`<run_root>/<address>/<uid>/stdout.log`):

```
$ op run --json gabriel-greeting-go:9090
{"uid":"ea346efb1123c4d5e6f7a8b9c0d1e2f3","slug":"gabriel-greeting-go","address":"tcp://127.0.0.1:9090","pid":12341,"metrics_addr":"http://127.0.0.1:9091"}
```

Exit codes on both forms:
- `0` — instance is `READY` (first listener bound, `Describe` answers).
- non-zero — instance failed to reach `READY` within the readiness
  timeout ([COMMUNICATION.md §2.1](COMMUNICATION.md#21-readiness-verification)).

---

## Lifecycle Modes

| Mode | Lifetime | Stdio kept alive? |
|---|---|---|
| **Volatile** | dies after the call | no |
| **Persistent** | survives until explicit stop or TTL expiry | yes |
| **Attached** | lives for the duration of a parent process | inherited |

> **Default Modes**: 
> - If an instance is launched over `stdio://`, the default mode is **Volatile**.
> - If an instance binds a socket (`tcp://` or `unix://`), the default mode is **Persistent**.

*(Note: The 'Attached' mode was previously called 'Session', but was renamed to avoid absolute naming collisions with the `sessions/v0.1` network tracing spec).*

Persistent instances with stdio keepalive skip fork+exec on every call —
this is the intended hot-path optimization.

---

## Ports vs Instances

Instances own ports, not the reverse.

A port (TCP or Unix socket) is a transport detail of an instance.
The instance-id is the stable handle. The same instance may be reachable
over stdio *or* a TCP port depending on how it was started.

Muxing multiple callers to a single stdio-monovalent instance is the
responsibility of `op proxy` (see [holons/grace-op/OP_PROXY.md](holons/grace-op/OP_PROXY.md))
— a v2 feature. In v1, multivalent transports (`tcp://`, `unix://`,
`ws://`, `wss://`) accept concurrent connections natively; the
proxy only adds routing/fan-in on top.

---

## Instance ↔ Session Linkage

An instance owns zero or more sessions ([SESSIONS.md](SESSIONS.md)).
The linkage is one-way: sessions know the owning instance, not the
other way around — the instance registry does not list session IDs.

### Propagation mechanism

When a launcher spawns a holon, it injects two env vars into the child:

| Env var | Value | Set by |
|---|---|---|
| `OP_INSTANCE_UID` | The full UID assigned at spawn | `op run`, `op proxy`, any supervisor |
| `OP_RUN_DIR` | Resolved registry root (see §Path Resolution) | same |

The SDK's serve runner reads `OP_INSTANCE_UID` on startup and uses it
as the `instance_uid` field of every `SessionInfo` (see
[SESSIONS.md §Session Model](SESSIONS.md#session-model)) and of
every `LogEntry`, `MetricSample`, `EventInfo` (see
[OBSERVABILITY.md](OBSERVABILITY.md)).

Manually launched holons (no supervisor) have an empty
`OP_INSTANCE_UID`, in which case `instance_uid` is the empty string
on all signals. The holon is still observable; it is simply not
correlatable to a registry entry.

### Composite recipes

Composite recipes form an **organism**. See [§Organism Hierarchy](#organism-hierarchy)
for the full tree layout, identity propagation, and observability
aggregation rules. The short version: every child gets a distinct
`OP_INSTANCE_UID`, shares a common `OP_ORGANISM_UID`, and writes its
own signals locally while the root holds the merged multilog.

---

## Organism Hierarchy

An **organism** is a running tree of holons rooted at a single process,
typically launched by `op run` on a composite recipe, by a Flutter
organism kit, or by any parent supervisor that spawns members. The tree
can be arbitrary depth — members may themselves spawn subchildren.

### Identity propagation

Two env vars announce the organism to every member of the tree:

| Env var | Value | Set by |
|---|---|---|
| `OP_ORGANISM_UID` | UID of the root holon (invariant across the whole tree) | the root launcher |
| `OP_ORGANISM_SLUG` | slug of the root holon | the root launcher |

Each member still gets its own `OP_INSTANCE_UID` and `OP_RUN_DIR` as
described in [§Registry](#registry) and [§Instance ↔ Session Linkage](#instance--session-linkage).
Members inherit `OP_ORGANISM_UID` / `OP_ORGANISM_SLUG` unchanged as they
spawn their own children — these values are **constant across every hop
of the tree**, which is how any node identifies the organism it belongs
to.

The root holon has `OP_ORGANISM_UID == OP_INSTANCE_UID`. This equality
is the canonical test for "am I the root?" in SDK code.

### Registry tree

When `OP_ORGANISM_UID` is present, the registry uses a hierarchical
layout rooted at the organism:

```
<run_root>/
  gabriel-greeting-app/               # organism_slug
    4a7b8c9d1123.../                  # organism_uid (== root's instance_uid)
      pid  meta.json
      stdout.log     events.jsonl     # root's OWN signals only
      multilog.jsonl                  # AGGREGATED signals from the whole tree
      members/
        gabriel-greeting-go/
          ea346efb4455.../            # member's own instance_uid
            pid  meta.json
            stdout.log  events.jsonl  # member's OWN signals, written locally
            members/                  # only present if this member has children
              phill-files/
                1c2d3e4f.../
                  pid  meta.json
                  stdout.log  events.jsonl
        other-member/
          ...
```

A standalone holon (no `OP_ORGANISM_UID`) continues to use the flat
layout described in [§Registry](#registry). No migration; the two
shapes coexist in the same `<run_root>`.

### Writer responsibilities

| Who writes | What | Where |
|---|---|---|
| Every holon | own `stdout.log`, `stderr.log`, `events.jsonl` | its own directory |
| Root only | `multilog.jsonl` (aggregated) | `<run_root>/<organism_slug>/<organism_uid>/` |
| Intermediate levels | nothing extra | they only **relay** signals upward through the streams described in [OBSERVABILITY.md §Organism Relay](OBSERVABILITY.md#organism-relay) |

Dual-filesystem rationale: the multilog is the live narrative for
`op logs <organism>` and external forwarders. The local files are the
per-member forensic record, resilient to a root crash. Both are written
concurrently; they never conflict because they hold different data.

### Discovery of the tree

From any node, the organism root's directory is reachable via:
`<run_root>/<OP_ORGANISM_SLUG>/<OP_ORGANISM_UID>/`. `op ps` walks this
tree when invoked with `--tree` (default when `<slug>` names an
organism root) and renders a nested view; `op ps --flat` bypasses
the tree and lists every running holon in one line-per-entry form.

### Remote members

A member declared by the organism's `mesh.yaml` — typically a holon on
another host — still gets `OP_ORGANISM_UID` / `OP_ORGANISM_SLUG` via
the mesh bootstrap. Its local `<run_root>` lives on its own machine;
the multilog stays on the root's machine. The root's Observe streams
traverse the mesh connection identically to a local subprocess
([OBSERVABILITY.md §Transport Constraints](OBSERVABILITY.md#transport-constraints)).

A remote dialed opportunistically (not a declared organism member) does
**not** inherit the organism env. It is a peer, not a child; its
signals are only visible via explicit `op logs <remote-slug>` pull.

---

## Cleanup

Stale entries are detected by a PID liveness check on access.
A future reaper mechanism may periodically sweep `.op/run/`.

---

## Design Decisions (Frozen)

- **Targeting Instances via Discover**: Yes, `Discover()` fully supports targeting a single running instance (e.g. `gabriel-greeting-go/ea346efb`). This natively works over `LOCAL` and `DELEGATED` scopes.
- **Op Proxy Muxing**: Yes, `op proxy` fan-ins multiple incoming channels to a single persistent instance.
- **Port Binding**: Dynamic (ephemeral) binding is strongly recommended for local TCP/Unix to avoid `EADDRINUSE` port collisions. The final binding is written to the `<run_root>/` registry.
- **Path Resolution**: The registry root is resolved via `$OP_RUN_DIR → $OPPATH/run → <OPROOT>/.op/run → ./.op/run`. The chosen path is propagated to children as `OP_RUN_DIR` so enumeration is deterministic across a process tree.
- **UID Propagation**: Every launcher injects `OP_INSTANCE_UID` and `OP_RUN_DIR` into children. The SDK stamps these onto every session and observability signal.
- **UID Return**: `op run <slug>` emits a `uid: …` first line; `op run --json <slug>` emits a single JSON object and redirects application stdout to `stdout.log`.
- **Enumeration RPC**: `HolonInstance.List` / `.Get` are auto-registered by the **supervisor** (typically `op`), not by individual holons.
- **Log Contract**: `<run_root>/<address>/<uid>/{stdout.log,stderr.log,events.jsonl}`, rotated at 16 MB, ring of 4 files.
- **Organism Hierarchy**: when `OP_ORGANISM_UID` is set, the registry uses a hierarchical layout rooted at the organism directory, with members nested under `members/<child_slug>/<child_uid>/` to arbitrary depth. The root is identified by `OP_ORGANISM_UID == OP_INSTANCE_UID`.
- **Multilog Ownership**: only the organism root writes `multilog.jsonl`. Intermediate members only relay; each member still writes its own local `stdout.log` / `events.jsonl` (dual filesystem, crash-resilient).
- **No Child-Initiated Dial**: observability signals flow upward over the **existing** bidirectional connection the parent established to the child. Children never dial back to parents — this respects js-web and similar dial-only SDKs and sandboxed environments.
