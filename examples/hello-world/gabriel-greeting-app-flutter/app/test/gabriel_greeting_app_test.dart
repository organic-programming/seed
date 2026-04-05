import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:gabriel_greeting_app_flutter/src/app.dart';
import 'package:gabriel_greeting_app_flutter/src/controller/coax_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';
import 'package:gabriel_greeting_app_flutter/src/settings_store.dart';

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
    final coaxController = CoaxController(
      greetingController: greetingController,
      settingsStore: MemorySettingsStore(),
    );

    final app = GabrielGreetingApp(
      greetingController: greetingController,
      coaxController: coaxController,
    );

    expect(app.greetingController, same(greetingController));
    expect(app.coaxController, same(coaxController));
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
    final coaxController = CoaxController(
      greetingController: greetingController,
      settingsStore: MemorySettingsStore(),
      capabilities: const AppPlatformCapabilities(supportsUnixSockets: true),
    );

    await tester.pumpWidget(
      GabrielGreetingApp(
        greetingController: greetingController,
        coaxController: coaxController,
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
    final coaxController = CoaxController(
      greetingController: greetingController,
      settingsStore: MemorySettingsStore(),
    );

    await tester.pumpWidget(
      GabrielGreetingApp(
        greetingController: greetingController,
        coaxController: coaxController,
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
}

Future<void> _settleApp(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pumpAndSettle();
}
