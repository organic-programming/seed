import 'package:flutter_test/flutter_test.dart';

import 'package:gabriel_greeting_app_flutter/src/settings_store.dart';

void main() {
  test('launch environment overrides seed coax settings', () async {
    final store = MemorySettingsStore();

    await applyLaunchEnvironmentOverrides(
      store,
      environment: const <String, String>{
        'OP_COAX_SERVER_ENABLED': 'true',
        'OP_COAX_SERVER_LISTEN_URI': 'tcp://127.0.0.1:61000',
      },
    );

    expect(store.readBool('coax.server.enabled'), isTrue);
    expect(
      store.readString('coax.server.settings'),
      contains('"serverPortText":"61000"'),
    );
  });
}
