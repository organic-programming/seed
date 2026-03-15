# Codex Prompt — Gabriel Greeting App (SwiftUI)

## Goal

Create `gabriel-greeting-app-swiftui`, a SwiftUI HostUI app that connects to the Gabriel greeting backends (Swift and Go **only** for now) and presents the same UX as the existing recipe at `recipes/hostui/gudule-greeting-hostui-universal-swiftui/`.

## Destination

`examples/hello-world/gabriel-greeting-app-swiftui/`

## Reference — existing recipe (READ FIRST)

The existing recipe is the source of truth for the UX. Read every file in:
`recipes/hostui/gudule-greeting-hostui-universal-swiftui/GreetingSwiftUI/`

Key files:
- `ContentView.swift` — Dark UI, speech bubble (LeftPointerBubble shape), language picker, name field, transport picker, daemon picker, status indicator.
- `DaemonProcess.swift` — Daemon lifecycle: discover → stage → connect → gRPC client. Multi-daemon discovery with descriptor pattern. Transport picker (mem/stdio/unix/tcp/rest+sse).
- `GreetingClient.swift` — gRPC client using `connect(slug)` from swift-holons SDK.
- `EmbeddedSwiftMemDaemon.swift` — In-process Swift daemon for `mem://` transport.
- `GreetingMessages.swift` — Hand-coded protobuf types (no protoc generation).
- `GreetingSwiftUIApp.swift` — App entry point, window lifecycle.

## What changes from the old recipe

### 1. Only Swift and Go backends

The old recipe discovers 12 language variants (Go, C++, Swift, Rust, C, C#, Dart, Java, Kotlin, Node, Python, Ruby) using the `gudule-daemon-greeting-*` naming convention.

The new app discovers exactly **2 backends**:

| Backend | Slug | Binary | Relative location |
|---------|------|--------|--------------------|
| **Go** | `gabriel-greeting-go` | `gabriel-greeting-go` | `../gabriel-greeting-go/` |
| **Swift** | `gabriel-greeting-swift` | `gabriel-greeting-swift` | `../gabriel-greeting-swift/` |

Daemon discovery must be adapted: instead of searching for `gudule-daemon-greeting-*` in `recipes/daemons/`, search for `gabriel-greeting-go` and `gabriel-greeting-swift` as **sibling directories** under `examples/hello-world/`.

### 2. Proto-based identity (no holon.yaml staging)

The old recipe stages a temporary `holon.yaml` for connect. The Gabriel holons use **proto-based identity** (`api/v1/holon.proto`). The staging logic must be adapted:
- Stage `holon.proto` instead of `holon.yaml` (or keep a minimal `holon.yaml` for connect compatibility if the SDK still requires it).
- Copy the shared `greeting.proto` from `../_protos/v1/greeting.proto`.

### 3. Package dependencies use local paths

```swift
// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "GabrielGreetingApp",
    platforms: [.macOS(.v15), .iOS(.v18)],
    dependencies: [
        .package(path: "../gabriel-greeting-swift"),          // Swift backend (for mem:// embedding)
        .package(path: "../../../sdk/swift-holons"),          // SDK
        .package(url: "https://github.com/grpc/grpc-swift.git", exact: "1.9.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.36.0"),
        .package(url: "https://github.com/apple/swift-protobuf.git", from: "1.35.0"),
    ],
    targets: [
        .executableTarget(
            name: "GabrielGreetingApp",
            dependencies: [
                .product(name: "Holons", package: "swift-holons", condition: .when(platforms: [.macOS])),
                .product(name: "GabrielGreetingServer", package: "gabriel-greeting-swift", condition: .when(platforms: [.macOS])),
                .product(name: "GRPC", package: "grpc-swift"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            path: "GabrielGreetingApp"
        ),
    ]
)
```

> `gabriel-greeting-swift` already exports two library products — `GabrielGreeting` (Code API + protobuf types) and `GabrielGreetingServer` (gRPC server + Serve integration). Use `GabrielGreetingServer` for mem:// embedding.

### 4. Naming — Gabriel, not Gudule

All identifiers, comments, and window titles must use "Gabriel" naming:
- `GabrielGreetingApp` (target, module, app name)
- `gabriel-greeting-app-swiftui` (slug)
- Window title: "Gabriel Greeting"

## UX — 100% identical

The UX must be **pixel-perfect identical** to the existing recipe:

1. **Dark background** (rgb 0.1, 0.1, 0.1), darker header (rgb 0.13).
2. **Top header**: daemon picker (dropdown) + slug display + transport picker (mem/stdio/unix/tcp/rest+sse) + status indicator (green/orange/red dot).
3. **Left column**: name input field (TextField, 300pt wide).
4. **Right column**: speech bubble (LeftPointerBubble shape with dashed stroke) showing greeting text (42pt), or loading spinner, or error state.
5. **Bottom center**: language picker (native + English name).
6. **Behavior**: typing in name field → auto-greet. Changing language → auto-greet. Changing transport → disconnect/reconnect. Changing daemon → disconnect/reconnect + reload languages.
7. **Minimum window**: 800×600, revealed and centered on launch.

## Structure

```
gabriel-greeting-app-swiftui/
├── Package.swift
├── GabrielGreetingApp/
│   ├── GabrielGreetingApp.swift       App entry (@main, WindowGroup, lifecycle)
│   ├── ContentView.swift              UI — port from recipe, identical UX
│   ├── DaemonProcess.swift            Lifecycle — simplified for 2 backends only
│   ├── GreetingClient.swift           gRPC client — same pattern
│   ├── EmbeddedSwiftMemDaemon.swift   In-process Swift daemon for mem://
│   └── GreetingMessages.swift         Protobuf types — same hand-coded approach
├── api/
│   └── v1/
│       └── holon.proto                Proto-based identity (new UUID)
└── README.md
```

## Proto manifest — `api/v1/holon.proto`

```protobuf
syntax = "proto3";
package greeting.v1;

import "holons/v1/manifest.proto";
import "v1/greeting.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "<GENERATE-NEW-UUID>"
    given_name: "Gabriel"
    family_name: "Greeting-App-SwiftUI"
    motto: "SwiftUI HostUI for the Gabriel greeting service."
    composer: "Codex"
    clade: "deterministic/pure"
    status: "draft"
    born: "2026-03-15"
  }
  lineage: {
    reproduction: "manual"
    generated_by: "manual"
  }
  description: "A standalone SwiftUI app that connects to Gabriel greeting holons (Go and Swift) and presents a multilingual greeting UI."
  kind: "composite"
  build: {
    runner: "recipe"
  }
  requires: {
    commands: ["xcodebuild"]
    files: ["Package.swift"]
  }
  artifacts: {
    primary: "build/GabrielGreetingApp.app"
  }
};
```

## DaemonProcess adaptation — key changes

### Daemon discovery (simplified)

Replace the 12-variant `supportedDaemons` array with exactly 2:

```swift
static let supportedDaemons: [GabrielDaemonDescriptor] = [
    GabrielDaemonDescriptor(
        variant: "swift",
        displayName: "Gabriel (Swift)",
        binaryName: "gabriel-greeting-swift",
        buildRunner: "swift-package",
        sortRank: 0,
        extraRelativePaths: [
            ".build/arm64-apple-macosx/debug/gabriel-greeting-swift"
        ]
    ),
    GabrielDaemonDescriptor(
        variant: "go",
        displayName: "Gabriel (Go)",
        binaryName: "gabriel-greeting-go",
        buildRunner: "go-module",
        sortRank: 1
    ),
]
```

### Source tree scanning

Adapt `sourceTreeDaemonRoots()` to scan sibling directories under `examples/hello-world/` rather than `recipes/daemons/`:

```swift
// Look for ../gabriel-greeting-go/ and ../gabriel-greeting-swift/ relative to the app
```

### Staging

Keep the `stageHolonRoot` approach for `connect(slug)`, but use the Gabriel slug and binary names. If the SDK still requires `holon.yaml`, generate a minimal one. Copy `greeting.proto` from `../_protos/v1/greeting.proto`.

### mem:// embedding

Only the Swift backend supports mem://. Import `GabrielGreetingServer` (from `gabriel-greeting-swift`) to embed the Swift daemon in-process.

## README.md

Adapt from the Gabriel Go holon's README style. Describe:
- What the app does (SwiftUI HostUI for Gabriel)
- Available backends (Swift and Go)
- Transport support (mem/stdio/unix/tcp)
- How to build (`op build gabriel-greeting-app-swiftui` or `swift build`)
- How to run (launch the app)
- The holon architecture (composite holon, proto-based identity)

## Validation

### Build

```bash
cd examples/hello-world/gabriel-greeting-app-swiftui && swift build
```

### Run

```bash
cd examples/hello-world/gabriel-greeting-app-swiftui
swift run GabrielGreetingApp
```

The app should:
1. Launch and show the dark UI
2. Discover at least one backend (Go or Swift, whichever is built)
3. Load 56 languages in the picker
4. Default to English, show "Hello Mary" in the speech bubble
5. Switch languages → greeting changes
6. Type a name → greeting updates live
7. Switch transport → reconnects
8. Switch daemon → reconnects with new backend

## Rules

- Do NOT modify any file outside `examples/hello-world/gabriel-greeting-app-swiftui/`.
- Do NOT modify the Go or Swift backend holons.
- Do NOT modify any shared proto files.
- Generate a new UUID for the holon identity.
- The UX must be visually identical to the existing recipe.
- Use the exact same LeftPointerBubble shape, colors, spacing, and font sizes.
- Keep all `#if os(macOS)` guards — the app is macOS-first but should not crash on iOS.
