# holons_app

`holons_app` is the reusable Flutter organism kit for COAX-enabled app holons.
It owns the reusable COAX runtime and the reusable Shadcn COAX UI:

- `CoaxController`
- `CoaxRpcService`
- `HolonCatalog` / `HolonConnector`
- `CoaxControlBar`
- `CoaxSettingsDialog`

`sdk/dart-holons` remains the pure Dart SDK. This kit is the Flutter layer on
top of that SDK.

## Fast Path: Henri Nobody

From the repository root:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed
op new --template coax-flutter henri-nobody
```

Then bootstrap the generated app member:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/henri-nobody/app
flutter pub get
flutter analyze lib
```

The scaffold gives you a working Flutter app holon with:

- the same reusable COAX header and settings dialog used by the Gabriel app
- full `holons.v1.CoaxService` support
- a demo member `holon` already wired through shared UI state so `Tell` works
- a placeholder `holon/` directory to replace with your real business holon

## Generated Tree

```text
henri-nobody/
├── api/v1/holon.proto
├── holon/README.md
├── app/
│   ├── pubspec.yaml
│   ├── lib/main.dart
│   ├── lib/src/app.dart
│   ├── tool/package_desktop.dart
│   └── macos/Runner/
│       ├── DebugProfile.entitlements
│       └── Release.entitlements
└── .gitignore
```

## What The Scaffold Already Wires

- `app/lib/main.dart` creates:
  - `FileSettingsStore`
  - `CoaxController`
  - `CoaxRpcService`
  - a Describe registration based on `api/v1/holon.proto`
- `app/lib/src/app.dart` mounts:
  - `CoaxControlBar`
  - `CoaxSettingsDialog`
  - a local demo organism controller
- the local demo controller exposes one member slug: `holon`
- the local demo controller implements all six COAX RPC behaviors

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

- the header toggle, endpoint, and settings UI stay kit-owned
- `Tell` updates the same visible greeting card used by the human UI
- `TurnOffCoax` answers first, then disables the server surface

## Replacing The Demo Member With A Real Holon

1. Replace `henri-nobody/holon/` with your real business holon.
2. Update `henri-nobody/app/tool/package_desktop.dart`.
   Add your real member slugs in `_memberSlugs`.
3. Replace the demo controller in `henri-nobody/app/lib/src/app.dart`.
   Swap the local in-memory member bridge for:
   - `DesktopHolonCatalog`
   - a real holon identity mapper
   - a real connector built on `ClientChannel`
4. Keep the COAX UI components unchanged.
   The app should still mount:
   - `CoaxControlBar`
   - `CoaxSettingsDialog`
5. Keep `CoaxController` as the owner of start/stop and settings persistence.

## Minimal Real-App Wiring Pattern

```dart
final settingsStore = await FileSettingsStore.create(
  applicationId: 'henri-nobody',
  applicationName: 'Henri Nobody',
);
await applyLaunchEnvironmentOverrides(settingsStore, defaults: coaxDefaults);

final organismController = MyOrganismController(
  catalog: DesktopHolonCatalog<MyHolonIdentity>(
    fromDiscovered: MyHolonIdentity.fromDiscovered,
    slugOf: (holon) => holon.slug,
    sortRankOf: (holon) => holon.sortRank,
    displayNameOf: (holon) => holon.displayName,
  ),
  connector: MyHolonConnectionFactory(),
);

late final CoaxController coaxController;
coaxController = CoaxController(
  settingsStore: settingsStore,
  defaults: coaxDefaults,
  serviceFactory: () => <Service>[
    CoaxRpcService(
      organismController: organismController,
      coaxController: coaxController,
    ),
  ],
  prepareDescribe: () async {
    final protoDir = findAppProtoDir();
    if (protoDir == null) {
      throw StateError('Could not locate api/v1/holon.proto');
    }
    holons.useStaticResponse(
      holons.buildDescribeResponse(protoDir: protoDir),
    );
  },
);
```

## UI Integration Contract

Keep the generated header wiring:

```dart
CoaxControlBar(
  controller: coaxController,
  onOpenSettings: () {
    showDialog<void>(
      context: context,
      builder: (_) => CoaxSettingsDialog(controller: coaxController),
    );
  },
)
```

That is the supported surface for the Flutter kit. The Gabriel app already uses
the same components, so preserving that integration preserves the visible COAX
behavior.

## Hardened Packaging

`app/tool/package_desktop.dart` preserves `_isHardenedBuild()` verbatim.

`OP_BUILD_HARDENED=true` only affects packaging-time signing. Runtime COAX
behavior does not branch on hardened mode. Runtime sandbox fallback remains in
`effectiveHolonTransport()`.

## Quick Checks

For a scaffolded app:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/henri-nobody/app
flutter analyze lib
```

For the Gabriel reference app after kit changes:

```sh
cd /Users/bpds/Documents/Entrepot/Git/Compilons/seed/examples/hello-world/gabriel-greeting-app-flutter/app
flutter analyze lib test integration_test
flutter test test
```
