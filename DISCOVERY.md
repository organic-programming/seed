# Holon Discovery Spec

Status: draft
⚠️ This document is the source of truth. Implementations may not be fully aligned yet. ⚠️

---

## `Discover` — scan all matches

⚠️ Not yet implemented across SDKs.

```
Discover(
  scope:      Scope,           // search context: LOCAL | PROXY | DELEGATED
  expression: string | null,   // slug | alias | uuid | path | direct-url | delegator-url, or null to return all
  root:       string | null,   // optional — search root, defaults to cwd
  specifiers: int,             // bitwise OR of Layer flags — valid range: 0x00–0x3F; 0 is treated as all
  limit:      uint,            // max results — 0 = no limit; stops scan early when reached
                               // languages without uint MUST return [] immediately for negative values
  timeout:    uint             // timeout in milliseconds — 0 = no timeout
) → DiscoverResult

// HolonInfo — proto-defined identity + metadata block, returned by Describe RPC.
//   SDK-autonomous: no dependency on op or .holon.json to obtain it.
//   op serializes HolonInfo → .holon.json at build time (op's responsibility, not the SDK's).
//   Fast path: read .holon.json and deserialize to HolonInfo.
//   Fallback: call Describe RPC — the holon returns HolonInfo directly.
//   Schema defined in PACKAGE.md.

// Transport URLs (passed as direct `expression`, or returned in `HolonRef.url`):
//   file://   local holon (installed, built, cached, source, siblings)
//   unix://   running holon exposed over a Unix domain socket
//   tcp://    running holon exposed over raw TCP
//   http://   running holon exposed over HTTP (REST+SSE transport)
//   https://  running holon exposed over HTTPS (REST+SSE transport)
//   ws://     running holon exposed over WebSocket
//   wss://    running holon exposed over secure WebSocket
//   <relay>   relayed URL from a target proxy — see Proxy Delegation below

Layer flags:
  siblings  = 0x01
  cwd       = 0x02
  source    = 0x04
  built     = 0x08
  installed = 0x10
  cached    = 0x20
  all       = 0x3F   // convenience alias for siblings | cwd | source | built | installed | cached

// Any value > 0x3F has bits with no defined meaning — implementations MUST return an invalid range error.

Scope flags:
  LOCAL     = 0   // Discover directly on this machine, return raw transport URLs
  PROXY     = 1   // Discover directly on this machine, but transpose URLs for remote caller
  DELEGATED = 2   // The expression is a delegator URL: dial it and forward the request with scope=PROXY


// Named result types:

HolonRef = {              // one discovered entry
  url:   string,
  info:  HolonInfo | null,  // null when Describe failed for this entry
  error: string | null      // entry-level error (e.g. "tcp: connection refused")
}

DiscoverResult = {
  found: []HolonRef,
  error: string | null      // operation-level error (e.g. invalid expression, invalid specifiers)
}

ResolveResult = {
  ref: HolonRef | null,
  error: string | null
}

ConnectResult = {
  channel:      channel | null,
  uid: string | null,   // UUID of the running process, if known (see INSTANCES.md)
  origin:       HolonRef | null, // the final resolved identity and transport URL
  error:        string | null
}
```


// Expression resolution by form:
//   slug | alias | uuid → scan layers, read .holon.json → HolonInfo, fallback: Describe
//   path | file://      → read .holon.json → HolonInfo, fallback: Describe
//   unix:// | tcp:// | ws:// | … (direct) → dial, call Describe → HolonInfo
//   <delegator-url>     → RPC to remote Discover, returns relayed url + HolonInfo


## `resolve` — first match

```
resolve(scope, expression, root, specifiers, timeout) → ResolveResult
// Discover(scope, expression, root, specifiers, limit=1, timeout).found[0]
// Returns ResolveResult with error set if nothing found or operation failed.
```

## `connect` — resolve + dial or launch

```
connect(scope, expression, root, specifiers, timeout) → ConnectResult
// 1. match = resolve(scope, expression, root, specifiers, timeout)
// 2. if match.error is set → return {channel: null,  uid: "" , origin: null, error: match.error}
// 3. if match.ref.url is reachable: 
//      dial it, return {channel: dialed,  uid: instance_uid  ,  origin: match.ref, error: null}
// 4. else if target is local and launchable (binary or source): 
//      launch the holon (which binds a new transport URL e.g. unix:// or tcp://)
//      dial it, return {channel: dialed,  uid: instance_uid  ,  origin: <updated-ref>, error: null}
// 5. else:
//      return {channel: null,  uid: ""  ,  origin: match.ref, error: "target unreachable"}
```

> **Definitions:**
> - **`reachable`**: A successful OS-level connection to the socket/port defined in `match.ref.url` (e.g., verifying a TCP `SYN/ACK` or opening a Unix domain socket without a "connection refused" error).
> - **`dialed channel`**: The raw transport stream (e.g., a TCP socket, Unix pipe, or WebSocket wrapped in the language's native Stream interface) returned to the SDK caller. It is purely the transport pipe. It has nothing to do with instance UIDs or holon identities—those are negotiated subsequently over this channel via the `Describe` RPC.

Usage examples (any language):

```
// SDKs export clarity constants for readability in positional calls:
// NO_LIMIT = 0, NO_TIMEOUT = 0, LOCAL = 0, PROXY = 1, DELEGATED = 2

Discover(LOCAL, null, null, all, NO_LIMIT, NO_TIMEOUT)
Discover(LOCAL, "gabriel-greeting-go", null, source | installed, NO_LIMIT, 5000)   // 5s timeout
Discover(DELEGATED, "wss://example.com/api/v1/op/Z2FicmllbC1ncmVldGluZy1nbw==", null, all, NO_LIMIT, NO_TIMEOUT)
Discover(DELEGATED, "https://example.com/api/v1/op/Z2FicmllbC1ncmVldGluZy1nbw==", null, all, NO_LIMIT, NO_TIMEOUT)
resolve(LOCAL, "gabriel-greeting-go", "/my/root", source, NO_TIMEOUT)              // first source match under root
connect(LOCAL, "gabriel-greeting-go", null, installed, 5000)                       // resolve + connect with timeout
```

> **Strict Universality**: The signatures above are strictly required across all 13 SDKs. Do not wrap them in language-specific syntax (e.g. no `Options` structs or Builder objects). To preserve readability in languages that lack named arguments, the SDK MUST export the default clarity constants (`NO_LIMIT`, `NO_TIMEOUT`, `LOCAL`, `REMOTE`) so that call sites remain perfectly symmetrical and self-documenting across all environments.

This algorithm is a **cross-SDK spec**.
Every SDK implements the same logic:
c, c++, c#, dart, go, java, js, js-web[^1], kotlin, python, ruby, rust, swift[^2].

---

## Expression Types

An `expression` can take any of these forms:

| Type | Example | Notes |
|---|---|---|
| **Slug** | `gabriel-greeting-go` | `given-family` lowercased |
| **Alias** | `op` | explicit `aliases` list in identity |
| **Dir name** | `gabriel-greeting-go.holon` | directory basename (`.holon` stripped) |
| **UUID** | `3f08b5c3` | identity, full or prefix |
| **Path** | `./holons/foo`, `.` | filesystem, no discovery walk |
| **Binary path** | `/path/to/binary` | direct file, bypasses discovery |
| **Direct URL** | `tcp://127.0.0.1:4000`, `wss://...` | `://` with `scope=LOCAL` → dials holon directly, skips discovery |
| **Proxy delegator** | `wss://proxy-domain/api/v1/op/<base64(key)>` | `://` with `scope=DELEGATED` → connects to proxy endpoint |

### Instance Targeting

Any `expression` (base address) can be constrained to a specific running process by appending `:<uid>`:
- `gabriel-greeting-go:ea346efb` (slug + UID)
- `op:ea346efb` (alias + UID)
- `wss://.../op/<base64(op:ea346efb)>` (delegator wrapping an instance expression)

See [INSTANCES.md](INSTANCES.md) for the process runtime and registry rules.

### Proxy Delegation

`Discover("<discoverer-url>/<base64-expression>", root, specifiers, ...)` issues a `Discover` RPC
to `<discoverer-url>`, which runs discovery on its side and returns a URL that
relays access back through the proxy.

> **Note**: The `<discoverer-url>` path preceding the `base64(expression)` is entirely arbitrary. It is defined by the remote proxy's routing configuration (e.g., `.../api/v1/op/`) and simply acts as the endpoint address.

```
1. Calling SDK asks proxy to perform discovery:
Discover(scope=DELEGATED, expression="wss://example.com/api/v1/op/Z2FicmllbC1ncmVldGluZy1nbw==", root=null, specifiers=all, ...)
                     └─────────────────────────────┘ └────────────────────────┘
                     target proxy discover endpoint      base64(expression)

2. Calling SDK extracts the endpoint, decodes the expression, and issues RPC:
→ connects to: wss://example.com/api/v1/op  (or unix:///var/run/op.sock, etc.)
→ sends payload: Discover(scope=PROXY, expression="gabriel-greeting-go", root=null, specifiers=all, ...)

3. Target proxy executes discovery locally, transposes local matches into relay URLs, and returns:
← DiscoverResult {
    found: [
      { url: "wss://example.com/api/v1/op-relay/Z2FicmllbC1ncmVldGluZy1nbw==", info: HolonInfo{...} }
    ]
  }
```

Detection: `scope: DELEGATED` triggers the client-side RPC, while `scope: PROXY` tells the target proxy to transpose its results.
The target proxy **must transpose** its locally discovered URLs to match the transport over which the `Discover` call arrived. For example, if the call arrived over `unix://`, a local directory match must be transposed and returned as `unix://<proxy-socket>/relay/...`.
The return value is still `DiscoverResult` — fully transparent to the caller.

Passive relay vs active launcher behaviour is governed by `op proxy` (see `PROXY.md`, forthcoming).

---

## What Discovery Finds

| Kind | Marker | Has binary? | Has `holon.proto`? |
|---|---|---|---|
| **Source holon** | `holon.proto` in tree | after build | yes |
| **`.holon` package** | `*.holon/` dir + `.holon.json` | yes (prebuilt) | no |

---

## Local Resolution Layers

Ordered by priority (first match wins):

| # | Layer | Location | Code API Specifier |
|---|---|---|---|
| 1 | **Siblings** | co-located holons (e.g. app bundle) | `siblings` (`0x01`) |
| 2 | **CWD** | current working directory tree | `cwd` (`0x02`) |
| 3 | **Source** | `holon.proto` walk from root | `source` (`0x04`) |
| 4 | **Built** | `.op/build/*.holon/` | `built` (`0x08`) |
| 5 | **Installed** | `$OPBIN/*.holon/` | `installed` (`0x10`) |
| 6 | **Cached** | `$OPPATH/cache/**/*.holon/` | `cached` (`0x20`) |

When multiple specifiers are given in the bitmask (e.g., `source | installed`), the resolution uses the **default fixed order** filtered to only the requested layers.

### `root` parameter

The `root` argument overrides the base search directory for layers that anchor to a root context (like `source`).

- When an explicit path is provided, the search anchors to this path.
- When `null`, the root defaults to the caller's true current working directory.
- Implementations MUST reject empty strings or invalid paths with an error.

*Note: For `op` CLI behavior and specific flag mappings (`--root`, `--source`), see [OP_DISCOVERY.md](holons/grace-op/OP_DISCOVERY.md).*

---

## Scan Rules

### Filesystem Search (`cwd`, `built`, `installed`, `cached`)

For layers resolving to pre-packaged holons, every SDK implements the following native walk mechanism:
1. Walk the target directory recursively (e.g. `root`, `$OPBIN`, `.op/build/`)
2. Match directories ending in `.holon/`
3. Read `.holon.json` inside it (fast path) or probe the binary via stdio `Describe` (fallback)

### Source Search (`source`)

Because finding and parsing `holon.proto` securely requires a full Protobuf compiler, the `source` layer is handled fundamentally differently:
- **Exclusive to `go-holons` SDK**: Walks the `root` tree natively, reading and parsing `holon.proto` definitions over the given directories.
- **Other SDKs**: Never walk the local tree for `holon.proto` directly. Instead, if a `source` layer is requested in the bitmask, the SDK natively **offloads** the entire search to the local `op` daemon. It calls `connect(LOCAL, "op", ...)` to spawn or dial `op`, and issues a local RPC `Discover(scope=LOCAL, ...)` against it. The SDK merges the RPC results with any native layer searches it performed.
  > *Note: This local offloading is unrelated to the `DELEGATED` scope network proxying. The SDK issues the RPC with `scope=LOCAL` because `op` shares the exact same filesystem and must return the raw `file://` transport URLs for the source holons.*

### Exclusions

Source tree walking skips: `.git`, `.op`, `node_modules`, `vendor`, `build`, `testdata`, and any directory starting with `.` (except `*.holon` package directories).

### `.holon.json` as Accelerator

- Generated by `op` at build time, never hand-edited
- Not a hard requirement — if missing, the SDK probes via stdio `Describe`
- Only `op` writes `.holon.json`; SDKs read or probe

---

## SDK Runtime Discovery

> **Reference Implementation**: `go-holons` is the definitive reference implementation for this spec. All other SDKs must replicate its discovery algorithm exactly, with one strict exception: the `source` layer parsing natively belongs exclusively to Go, and other SDKs must delegate it via RPC.

Every SDK implements `Discover(scope, expression, root, specifiers...)` independently[^2]. Same signature, same algorithm, no dependency on `op` at runtime (except for `source` discovery).

**Default root:** cwd of the running process. An organism can override this — e.g. an app defining a plugin directory as root.

**Default specifiers:** `--cwd --siblings --installed --cached` (no `--source` — running holons don't need source code).

**Siblings resolution:** the [organism kit's](organism_kits/README.md) responsibility, not the SDK's. The organism kit knows where its members live and provides that context to the SDK's `discover`:

| Mode | Siblings location | Who provides the root |
|---|---|---|
| **Published app** | `Contents/Resources/Holons/*.holon/` (macOS), executable-relative (Linux) | Organism kit |
| **Development** | Source tree sibling directories | Organism kit delegates to `op` |

An organism can define custom roots for plugin-style discovery — the `root` parameter in `Discover()` makes this standard.

[^1]: `js-web` runs in a browser — no filesystem access. `Discover` is limited to network-reachable schemes only (`ws://`, `wss://`, `https://`, relay). `file://`, `unix://`, and `tcp://` are not available.
[^2]: See [sdk/README.md](sdk/README.md) — `Discover`, `resolve`, and `connect` are part of every SDK's contract.
