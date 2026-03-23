import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'package:gudule_greeting_hostui_flutter/src/client/greeting_client.dart';
import 'package:gudule_greeting_hostui_flutter/src/client/greeting_target.dart';

void main() {
  test('real Rust greeting daemon serves stdio RPCs through GreetingClient',
      () async {
    if (Platform.isWindows) {
      return;
    }

    final daemonDir = _resolveRustDaemonDir();
    final targetDir = Directory('${daemonDir.path}/.op/build/cargo');
    final build = await Process.run(
      _cargoBinary(),
      <String>['build', '--target-dir', targetDir.path],
      workingDirectory: daemonDir.path,
    );
    expect(
      build.exitCode,
      0,
      reason:
          'cargo build failed:\nstdout:\n${build.stdout}\nstderr:\n${build.stderr}',
    );

    final binaryName = Platform.isWindows
        ? 'gudule-daemon-greeting-rust.exe'
        : 'gudule-daemon-greeting-rust';
    final binary = File('${targetDir.path}/debug/$binaryName');
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

String _cargoBinary() {
  final fromEnv = (Platform.environment['CARGO_BIN'] ?? '').trim();
  return fromEnv.isNotEmpty ? fromEnv : 'cargo';
}

Directory _resolveRustDaemonDir() {
  final candidates = <Directory>[
    Directory(
        '${Directory.current.path}/../../daemons/gudule-daemon-greeting-rust'),
    Directory(
        '${Directory.current.path}/recipes/daemons/gudule-daemon-greeting-rust'),
    Directory(
        '${Directory.current.path}/../daemons/gudule-daemon-greeting-rust'),
  ];

  for (final candidate in candidates) {
    if (candidate.existsSync()) {
      return candidate.absolute;
    }
  }

  return candidates.first.absolute;
}
