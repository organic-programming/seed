import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'package:gudule_greeting_hostui_flutter/src/client/greeting_client.dart';
import 'package:gudule_greeting_hostui_flutter/src/client/greeting_target.dart';

void main() {
  test('real Node greeting daemon serves stdio RPCs through a copied launcher',
      () async {
    if (Platform.isWindows) {
      return;
    }

    final daemonDir = _resolveNodeDaemonDir();
    final npmCi = await Process.run(
      _npmBinary(),
      const <String>['ci'],
      workingDirectory: daemonDir.path,
    );
    expect(
      npmCi.exitCode,
      0,
      reason:
          'npm ci failed:\nstdout:\n${npmCi.stdout}\nstderr:\n${npmCi.stderr}',
    );

    final build = await Process.run(
      _npmBinary(),
      const <String>['run', 'build'],
      workingDirectory: daemonDir.path,
    );
    expect(
      build.exitCode,
      0,
      reason:
          'npm run build failed:\nstdout:\n${build.stdout}\nstderr:\n${build.stderr}',
    );

    final entrypoint =
        File('${daemonDir.path}/dist/gudule-daemon-greeting-node.js');
    expect(entrypoint.existsSync(), isTrue,
        reason: 'missing daemon entrypoint');

    final sandbox =
        await Directory.systemTemp.createTemp('node-daemon-bundle-');
    addTearDown(() => sandbox.delete(recursive: true));
    final launcher = File('${sandbox.path}/gudule-daemon-greeting-node');

    final nodePath = await _resolvedNodeBinary();
    final script = StringBuffer()
      ..writeln('#!/bin/sh')
      ..writeln('set -eu')
      ..writeln("cd '${_shellQuote(daemonDir.path)}'")
      ..writeln(
        "exec '${_shellQuote(nodePath)}' '${_shellQuote(entrypoint.path)}' \"\$@\"",
      );
    await launcher.writeAsString(script.toString());
    await Process.run('chmod', <String>['755', launcher.path]);

    final client = GreetingClient();
    try {
      await client.connect(
        GreetingEndpoint(
          bundledBinaryPath: launcher.path,
          daemon: GreetingDaemonIdentity.fromBinaryPath(launcher.path),
        ),
      );

      await Future<void>.delayed(const Duration(milliseconds: 600));

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

String _npmBinary() {
  final fromEnv = (Platform.environment['NPM_BIN'] ?? '').trim();
  return fromEnv.isNotEmpty ? fromEnv : 'npm';
}

Future<String> _resolvedNodeBinary() async {
  final fromEnv = (Platform.environment['NODE_BIN'] ?? '').trim();
  if (fromEnv.isNotEmpty) {
    return fromEnv;
  }

  final probe = await Process.run(
    'node',
    const <String>['-p', 'process.execPath'],
  );
  if (probe.exitCode == 0) {
    final resolved = '${probe.stdout}'.trim();
    if (resolved.isNotEmpty) {
      return resolved;
    }
  }
  return 'node';
}

Directory _resolveNodeDaemonDir() {
  final candidates = <Directory>[
    Directory(
        '${Directory.current.path}/../../daemons/gudule-daemon-greeting-node'),
    Directory(
        '${Directory.current.path}/recipes/daemons/gudule-daemon-greeting-node'),
    Directory(
        '${Directory.current.path}/../daemons/gudule-daemon-greeting-node'),
  ];

  for (final candidate in candidates) {
    if (candidate.existsSync()) {
      return candidate.absolute;
    }
  }

  return candidates.first.absolute;
}

String _shellQuote(String value) => value.replaceAll("'", "'\"'\"'");
