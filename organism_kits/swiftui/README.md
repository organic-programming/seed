# HolonsApp

`HolonsApp` is the reusable SwiftUI organism kit for COAX-enabled app holons.
It owns the reusable COAX runtime and the reusable SwiftUI COAX UI:

- `CoaxServer`
- `CoaxServiceProvider`
- `OrganismState`
- `CoaxControlsView`
- `CoaxSettingsView`

`sdk/swift-holons` remains the pure Swift SDK. This package is the SwiftUI
layer on top of that SDK.

## Fast Path: Henri Nobody

From the repository root:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed
op new --template coax-swiftui henri-nobody
```

Then verify the generated Swift package:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/henri-nobody/Modules
swift build
```

The scaffold gives you a working SwiftUI app holon with:

- the same reusable COAX controls and settings sheet used by the Gabriel app
- full `holons.v1.CoaxService` support
- a demo member `holon` already wired through shared UI state so `Tell` works
- a placeholder `holon/` directory to replace with your real business holon

## Generated Tree

```text
henri-nobody/
├── api/v1/holon.proto
├── holon/README.md
├── project.yml
├── App/
│   ├── HenriNobodyApp.swift
│   ├── ContentView.swift
│   ├── Info.plist
│   └── HenriNobody.entitlements
├── Modules/
│   ├── Package.swift
│   └── Sources/AppKit/
│       ├── .gitkeep
│       └── HolonProcess.swift
└── .op/build/.gitkeep
```

## What The Scaffold Already Wires

- `App/{{PascalSlug}}App.swift` creates:
  - `CoaxServer`
  - `CoaxServiceProvider`
  - a Describe registration based on `api/v1/holon.proto`
- `App/ContentView.swift` mounts:
  - `CoaxControlsView`
  - `CoaxSettingsView`
  - a local demo organism model
- `Modules/Sources/AppKit/HolonProcess.swift` exposes one member slug: `holon`
- the local demo model implements all six COAX RPC behaviors

The COAX server remains opt-in:

- off by default
- persisted under `coax.server.enabled`
- overridden by `OP_COAX_SERVER_ENABLED`
- overridden by `OP_COAX_SERVER_LISTEN_URI`

## Verifying COAX Immediately

Run the app, enable COAX in the header, then exercise the demo member:

```sh
op tcp://127.0.0.1:60000 ListMembers
op tcp://127.0.0.1:60000 MemberStatus '{"slug":"holon"}'
op tcp://127.0.0.1:60000 ConnectMember '{"slug":"holon","transport":"stdio"}'
op tcp://127.0.0.1:60000 Tell '{"member_slug":"holon","method":"sample.v1.SampleHolon/SetGreeting","payload":"{\"text\":\"Hello from COAX\"}"}'
op tcp://127.0.0.1:60000 TurnOffCoax
```

Expected behavior:

- the COAX toggle, endpoint line, badge, and settings sheet stay kit-owned
- `Tell` updates the same visible greeting card used by the human UI
- `TurnOffCoax` answers first, then disables the server surface

## Replacing The Demo Member With A Real Holon

1. Replace `henri-nobody/holon/` with your real business holon.
2. Replace `HolonProcess` in `henri-nobody/Modules/Sources/AppKit/HolonProcess.swift`.
   Swap the local in-memory member bridge for your real connection logic.
3. Keep the COAX server integration unchanged in `App/{{PascalSlug}}App.swift`.
   The intended wiring remains:
   - `CoaxServer`
   - `CoaxServiceProvider`
   - your `OrganismState` implementation
4. Keep the COAX UI views unchanged in `App/ContentView.swift`.
   The supported public surface is:
   - `CoaxControlsView`
   - `CoaxSettingsView`

## Minimal Real-App Wiring Pattern

```swift
let holon = MyOrganismModel()
let coaxServerBox = WeakCoaxServerBox()
let server = CoaxServer(
    providers: {
        [
            CoaxServiceProvider(organism: holon, coaxServer: coaxServerBox.value!)
        ]
    },
    registerDescribe: {
        try registerDescribeResponse()
    },
    coaxDefaults: .standard(socketName: "henri-nobody-coax.sock")
)
Task { @MainActor [server] in
    server.startIfEnabled()
}
```

Your organism model should conform to `OrganismState`:

```swift
@MainActor
final class MyOrganismModel: ObservableObject, OrganismState {
    var coaxMembers: [CoaxMember] { ... }
    func connectCoaxMember(slug: String, transport: String) async throws -> CoaxMember { ... }
    func disconnectCoaxMember(slug: String) async { ... }
    func tellCoaxMember(slug: String, method: String, payloadJSON: Data) async throws -> Data { ... }
}
```

## UI Integration Contract

Keep the generated SwiftUI surface:

```swift
CoaxControlsView(
    coaxServer: coaxServer,
    isShowingSettings: $isShowingCoaxSettings
)
.sheet(isPresented: $isShowingCoaxSettings) {
    CoaxSettingsView(
        coaxServer: coaxServer,
        isPresented: $isShowingCoaxSettings
    )
}
```

That is the supported SwiftUI kit surface. The Gabriel app already uses the
same views, so preserving that integration preserves the visible COAX behavior.

## Hardened Builds

`project.yml` emits the literal build-template guard:

```text
{{ if .Hardened }}
CODE_SIGN_ENTITLEMENTS: App/HenriNobody.entitlements
{{ end }}
```

`op build --hardened` is a build-time concern only. Runtime COAX behavior does
not branch on hardened mode.

## Quick Checks

For a scaffolded app:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/henri-nobody/Modules
swift build
```

For the Gabriel reference app after kit changes:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/examples/hello-world/gabriel-greeting-app-swiftui/Modules
swift test
```
