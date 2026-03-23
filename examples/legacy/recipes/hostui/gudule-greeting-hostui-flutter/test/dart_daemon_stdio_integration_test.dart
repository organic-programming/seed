import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'package:gudule_greeting_hostui_flutter/src/client/greeting_client.dart';
import 'package:gudule_greeting_hostui_flutter/src/client/greeting_target.dart';

void main() {
  test('real Dart greeting daemon serves stdio RPCs through GreetingClient',
      () async {
    if (Platform.isWindows) {
      return;
    }

    final daemonDir = _resolveDartDaemonDir();
    final pubGet = await Process.run(
      _dartBinary(),
      const <String>['pub', 'get'],
      workingDirectory: daemonDir.path,
    );
    expect(
      pubGet.exitCode,
      0,
      reason:
          'dart pub get failed:\nstdout:\n${pubGet.stdout}\nstderr:\n${pubGet.stderr}',
    );

    final sandbox =
        await Directory.systemTemp.createTemp('dart-daemon-bundle-');
    addTearDown(() => sandbox.delete(recursive: true));
    final binaryName = Platform.isWindows
        ? 'gudule-daemon-greeting-dart.exe'
        : 'gudule-daemon-greeting-dart';
    final binary = File('${sandbox.path}/$binaryName');

    final compile = await Process.run(
      _dartBinary(),
      <String>['compile', 'exe', 'bin/main.dart', '-o', binary.path],
      workingDirectory: daemonDir.path,
    );
    expect(
      compile.exitCode,
      0,
      reason:
          'dart compile exe failed:\nstdout:\n${compile.stdout}\nstderr:\n${compile.stderr}',
    );
    expect(binary.existsSync(), isTrue, reason: 'missing daemon binary');

    final client = GreetingClient();
    try {
      await client.connect(
        GreetingEndpoint(
          bundledBinaryPath: binary.path,
          daemon: GreetingDaemonIdentity.fromBinaryPath(binary.path),
        ),
      );

      final languages = await client.listLanguages();
      expect(languages.languages, isNotEmpty);
      expect(
        languages.languages.any((language) => language.code == 'en'),
        isTrue,
      );

      final response = await client.sayHello('Bob', 'fr');
      expect(response.greeting, contains('Bob'));
      expect(response.langCode, 'fr');
    } finally {
      await client.close();
    }
  });
}

String _dartBinary() {
  final fromEnv = (Platform.environment['DART_BIN'] ?? '').trim();
  return fromEnv.isNotEmpty ? fromEnv : 'dart';
}

Directory _resolveDartDaemonDir() {
  final candidates = <Directory>[
    Directory(
        '${Directory.current.path}/../../daemons/gudule-daemon-greeting-dart'),
    Directory(
        '${Directory.current.path}/recipes/daemons/gudule-daemon-greeting-dart'),
    Directory(
        '${Directory.current.path}/../daemons/gudule-daemon-greeting-dart'),
  ];

  for (final candidate in candidates) {
    if (candidate.existsSync()) {
      return candidate.absolute;
    }
  }

  return candidates.first.absolute;
}
