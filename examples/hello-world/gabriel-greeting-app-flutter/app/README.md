# gabriel_greeting_app_flutter

Gabriel greeting Flutter desktop app.

## Toolchain and verification

Use Flutter **3.50.0** (stable), including Dart **3.14.0**, or newer. The CI
build uses Flutter 3.50.0. Use the `dart` executable bundled with that Flutter
SDK when running Dart commands for this app.

The macOS runner targets macOS **12.0** or newer and uses Swift Package
Manager for native plugins. CocoaPods is no longer required.

The app uses `shadcn_flutter` 0.0.55 with `MaterialShadcnApp` from
`shadcn_flutter_material`. The shared kit and the app import Material widgets
from `package:material_ui/material_ui.dart`.

From this directory:

```sh
flutter pub get
flutter analyze lib test integration_test
flutter test test
flutter test -d macos integration_test/greeting_smoke_test.dart
flutter build macos --debug
```

On Linux or Windows, replace both `macos` targets with `linux` or `windows`.
For a complete app with bundled member holons, run
`op build gabriel-greeting-app-flutter --hardened` from the repository root.

Also run the shared kit's checks from `organism_kits/flutter`:

```sh
flutter pub get
flutter analyze lib test
flutter test
```

## Member holons

The app discovers and drives the same Gabriel greeting member set as the other
composite examples: C, C++, C#, Dart, Go, Java, Kotlin, Node, Python, Ruby, Rust,
Swift, and Zig (`gabriel-greeting-zig`).
