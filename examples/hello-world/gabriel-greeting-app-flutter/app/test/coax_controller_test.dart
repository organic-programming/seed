import 'package:flutter_test/flutter_test.dart';
import 'package:holons_app/holons_app.dart';

import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';

import 'support/fakes.dart';

void main() {
  test('starts a TCP COAX server and persists settings', () async {
    final connection = FakeGreetingHolonConnection(
      languages: [language(code: 'en', name: 'English', native: 'English')],
      greetingBuilder: ({required name, required langCode}) => 'Hello $name',
    );
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog(<GabrielHolonIdentity>[
        holon('gabriel-greeting-swift'),
      ]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (_) => connection,
        },
      ),
    );
    final store = MemorySettingsStore();
    final coaxController = buildCoaxController(
      greetingController: greetingController,
      settingsStore: store,
      capabilities: const AppPlatformCapabilities(supportsUnixSockets: false),
    );

    await greetingController.initialize();
    final port = await reserveTcpPort();
    await coaxController.setServerPortText(port.toString());
    await coaxController.setIsEnabled(true);

    expect(coaxController.listenUri, 'tcp://127.0.0.1:$port');
    expect(coaxController.serverStatus.state, CoaxSurfaceState.live);
    expect(store.readBool('coax.server.enabled'), isTrue);
    expect(
      store.readString('coax.server.settings'),
      contains('"serverPortText":"$port"'),
    );

    await coaxController.shutdown();
    await greetingController.shutdown();
  });
}
