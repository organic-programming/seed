import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;
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
    holons.checkEnv();
  } catch (e) {
    // Fail-fast per spec §Layer 3 (otel v2 or unknown token).
    // ignore: avoid_print
    print('OP_OBS misconfigured: $e');
    rethrow;
  }
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
  final observabilityKit = ObservabilityKit.standalone(
    slug: 'gabriel-greeting-app-flutter',
    declaredFamilies: const [
      holons.Family.logs,
      holons.Family.metrics,
      holons.Family.events,
      holons.Family.prom,
    ],
    settings: settingsStore,
    bundledHolons: await _observabilityMembers(greetingController),
  );
  final observability = observabilityKit.obs;
  greetingController.attachObservability(observability);
  observability.emit(
    holons.EventType.instanceSpawned,
    payload: const {'runtime': 'flutter'},
  );
  observability.logger('app').info('Flutter main starting');

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
      observabilityKit: observabilityKit,
    ),
  );
}

Future<List<ObservabilityMemberRef>> _observabilityMembers(
  GreetingController greetingController,
) async {
  try {
    final members = await greetingController.holons.list();
    return members
        .map(
          (member) => ObservabilityMemberRef(
            slug: member.slug,
            uid: member.slug,
            address: member.discoveryPath,
          ),
        )
        .toList(growable: false);
  } on Object {
    return const [];
  }
}
