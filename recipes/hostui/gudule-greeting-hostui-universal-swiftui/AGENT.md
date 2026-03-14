# Universal SwiftUI HostUI — Agent Notes

This document is a short onboarding note for agents and developers working on
`gudule-greeting-hostui-universal-swiftui`.

---

## What this project is

The Universal SwiftUI HostUI is a macOS SwiftUI application for the Gudule
greeting demos.

Its purpose is to provide **one single SwiftUI app** that can connect to
**multiple greeting daemons** implemented in different languages such as Go,
Rust, Swift, C++, Python, and others.

This project exists alongside the non-universal SwiftUI HostUI because the two
serve different goals:

- `gudule-greeting-hostui-swiftui` is the simpler single-daemon version.
- `gudule-greeting-hostui-universal-swiftui` is the developer-facing version
  used to switch between daemon implementations from one UI.

## How it works

The app exposes:

- a daemon selector
- a transport selector
- a language selector
- a greeting input/output UI

When the user selects a daemon, the app:

1. discovers the available daemon binaries using known build locations
2. stages a temporary holon manifest for the selected daemon
3. connects through `connect(slug)`
4. loads languages through `greeting.v1.GreetingService/ListLanguages`
5. sends greetings through `greeting.v1.GreetingService/SayHello`

The selected daemon slug is displayed in the UI under the daemon selector.
That slug is used as a visible proof of which daemon the app is currently using.

## Failure behavior

Some daemons are still experimental or not yet fully working.

When a daemon is unavailable, the UI is expected to:

- show a connecting state
- fail more cleanly than before
- avoid freezing the UI while waiting for the error
- avoid re-showing an old greeting after a failed switch

Not every daemon/transport combination is guaranteed to work yet.

## Build model

There are two main build modes:

- `op build gudule-greeting-hostui-universal-swiftui`
  Rebuilds only the Universal SwiftUI HostUI.
- `op build gudule-greeting-universal-swiftui`
  Rebuilds the full universal app bundle and its daemon members.

For fast local iteration, build directly from this directory:

```sh
swift build
```

Run the Swift binary:

```sh
.build/debug/GreetingSwiftUI
```

If you built the HostUI recipe with `op build gudule-greeting-hostui-universal-swiftui`,
run:

```sh
./.build/xcode/macos/Build/Products/Debug/GreetingSwiftUI
```

If you built the full universal assembly with
`op build gudule-greeting-universal-swiftui`, run the packaged app with:

```sh
open ../../assemblies/gudule-greeting-universal-swiftui/build/GreetingSwiftUI.app
```

## Main files

- `README.md`
- `GreetingSwiftUI/ContentView.swift`
- `GreetingSwiftUI/DaemonProcess.swift`
- `GreetingSwiftUI/GreetingClient.swift`
- `GreetingSwiftUI/GreetingSwiftUIApp.swift`
- `holon.yaml`
- `Protos/greeting.proto`
- `../../assemblies/gudule-greeting-universal-swiftui/holon.yaml`

## Recommended reading order

1. `README.md`
2. `GreetingSwiftUI/ContentView.swift`
3. `GreetingSwiftUI/DaemonProcess.swift`
4. `GreetingSwiftUI/GreetingClient.swift`
5. `../../assemblies/gudule-greeting-universal-swiftui/holon.yaml`

## Practical summary

Use this HostUI when you want to test the same greeting contract against
multiple daemon implementations from one macOS SwiftUI app.

For day-to-day development:

- use `swift build` for the fastest loop on the UI itself
- use `op build gudule-greeting-hostui-universal-swiftui` when you want the
  HostUI recipe build
- use `op build gudule-greeting-universal-swiftui` when you want the full app
  bundle with the broader universal assembly flow
