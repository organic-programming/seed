import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';
import 'package:gabriel_greeting_app_flutter/src/settings_store.dart';

void main() {
  test('default unix path uses system temp', () {
    final path = CoaxSettingsSnapshot.defaultUnixPath;
    expect(
      path,
      endsWith('${Platform.pathSeparator}gabriel-greeting-coax.sock'),
    );
    expect(_isWithinTempHierarchy(path), isTrue);
  });

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

bool _isWithinTempHierarchy(String path) {
  final socketDir = File(path).parent.absolute.path;
  var current = Directory.systemTemp.absolute.path;
  while (true) {
    if (socketDir == current) {
      return true;
    }
    final parent = Directory(current).parent.absolute.path;
    if (parent == current) {
      return false;
    }
    current = parent;
  }
}
