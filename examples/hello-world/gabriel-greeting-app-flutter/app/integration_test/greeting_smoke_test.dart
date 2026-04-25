import 'package:flutter_test/flutter_test.dart';

import 'package:gabriel_greeting_app_flutter/src/app.dart';
import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';

import '../test/support/fakes.dart';

void main() {
  testWidgets('desktop smoke flow reaches the greeting bubble', (tester) async {
    final connection = FakeGreetingHolonConnection(
      languages: [language(code: 'en', name: 'English', native: 'English')],
      greetingBuilder: ({required name, required langCode}) => 'Hello $name',
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

    await greetingController.initialize();
    await waitForCoaxUpdate();
    await tester.pumpWidget(
      GabrielGreetingApp(
        greetingController: greetingController,
        coaxManager: coaxManager,
        observabilityKit: observabilityKit,
      ),
    );
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('Hello World'), findsOneWidget);
  });
}
