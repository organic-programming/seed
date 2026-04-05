import 'dart:async';

import 'package:flutter/widgets.dart';

import 'src/app.dart';
import 'src/controller/coax_controller.dart';
import 'src/controller/greeting_controller.dart';
import 'src/runtime/holon_catalog.dart';
import 'src/runtime/holon_connector.dart';
import 'src/settings_store.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final settingsStore = await FileSettingsStore.create();
  await applyLaunchEnvironmentOverrides(settingsStore);
  final greetingController = GreetingController(
    catalog: DesktopHolonCatalog(),
    connector: DesktopHolonConnector(),
  );
  final coaxController = CoaxController(
    greetingController: greetingController,
    settingsStore: settingsStore,
  );

  runApp(
    GabrielGreetingApp(
      greetingController: greetingController,
      coaxController: coaxController,
    ),
  );
}
