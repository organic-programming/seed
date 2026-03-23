import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'package:gudule_greeting_hostui_flutter/src/client/greeting_client.dart';
import 'package:gudule_greeting_hostui_flutter/src/client/greeting_target.dart';

void main() {
  test('real Swift greeting daemon serves stdio RPCs through GreetingClient',
      () async {
    if (Platform.isWindows) {
      return;
    }

    final daemonDir = _resolveSwiftDaemonDir();
    final buildPath = Directory('${daemonDir.path}/.op/build/swift');
    final build = await Process.run(
      _swiftBinary(),
      <String>['build', '--build-path', buildPath.path, '-c', 'debug'],
      workingDirectory: daemonDir.path,
    );
    expect(
      build.exitCode,
      0,
      reason:
          'swift build failed:\nstdout:\n${build.stdout}\nstderr:\n${build.stderr}',
    );

    final binaryName = Platform.isWindows
        ? 'gudule-daemon-greeting-swift.exe'
        : 'gudule-daemon-greeting-swift';
    final builtBinary = File('${buildPath.path}/debug/$binaryName');
    expect(builtBinary.existsSync(), isTrue, reason: 'missing daemon binary');

    final sandbox =
        await Directory.systemTemp.createTemp('swift-daemon-bundle-');
    addTearDown(() => sandbox.delete(recursive: true));
    final binary = File('${sandbox.path}/$binaryName');
    await builtBinary.copy(binary.path);

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

String _swiftBinary() {
  final fromEnv = (Platform.environment['SWIFT_BIN'] ?? '').trim();
  return fromEnv.isNotEmpty ? fromEnv : 'swift';
}

Directory _resolveSwiftDaemonDir() {
  final candidates = <Directory>[
    Directory(
        '${Directory.current.path}/../../daemons/gudule-daemon-greeting-swift'),
    Directory(
        '${Directory.current.path}/recipes/daemons/gudule-daemon-greeting-swift'),
    Directory(
        '${Directory.current.path}/../daemons/gudule-daemon-greeting-swift'),
  ];

  for (final candidate in candidates) {
    if (candidate.existsSync()) {
      return candidate.absolute;
    }
  }

  return candidates.first.absolute;
}
