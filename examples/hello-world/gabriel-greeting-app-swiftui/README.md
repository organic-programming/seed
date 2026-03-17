# Gabriel Greeting App SwiftUI

SwiftUI HostUI for the Gabriel greeting holons. The app presents the same dark, speech-bubble-driven UX as the existing Gudule SwiftUI recipe while targeting the Gabriel backends only.

Gabriel is a multilingual greeting service with 56 languages. This HostUI is the composite holon that discovers sibling Gabriel daemons, connects over the Holons transport layer, and renders the interactive greeting UI on top.

## Available backends

| Backend | Slug | Binary | Source location |
|---------|------|--------|-----------------|
| Swift | `gabriel-greeting-swift` | `gabriel-greeting-swift` | `../gabriel-greeting-swift/` |
| Go | `gabriel-greeting-go` | `gabriel-greeting-go` | `../gabriel-greeting-go/` |

## Transport support

The UI exposes the same transport picker as the reference recipe:

- `mem` for in-process embedding of the Swift backend
- `stdio`
- `unix`
- `tcp`
- `rest+sse`

The HostUI uses a staged holon root for `connect(slug)` compatibility. It copies Gabriel's proto identity (`holon.proto`) plus the shared `greeting.proto` so discovery and Describe can resolve the staged holon from proto metadata alone.

## Build

From the repository root:

```sh
op build gabriel-greeting-app-swiftui
```

Or build the package directly:

```sh
cd examples/hello-world/gabriel-greeting-app-swiftui
swift build
```

## Run

After building:

```sh
cd examples/hello-world/gabriel-greeting-app-swiftui
swift run GabrielGreetingApp
```

The app should reveal an 800x600 window titled "Gabriel Greeting", discover built sibling backends, load the language picker, and refresh the greeting automatically as you edit the name, switch languages, change transports, or switch daemons.

## Holon architecture

- `GabrielGreetingApp/ContentView.swift` mirrors the reference HostUI layout and interaction model.
- `GabrielGreetingApp/DaemonProcess.swift` discovers the Swift and Go sibling daemons, stages the holon root, and manages reconnects.
- `GabrielGreetingApp/EmbeddedSwiftMemDaemon.swift` embeds the Swift backend for `mem://`.
- `GabrielGreetingApp/GreetingMessages.swift` keeps the hand-coded protobuf message approach used by the reference recipe.
- `api/v1/holon.proto` gives the app its proto-based composite identity.
