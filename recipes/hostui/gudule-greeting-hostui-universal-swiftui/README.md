# Gudule Greeting HostUI Universal SwiftUI

Standalone SwiftUI HostUI for the shared `greeting.v1.GreetingService`.

This recipe is the SwiftUI front-end used by the universal Gudule app on macOS.
It can talk to multiple greeting daemons and switch between them from a single UI.

## Build the full universal app

From the `organic-programming` repository root:

```sh
op build gudule-greeting-universal-swiftui
```

This rebuilds the universal assembly, including the SwiftUI app and all daemon members.

## Run the full universal app

From the `organic-programming` repository root:

```sh
op run gudule-greeting-universal-swiftui
```

Or open the generated app bundle directly:

```sh
open recipes/assemblies/gudule-greeting-universal-swiftui/build/GreetingSwiftUI.app
```

## Rebuild only the SwiftUI HostUI

For a faster iteration loop, rebuild only this HostUI without rebuilding the daemons:

```sh
op build gudule-greeting-hostui-universal-swiftui
```

The built binary is written to:

```sh
recipes/hostui/gudule-greeting-hostui-universal-swiftui/.build/xcode/macos/Build/Products/Debug/GreetingSwiftUI
```

You can run it directly:

```sh
recipes/hostui/gudule-greeting-hostui-universal-swiftui/.build/xcode/macos/Build/Products/Debug/GreetingSwiftUI
```

## Fast local Swift-only loop

From the HostUI recipe directory:

```sh
cd ../../hostui/gudule-greeting-hostui-universal-swiftui
swift build
.build/debug/GreetingSwiftUI
```

This is usually the quickest way to iterate on SwiftUI layout and interaction changes.

## Run after build

If you built the HostUI with `swift build`, run:

```sh
.build/debug/GreetingSwiftUI
```

If you built the HostUI with `op build gudule-greeting-hostui-universal-swiftui`, run:

```sh
./.build/xcode/macos/Build/Products/Debug/GreetingSwiftUI
```

If you built the full universal app with `op build gudule-greeting-universal-swiftui`, open the packaged app:

```sh
open ../../assemblies/gudule-greeting-universal-swiftui/build/GreetingSwiftUI.app
```

## Notes

- The universal assembly build (`gudule-greeting-universal-swiftui`) rebuilds all daemon members.
- The HostUI build (`gudule-greeting-hostui-universal-swiftui`) rebuilds only the SwiftUI app.
- The current development flow expects the repository checkout to be present so the app can discover sibling daemon builds during local development.
