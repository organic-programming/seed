import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as obs;
import 'package:holons_app/holons_app.dart';

import 'src/app.dart';
import 'src/controller/greeting_controller.dart';
import 'src/model/app_model.dart';
import 'src/rpc/greeting_app_service.dart';
import 'src/runtime/describe_registration.dart';
import 'src/runtime/greeting_holon_connection.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Cross-SDK observability bootstrap: reads OP_OBS from the env the
  // launcher injected (or the app's own overrides). See
  // OBSERVABILITY.md §Activation. Safe no-op when OP_OBS is empty.
  try {
    obs.checkEnv();
  } catch (e) {
    // Fail-fast per spec §Layer 3 (otel v2 or unknown token).
    // ignore: avoid_print
    print('OP_OBS misconfigured: $e');
    rethrow;
  }
  final observability = obs.fromEnv(const obs.Config(
    slug: 'gabriel-greeting-app',
    defaultLogLevel: obs.Level.info,
  ));
  observability.emit(obs.EventType.instanceSpawned,
      payload: const {'runtime': 'flutter'});
  final appLog = observability.logger('app');
  appLog.info('Flutter main starting');
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
