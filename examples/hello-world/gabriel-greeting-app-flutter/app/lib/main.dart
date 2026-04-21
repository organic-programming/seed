import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:grpc/grpc.dart';
import 'package:holons_app/holons_app.dart';

import 'src/app.dart';
import 'src/controller/greeting_controller.dart';
import 'src/model/app_model.dart';
import 'src/rpc/greeting_app_service.dart';
import 'src/runtime/describe_registration.dart';
import 'src/runtime/greeting_holon_connection.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final coaxDefaults = CoaxSettingsDefaults.standard(
    socketName: 'gabriel-greeting-coax.sock',
  );
  final settingsStore = await FileSettingsStore.create(
    applicationId: 'gabriel-greeting-app-flutter',
    applicationName: 'Gabriel Greeting App Flutter',
  );
  await applyLaunchEnvironmentOverrides(settingsStore, defaults: coaxDefaults);

  final greetingController = GreetingController(
    holons: BundledHolons<GabrielHolonIdentity>(
      fromDiscovered: GabrielHolonIdentity.fromDiscovered,
      slugOf: (holon) => holon.slug,
      sortRankOf: (holon) => holon.sortRank,
      displayNameOf: (holon) => holon.displayName,
    ),
    connector: BundledGreetingHolonConnectionFactory(),
  );

  late final CoaxManager coaxManager;
  coaxManager = CoaxManager(
    settingsStore: settingsStore,
    defaults: coaxDefaults,
    serviceFactory: () => <Service>[
      CoaxRpcService(
        holonManager: greetingController,
        coaxManager: coaxManager,
      ),
      GreetingAppRpcService(greetingController),
    ],
    prepareDescribe: () async {
      ensureAppDescribeRegistered();
    },
  );

  runApp(
    GabrielGreetingApp(
      greetingController: greetingController,
      coaxManager: coaxManager,
    ),
  );
}
