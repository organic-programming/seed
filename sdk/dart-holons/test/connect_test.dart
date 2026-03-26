import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:test/test.dart';

void main() {
  final sdkRoot = Directory.current.path;
  late String echoServerPath;

  setUpAll(() {
    if (Platform.isWindows) {
      return;
    }

    final goHolonsDir = '$sdkRoot/../go-holons';
    if (!Directory(goHolonsDir).existsSync()) {
      fail('go-holons SDK not found at $goHolonsDir');
    }

    echoServerPath = '${Directory.systemTemp.path}/echo-server-connect-test';
    final result = Process.runSync(
      _resolveGoBinary(),
      <String>['build', '-o', echoServerPath, './cmd/echo-server'],
      workingDirectory: goHolonsDir,
      environment: _withGoCache(),
    );
    if (result.exitCode != 0) {
      fail('Failed to build echo-server: ${result.stderr}');
    }
  });

  tearDownAll(() {
    if (Platform.isWindows) {
      return;
    }
    try {
      File(echoServerPath).deleteSync();
    } catch (_) {}
  });

  test(
    'connect(slug) defaults to stdio and disconnect stops the child',
    () async {
      final sandbox =
          Directory.systemTemp.createTempSync('dart-holons-connect');
      addTearDown(() => sandbox.delete(recursive: true));

      final fixture = _writeHolonFixture(sandbox, echoServerPath);
      final runner = _writeRunnerScript(sdkRoot, fixture.slug);
      addTearDown(() {
        if (runner.existsSync()) {
          runner.deleteSync();
        }
      });

      final result = await Process.run(
        Platform.resolvedExecutable,
        <String>[runner.path],
        workingDirectory: sandbox.path,
      );
      expect(
        result.exitCode,
        equals(0),
        reason:
            'runner failed:\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}',
      );

      final response =
          jsonDecode((result.stdout as String).trim()) as Map<String, dynamic>;
      expect(response['message'], equals('stdio-default-connect'));

      final pid = await _waitForPid(fixture.pidFile);
      expect(
        await _waitForFileContents(fixture.argsFile),
        equals('serve --listen stdio://'),
      );
      expect(File(fixture.portFile).existsSync(), isFalse);
      await _waitForProcessExit(pid);
      expect(File(fixture.portFile).existsSync(), isFalse);
    },
    timeout: const Timeout(Duration(seconds: 20)),
    skip: Platform.isWindows
        ? 'connect stdio fixture uses a POSIX shell wrapper'
        : false,
  );

  test('ConnectOptions defaults transport to stdio', () {
    expect(const ConnectOptions().transport, equals('stdio'));
  });

  test(
    'connect(slug) writes a unix port file and reuses the daemon',
    () async {
      final sandbox =
          Directory.systemTemp.createTempSync('dart-holons-connect-unix');
      addTearDown(() => sandbox.delete(recursive: true));

      final fixture = _writeHolonFixture(
        sandbox,
        echoServerPath,
        slug: 'connect-unix',
      );
      final runner = _writeUnixRunnerScript(sdkRoot, fixture.slug);
      addTearDown(() {
        if (runner.existsSync()) {
          runner.deleteSync();
        }
      });

      final result = await Process.run(
        Platform.resolvedExecutable,
        <String>[runner.path],
        workingDirectory: sandbox.path,
      );
      expect(
        result.exitCode,
        equals(0),
        reason:
            'runner failed:\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}',
      );

      final decoded =
          jsonDecode((result.stdout as String).trim()) as Map<String, dynamic>;
      expect(decoded['first'], equals('unix-dart'));
      expect(decoded['second'], equals('unix-dart-reuse'));

      final pid = await _waitForPid(fixture.pidFile);
      expect(
        await _waitForFileContents(fixture.argsFile),
        startsWith('serve --listen unix:///tmp/holons-'),
      );
      final target = await _waitForFileContents(fixture.portFile);
      expect(target, startsWith('unix:///tmp/holons-'));

      final kill = await Process.run('/bin/kill', <String>['-TERM', '$pid']);
      expect(kill.exitCode, anyOf(equals(0), equals(1)));
      await _waitForProcessExit(pid);
    },
    timeout: const Timeout(Duration(seconds: 20)),
    skip: Platform.isWindows
        ? 'connect unix fixture uses a POSIX shell wrapper'
        : false,
  );

  test(
    'connect(slug) fails fast when a stdio child exits during startup',
    () async {
      final sandbox =
          Directory.systemTemp.createTempSync('dart-holons-connect-fail');
      addTearDown(() => sandbox.delete(recursive: true));

      final fixture = _writeFailingHolonFixture(sandbox);
      final runner = _writeFailingRunnerScript(sdkRoot, fixture.slug);
      addTearDown(() {
        if (runner.existsSync()) {
          runner.deleteSync();
        }
      });

      final result = await Process.run(
        Platform.resolvedExecutable,
        <String>[runner.path],
        workingDirectory: sandbox.path,
      );
      expect(result.exitCode, equals(0), reason: result.stderr.toString());
      expect(
        result.stdout.toString(),
        contains('holon exited before accepting stdio RPCs'),
      );
      expect(
        result.stdout.toString(),
        contains('stdio fixture refused startup'),
      );

      expect(
        await _waitForFileContents(fixture.argsFile),
        equals('serve --listen stdio://'),
      );
    },
    timeout: const Timeout(Duration(seconds: 10)),
    skip: Platform.isWindows
        ? 'connect stdio fixture uses a POSIX shell wrapper'
        : false,
  );
}

class _ConnectFixture {
  final String slug;
  final String pidFile;
  final String argsFile;
  final String portFile;

  const _ConnectFixture({
    required this.slug,
    required this.pidFile,
    required this.argsFile,
    required this.portFile,
  });
}

_ConnectFixture _writeHolonFixture(
  Directory sandbox,
  String echoServerPath, {
  String slug = 'connect-stdio',
}) {
  final slugParts = slug.split('-').where((part) => part.isNotEmpty).toList();
  final givenName =
      slugParts.isNotEmpty ? _titleCase(slugParts.first) : 'Connect';
  final familyName = slugParts.length > 1
      ? slugParts.skip(1).map(_titleCase).join('-')
      : 'Stdio';
  final holonDir = Directory('${sandbox.path}/holons/$slug')
    ..createSync(recursive: true);
  final binDir = Directory('${holonDir.path}/.op/build/bin')
    ..createSync(recursive: true);

  final pidFile = '${sandbox.path}/$slug.pid';
  final argsFile = '${sandbox.path}/$slug.args';
  final wrapper = File('${binDir.path}/echo-server-wrapper');
  wrapper.writeAsStringSync('''
#!/bin/sh
printf '%s\\n' "\$\$" > ${_shellQuote(pidFile)}
printf '%s\\n' "\$*" > ${_shellQuote(argsFile)}
exec ${_shellQuote(echoServerPath)} "\$@"
''');
  wrapper.setLastModifiedSync(DateTime.now());
  Process.runSync('chmod', <String>['755', wrapper.path]);

  File('${holonDir.path}/holon.proto').writeAsStringSync('''
syntax = "proto3";

package test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "$slug-0000-0000-0000-000000000001"
    given_name: "$givenName"
    family_name: "$familyName"
    motto: "Round-trip."
    composer: "dart-holons-tests"
  }
  kind: "native"
  build: {
    runner: "go-module"
  }
  artifacts: {
    binary: "echo-server-wrapper"
  }
};
''');

  return _ConnectFixture(
    slug: slug,
    pidFile: pidFile,
    argsFile: argsFile,
    portFile: '${sandbox.path}/.op/run/$slug.port',
  );
}

_ConnectFixture _writeFailingHolonFixture(Directory sandbox) {
  const slug = 'connect-stdio-fail';
  final holonDir = Directory('${sandbox.path}/holons/$slug')
    ..createSync(recursive: true);
  final binDir = Directory('${holonDir.path}/.op/build/bin')
    ..createSync(recursive: true);

  final pidFile = '${sandbox.path}/$slug.pid';
  final argsFile = '${sandbox.path}/$slug.args';
  final wrapper = File('${binDir.path}/stdio-fail-wrapper');
  wrapper.writeAsStringSync('''
#!/bin/sh
printf '%s\\n' "\$\$" > ${_shellQuote(pidFile)}
printf '%s\\n' "\$*" > ${_shellQuote(argsFile)}
printf '%s\\n' "stdio fixture refused startup" >&2
exit 41
''');
  wrapper.setLastModifiedSync(DateTime.now());
  Process.runSync('chmod', <String>['755', wrapper.path]);

  File('${holonDir.path}/holon.proto').writeAsStringSync('''
syntax = "proto3";

package test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "connect-stdio-fail-0000-0000-0000-000000001"
    given_name: "Connect"
    family_name: "Stdio-Fail"
    motto: "Fails fast."
    composer: "dart-holons-tests"
  }
  kind: "native"
  build: {
    runner: "go-module"
  }
  artifacts: {
    binary: "stdio-fail-wrapper"
  }
};
''');

  return _ConnectFixture(
    slug: slug,
    pidFile: pidFile,
    argsFile: argsFile,
    portFile: '${sandbox.path}/.op/run/$slug.port',
  );
}

File _writeRunnerScript(String sdkRoot, String slug) {
  final runner = File(
    '$sdkRoot/.dart_tool/connect-runner-${DateTime.now().microsecondsSinceEpoch}.dart',
  );
  runner.parent.createSync(recursive: true);
  runner.writeAsStringSync('''
import 'dart:convert';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

Future<void> main() async {
  final channel = await connect(${jsonEncode(slug)});
  try {
    final method = ClientMethod<Map<String, dynamic>, Map<String, dynamic>>(
      '/echo.v1.Echo/Ping',
      (request) => utf8.encode(jsonEncode(request)),
      (payload) => jsonDecode(utf8.decode(payload)) as Map<String, dynamic>,
    );

    final call = channel.createCall(
      method,
      Stream<Map<String, dynamic>>.value(const <String, dynamic>{
        'message': 'stdio-default-connect',
      }),
      CallOptions(timeout: const Duration(seconds: 5)),
    );

    final response = await call.response.single;
    print(jsonEncode(response));
  } finally {
    await disconnect(channel);
  }
}
''');
  return runner;
}

File _writeUnixRunnerScript(String sdkRoot, String slug) {
  final runner = File(
    '$sdkRoot/.dart_tool/connect-unix-runner-${DateTime.now().microsecondsSinceEpoch}.dart',
  );
  runner.parent.createSync(recursive: true);
  runner.writeAsStringSync('''
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

Future<void> main() async {
  final first = await connect(
    ${jsonEncode(slug)},
    const ConnectOptions(
      timeout: Duration(seconds: 5),
      transport: 'unix',
      start: true,
    ),
  );

  try {
    final firstResponse = await _invoke(first, 'unix-dart');
    final second = await connect(
      ${jsonEncode(slug)},
      const ConnectOptions(
        timeout: Duration(seconds: 5),
        transport: 'unix',
        start: false,
      ),
    );

    try {
      final secondResponse = await _invoke(second, 'unix-dart-reuse');
      print(jsonEncode(<String, dynamic>{
        'first': firstResponse['message'],
        'second': secondResponse['message'],
      }));
    } finally {
      await disconnect(second);
    }
  } finally {
    await disconnect(first);
  }

  await stdout.flush();
  exit(0);
}

Future<Map<String, dynamic>> _invoke(ClientChannel channel, String message) async {
  final method = ClientMethod<Map<String, dynamic>, Map<String, dynamic>>(
    '/echo.v1.Echo/Ping',
    (request) => utf8.encode(jsonEncode(request)),
    (payload) => jsonDecode(utf8.decode(payload)) as Map<String, dynamic>,
  );

  final call = channel.createCall(
    method,
    Stream<Map<String, dynamic>>.value(<String, dynamic>{'message': message}),
    CallOptions(timeout: const Duration(seconds: 5)),
  );

  return call.response.single;
}
''');
  return runner;
}

File _writeFailingRunnerScript(String sdkRoot, String slug) {
  final runner = File(
    '$sdkRoot/.dart_tool/connect-fail-runner-${DateTime.now().microsecondsSinceEpoch}.dart',
  );
  runner.parent.createSync(recursive: true);
  runner.writeAsStringSync('''
import 'package:holons/holons.dart';

Future<void> main() async {
  try {
    await connect(${jsonEncode(slug)});
    throw StateError('connect unexpectedly succeeded');
  } on StateError catch (error) {
    print(error.message);
  }
}
''');
  return runner;
}

Future<int> _waitForPid(
  String path, {
  Duration timeout = const Duration(seconds: 5),
}) async {
  final contents = await _waitForFileContents(path, timeout: timeout);
  return int.parse(contents);
}

Future<String> _waitForFileContents(
  String path, {
  Duration timeout = const Duration(seconds: 5),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    final file = File(path);
    if (file.existsSync()) {
      final contents = (await file.readAsString()).trim();
      if (contents.isNotEmpty) {
        return contents;
      }
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('timed out waiting for file: $path');
}

Future<void> _waitForProcessExit(
  int pid, {
  Duration timeout = const Duration(seconds: 5),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (!await _pidExists(pid)) {
      return;
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('timed out waiting for pid $pid to exit');
}

Future<bool> _pidExists(int pid) async {
  final result = await Process.run('/bin/kill', <String>['-0', '$pid']);
  return result.exitCode == 0;
}

Future<Map<String, dynamic>> _invokePing(
  ClientChannel channel,
  String message,
) async {
  final method = ClientMethod<Map<String, dynamic>, Map<String, dynamic>>(
    '/echo.v1.Echo/Ping',
    (request) => utf8.encode(jsonEncode(request)),
    (payload) => jsonDecode(utf8.decode(payload)) as Map<String, dynamic>,
  );

  final call = channel.createCall(
    method,
    Stream<Map<String, dynamic>>.value(<String, dynamic>{
      'message': message,
    }),
    CallOptions(timeout: const Duration(seconds: 5)),
  );

  return call.response.single;
}

String _resolveGoBinary() {
  final fromEnv = (Platform.environment['GO_BIN'] ?? '').trim();
  if (fromEnv.isNotEmpty) {
    return fromEnv;
  }

  const preferredGoBinary = '/Users/bpds/go/go1.25.1/bin/go';
  final preferred = File(preferredGoBinary);
  if (preferred.existsSync()) {
    return preferred.path;
  }

  return 'go';
}

Map<String, String> _withGoCache() {
  final environment = Map<String, String>.from(Platform.environment);
  if ((environment['GOCACHE'] ?? '').trim().isEmpty) {
    environment['GOCACHE'] = '${Directory.systemTemp.path}/go-cache';
  }
  return environment;
}

String _shellQuote(String value) {
  return "'${value.replaceAll("'", "'\"'\"'")}'";
}

String _titleCase(String value) {
  if (value.isEmpty) {
    return value;
  }
  return value[0].toUpperCase() + value.substring(1).toLowerCase();
}
