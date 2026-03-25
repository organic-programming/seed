# COAX Interaction Surface for Composite App Holons

> **Scope:** Modeling and R&D only — no implementation at this stage.

## The Gap

A composite app holon (e.g. `gabriel-greeting-app-swiftui`) is today a **build recipe only** — 12 members, a build target, a `.app` output. It has **no contract, no service, no interaction surface**. A human can use the UI; no agent, script, or holon can drive it. This violates COAX.

## Resolved Decisions

| Decision | Answer |
|----------|--------|
| What is a composite app? | An **Organism** — the living whole that assembles its member holons |
| Surface name | **COAX interaction surface** (not "automation" — interaction is broader) |
| Proto terminology | **Members / holons** (not "organs" — clearer in proto context) |
| Shared vs. local proto | Shared: `_protos/holons/v1/coax.proto` |
| Recursive? | **Yes** — an organism's members can themselves be organisms, and COAX services apply recursively at every level |
| Tell/Relay | **Yes, it should exist** — applied at the organism level, not tied to any platform |
| Scope | **Modeling only** — proto design, document updates. No code. |

## The COAX Interaction Surface

### Why This Generalizes AppleScript

AppleScript gave apps a scriptable dictionary — platform-locked, stringly-typed, macOS-only. The COAX interaction surface achieves the same goal through proto contracts:

- **Universal** — proto-based, any language, any platform
- **Typed** — full protobuf messages, not string dictionaries
- **Self-describing** — [Describe](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/describe.proto#11-13) exposes it like any holon service
- **Composable** — other holons `connect(slug)` to the organism and interact
- **Recursive** — an organism within an organism inherits the same surface

### Open Design Question: Agent–Human Co-Interaction

> When an agent launches the app, it connects via stdio or tcp. But how?
> Can a human launch an app and an agent co-interact with the same running instance?

This is the critical question. Possible models:

1. **Separate channels** — human uses UI, agent connects via a second transport (tcp alongside the UI). Both interact concurrently.
2. **Agent-as-user** — agent drives the same UI surface programmatically (accessibility-style).
3. **Shared state** — both human and agent observe and mutate the same `HolonPickerModel` / organism state through the COAX surface.

Model 3 is the most aligned with COAX philosophy: same state, same contracts, two audiences.

## Proto Model Sketch

### Layer 1 — Member Management (`coax.proto`, shared)

```protobuf
// _protos/holons/v1/coax.proto
syntax = "proto3";
package holons.v1;

// CoaxService is the COAX interaction surface for any organism.
// It enables programmatic discovery, connection, and interaction
// with the organism's member holons — the same operations a human
// performs through the UI.
//
// This service is recursive: a member that is itself an organism
// exposes its own CoaxService.
service CoaxService {
  // List available member holons.
  rpc ListMembers(ListMembersRequest) returns (ListMembersResponse);

  // Connect a member holon (start it if needed).
  rpc ConnectMember(ConnectMemberRequest) returns (ConnectMemberResponse);

  // Disconnect a member holon.
  rpc DisconnectMember(DisconnectMemberRequest) returns (DisconnectMemberResponse);

  // Query the status of a member holon.
  rpc MemberStatus(MemberStatusRequest) returns (MemberStatusResponse);

  // Tell: forward a command to a member holon by slug.
  // The organism resolves, connects if needed, and relays.
  rpc Tell(TellRequest) returns (TellResponse);
}
```

`Tell` is the generalized AppleScript `tell application "X" to do Y`:

```protobuf
message TellRequest {
  string member_slug = 1;   // which member to address
  string method = 2;        // fully qualified RPC method name
  bytes  payload = 3;       // JSON-encoded request
}

message TellResponse {
  bytes  payload = 1;       // JSON-encoded response
}
```

### Layer 2 — App-Specific Domain Verbs (per organism)

Each organism defines its own domain vocabulary. For Gabriel Greeting App:

```protobuf
// gabriel-greeting-app-swiftui/api/v1/holon.proto
service GreetingAppService {
  // Select which greeting holon to use
  rpc SelectHolon(SelectHolonRequest) returns (SelectHolonResponse);

  // Select a language
  rpc SelectLanguage(SelectLanguageRequest) returns (SelectLanguageResponse);

  // Greet using current selection
  rpc Greet(GreetRequest) returns (GreetResponse);
}
```

These are high-level domain verbs — the actions that make sense for *this* organism. They abstract over the member plumbing. An agent calling `SelectHolon` + `SelectLanguage` + `Greet` achieves the same result as a human clicking the dropdown, picking a language, and typing a name.

## Impact on Documents

| Document | Change |
|----------|--------|
| `_protos/holons/v1/coax.proto` | **[NEW]** Shared COAX interaction proto |
| [apps_kits/DESIGN.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/apps_kits/DESIGN.md) | Add section linking App Kit primitives to the COAX proto surface |
| [CONSTITUTION.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/CONSTITUTION.md) | Already done (COAX in Article 1) |
| Gabriel app [holon.proto](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/examples/hello-world/gabriel-greeting-go/api/v1/holon.proto) | Add contract + service declaration (modeling only) |
| [manifest.proto](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/_protos/holons/v1/manifest.proto) | No changes needed — `contract` field already supports this |

## Next Steps (Modeling)

1. Draft `coax.proto` with full message definitions
2. Draft Gabriel app service proto (domain verbs)
3. Update [DESIGN.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/apps_kits/DESIGN.md) with the organism/COAX surface section
4. Document the co-interaction question in [DESIGN.md](file:///Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/apps_kits/DESIGN.md) as an open design topic
