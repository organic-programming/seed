import 'dart:async';
import 'dart:ui' show AppExitResponse;

import 'package:flutter/material.dart' show SwitchListTile, Tab;
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:holons/holons.dart' as holons;
import 'package:holons_app/holons_app.dart';

import 'package:gabriel_greeting_app_flutter/src/app.dart';
import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';

import 'support/fakes.dart';

void main() {
  test('assembles the root application widget', () {
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
            ],
            greetingBuilder: ({required name, required langCode}) =>
                'Hello $name',
          ),
        },
      ),
    );
    final coaxManager = buildCoaxManager(
      greetingController: greetingController,
    );
    final observabilityKit = buildObservabilityKit();
    addTearDown(observabilityKit.dispose);
    greetingController.attachObservability(observabilityKit.obs);

    final app = GabrielGreetingApp(
      greetingController: greetingController,
      coaxManager: coaxManager,
      observabilityKit: observabilityKit,
    );

    expect(app.greetingController, same(greetingController));
    expect(app.coaxManager, same(coaxManager));
  });

  testWidgets('select popups apply holon, runtime, and language changes', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1400, 1100);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    final connection = FakeGreetingHolonConnection(
      languages: [
        language(code: 'en', name: 'English', native: 'English'),
        language(code: 'fr', name: 'French', native: 'Francais'),
      ],
      greetingBuilder: ({required name, required langCode}) {
        return langCode == 'fr' ? 'Bonjour $name' : 'Hello $name';
      },
    );
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([
        holon('gabriel-greeting-swift'),
        holon('gabriel-greeting-go'),
      ]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (_) => connection,
          'gabriel-greeting-go': (_) => connection,
        },
      ),
      capabilities: const AppPlatformCapabilities(supportsUnixSockets: true),
    );
    final coaxManager = buildCoaxManager(
      greetingController: greetingController,
      capabilities: const AppPlatformCapabilities(supportsUnixSockets: true),
    );
    final observabilityKit = buildObservabilityKit();
    addTearDown(observabilityKit.dispose);
    greetingController.attachObservability(observabilityKit.obs);

    await tester.pumpWidget(
      GabrielGreetingApp(
        greetingController: greetingController,
        coaxManager: coaxManager,
        observabilityKit: observabilityKit,
      ),
    );
    await _settleApp(tester);

    expect(greetingController.selectedHolon?.slug, 'gabriel-greeting-swift');
    expect(greetingController.transport, 'stdio');
    expect(greetingController.selectedLanguageCode, 'en');

    await tester.tap(find.text('Gabriel (Swift)').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Gabriel (Go)').last);
    await _settleApp(tester);
    expect(greetingController.selectedHolon?.slug, 'gabriel-greeting-go');

    await tester.tap(find.text('stdio').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('tcp').last);
    await _settleApp(tester);
    expect(greetingController.transport, 'tcp');

    await tester.tap(find.text('English (English)').first);
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('Francais (French)').last);
    await tester.tap(find.text('Francais (French)').last);
    await _settleApp(tester);
    expect(greetingController.selectedLanguageCode, 'fr');
  });

  testWidgets('name field stays beside the bubble on a narrow window', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(920, 760);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
            ],
            greetingBuilder: ({required name, required langCode}) =>
                'Hello $name',
          ),
        },
      ),
    );
    final coaxManager = buildCoaxManager(
      greetingController: greetingController,
    );
    final observabilityKit = buildObservabilityKit();
    addTearDown(observabilityKit.dispose);
    greetingController.attachObservability(observabilityKit.obs);

    await tester.pumpWidget(
      GabrielGreetingApp(
        greetingController: greetingController,
        coaxManager: coaxManager,
        observabilityKit: observabilityKit,
      ),
    );
    await _settleApp(tester);

    final fieldRect = tester.getRect(
      find.byKey(const ValueKey<String>('name-input')),
    );
    final bubbleTextRect = tester.getRect(find.text('Hello World'));

    expect(fieldRect.right, lessThan(bubbleTextRect.left));
    expect(
      (fieldRect.center.dy - bubbleTextRect.center.dy).abs(),
      lessThan(80),
    );
  });

  testWidgets('app exit waits for controller shutdown before terminating', (
    tester,
  ) async {
    final closeCompleter = Completer<void>();
    final connection = FakeGreetingHolonConnection(
      languages: [language(code: 'en', name: 'English', native: 'English')],
      greetingBuilder: ({required name, required langCode}) => 'Hello $name',
      closeFuture: closeCompleter.future,
    );
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (_) => connection,
        },
      ),
    );
    final coaxManager = buildCoaxManager(
      greetingController: greetingController,
    );
    final observabilityKit = buildObservabilityKit();
    addTearDown(observabilityKit.dispose);
    greetingController.attachObservability(observabilityKit.obs);

    await tester.pumpWidget(
      GabrielGreetingApp(
        greetingController: greetingController,
        coaxManager: coaxManager,
        observabilityKit: observabilityKit,
      ),
    );
    await _settleApp(tester);

    var completed = false;
    late AppExitResponse response;
    final exitFuture = tester.binding.handleRequestAppExit().then((value) {
      response = value;
      completed = true;
    });

    await tester.pump();
    expect(connection.closed, isTrue);
    expect(completed, isFalse);

    closeCompleter.complete();
    await exitFuture;

    expect(completed, isTrue);
    expect(response, AppExitResponse.exit);
  });

  testWidgets('observability panel opens and reads kit state', (tester) async {
    tester.view.physicalSize = const Size(1400, 1100);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
            ],
            greetingBuilder: ({required name, required langCode}) =>
                'Hello $name',
          ),
        },
      ),
    );
    final coaxManager = buildCoaxManager(
      greetingController: greetingController,
    );
    final observabilityKit = buildObservabilityKit();
    addTearDown(observabilityKit.dispose);
    greetingController.attachObservability(observabilityKit.obs);
    await observabilityKit.gate.setMaster(false);
    await observabilityKit.gate.setFamily(holons.Family.logs, false);

    await tester.pumpWidget(
      GabrielGreetingApp(
        greetingController: greetingController,
        coaxManager: coaxManager,
        observabilityKit: observabilityKit,
      ),
    );
    await _settleApp(tester);

    await tester.tap(
      find.byKey(const ValueKey<String>('observability-toggle')),
    );
    await tester.pumpAndSettle();

    expect(find.text('Logs'), findsWidgets);
    expect(find.text('Metrics'), findsWidgets);
    expect(find.text('Prometheus /metrics'), findsOneWidget);
    expect(observabilityKit.gate.masterEnabled, isFalse);

    await tester.tap(find.widgetWithText(SwitchListTile, 'Master'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(SwitchListTile, 'Logs'));
    await tester.pumpAndSettle();

    expect(observabilityKit.gate.masterEnabled, isTrue);
    expect(observabilityKit.gate.logsEnabled, isTrue);

    await tester.tap(
      find.byKey(const ValueKey<String>('observability-toggle')),
    );
    await _settleApp(tester);
    await tester.enterText(
      find.byKey(const ValueKey<String>('name-input')),
      'Ada',
    );
    await _settleApp(tester);
    expect(find.text('Hello Ada'), findsOneWidget);

    await tester.tap(
      find.byKey(const ValueKey<String>('observability-toggle')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(Tab, 'Logs'));
    await tester.pumpAndSettle();

    expect(find.text('Greeting response received'), findsWidgets);
  });
}

Future<void> _settleApp(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pumpAndSettle();
}
