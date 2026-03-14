# Design Note: `mem://` Ping-Pong Validation via `op`

## Purpose

v0.5a is the proof milestone for **operator-launched in-process
composition**.

The goal is not to invent a new transport. The goal is to **verify**
that, after v0.5 transport completion, **`op` can launch a
language-specific mem-to-mem holon flow** in a repeatable,
language-native way.

Unit tests already show pieces of `mem://` behavior in several SDKs.
The examples must now show the full developer-facing story:

- `op` launch path
- same-language composition
- same-process composition
- SDK `connect(slug)` composition
- deterministic completion
- structured timing output

## SDK Rule

Every language implementation under `examples/mem-ping-pong/` must be
a **real SDK example**.

That means:

- it must import and use the matching language SDK directly
- it must rely on the SDK for `serve`, `transport`, `discover`, and
  `connect` behavior as applicable
- it must not regress into a raw gRPC-only baseline

If the example reveals that the SDK is missing required `mem://`,
`connect(slug)`, discovery, or lifecycle behavior, **the SDK is in
scope to modify** as part of the task.

The rule is:

> fix the SDK, then keep the example thin.

The example is the validation surface. It should not carry private
transport hacks or one-off workarounds that bypass the SDK contract.

## Scope

This design applies to one shared example root:

- `examples/mem-ping-pong/`

That root contains one language implementation per supported native SDK:

- `examples/mem-ping-pong/c/`
- `examples/mem-ping-pong/cpp/`
- `examples/mem-ping-pong/csharp/`
- `examples/mem-ping-pong/dart/`
- `examples/mem-ping-pong/go/`
- `examples/mem-ping-pong/java/`
- `examples/mem-ping-pong/js/`
- `examples/mem-ping-pong/kotlin/`
- `examples/mem-ping-pong/python/`
- `examples/mem-ping-pong/ruby/`
- `examples/mem-ping-pong/rust/`
- `examples/mem-ping-pong/swift/`

v0.5a is a **functional validation** milestone with timing output,
not a comparative benchmark program.

## Reference Implementation

Go is the v0.5a **reference implementation**.

The Go example is built first and establishes the canonical:

- example structure
- `op` launch contract
- SDK import pattern
- RPC semantics
- JSON result format
- timing fields
- test expectations

All other language examples follow that validated shape, translated
idiomatically into their own SDK and build system.

## Required Composition Model

Each example must host **two logical holons** in the same language:

- `ping`
- `pong`

Both logical holons must live inside the **same OS process**.
That is required because `mem://` is an in-process transport.

Each logical holon exposes the same canonical RPC behavior and each
side calls the other through the SDK's normal **`connect(slug)`**
primitive or the closest official language-level equivalent.

> `mem://` validation is only valid if the example uses the SDK
> composition path. A raw gRPC dial, direct function call, or
> transport-specific bypass does not satisfy this milestone.

## Canonical Party Algorithm

To remove ambiguity, v0.5a defines the ping-pong "party" in terms of
**turns**, not round-trips.

### Initial state

- `initial_value = 0`
- `turn_limit = 1000`

### One turn

One turn is exactly one holon doing all of the following:

1. receive the current value
2. increment it by `1`
3. decrement the remaining turn count by `1`
4. either forward to the peer or terminate

### Termination rule

If the incrementing holon reduces `remaining_turns` to `0`, the party
stops immediately and returns the terminal result.

### Expected final value

Because the party performs **1000 total turns**, starting from `0`,
the expected final value is:

```text
1000
```

This is intentionally **not** a "1000 round-trip" benchmark.
If each round-trip counted as two turns, the final value would be
`2000`, but that is not the definition used here.

## Canonical RPC Shape

Each language may name its generated files idiomatically, but the
example behavior must match this abstract contract:

```text
service PingPongService {
  rpc Bounce(BounceRequest) returns (BounceResult);
}

message BounceRequest {
  int32 value = 1;
  int32 remaining_turns = 2;
  string peer_slug = 3;
  string party_id = 4;
}

message BounceResult {
  int32 final_value = 1;
  int32 completed_turns = 2;
  string finished_by = 3;
}
```

Languages may add metadata fields if useful, but they must preserve
the semantics above.

## Startup Model

Each language implementation must provide one operator-facing launcher
entry point for the developer.

That launcher must be reachable through `op`.

That launcher:

1. is launched by `op`
2. starts the logical `ping` holon on a `mem://` listener
3. starts the logical `pong` holon on a `mem://` listener
4. ensures both are discoverable by slug to the same-language
   `connect(slug)` path
5. triggers the first call with `value = 0` and `remaining_turns = 1000`
6. waits for completion
7. prints one final report to stdout

The internal implementation is flexible:

- one executable with two embedded listeners is acceptable
- one language-native workspace with multiple targets is acceptable
- helper packages shared with the existing hello-world example are acceptable

But the runtime shape must still be **one `op`-launched process, two
logical holons, `mem://`, and `connect(slug)`**.

## Structured Timing Output

Each example must print exactly one final machine-readable report when
the party completes successfully.

The canonical output format is JSON on stdout:

```json
{
  "status": "ok",
  "language": "go",
  "example": "mem-ping-pong",
  "transport": "mem://",
  "initial_value": 0,
  "turn_limit": 1000,
  "final_value": 1000,
  "elapsed_ns": 1234567
}
```

### Output rules

- `elapsed_ns` must be measured with a monotonic clock
- `transport` must be exactly `mem://`
- `final_value` must be exactly `1000`
- `status` must be `ok` on success
- failed runs should exit non-zero and may emit an error object or a
  textual error, as long as success output remains canonical

Extra fields are allowed, but the canonical fields above must remain
present and stable.

## Testing Requirements

Every example must ship with:

- a language-native automated test
- a README with build, run, and test steps
- a documented `op` launch command

The automated test must verify at least:

- success exit status
- success through the documented `op` launch path
- `transport == "mem://"`
- `final_value == 1000`
- `turn_limit == 1000`

## Acceptance Standard

A language implementation satisfies v0.5a only if all of the following
are true:

- it lives under `examples/mem-ping-pong/`
- it is same-language end to end
- it is launchable through `op`
- it imports and uses the matching language SDK
- it uses `mem://`
- it uses SDK `connect(slug)` composition
- it completes exactly 1000 turns from initial value 0
- it prints the canonical structured result
- it has a runnable test and README
