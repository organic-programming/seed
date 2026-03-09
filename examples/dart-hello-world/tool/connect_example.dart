import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

Future<File> sdkEchoServer() async {
  final path = File(
    '${Directory.current.path}/../../sdk/dart-holons/bin/echo-server',
  ).absolute;
  if (!await path.exists()) {
    throw StateError('echo-server not found at ${path.path}');
  }
  return path;
}

Future<void> writeEchoHolon(Directory root, File binaryPath) async {
  final holonDir = Directory('${root.path}/holons/echo-server');
  await holonDir.create(recursive: true);
  final holonYaml = File('${holonDir.path}/holon.yaml');
  await holonYaml.writeAsString('''
uuid: "echo-server-connect-example"
given_name: Echo
family_name: Server
motto: Reply precisely.
composer: "connect-example"
kind: service
build:
  runner: dart
  main: bin/echo-server
artifacts:
  binary: "${binaryPath.path}"
''');
}

Future<void> main() async {
  final root = await Directory.systemTemp.createTemp(
    'dart-holons-connect-example-',
  );
  final previousCwd = Directory.current;

  try {
    await writeEchoHolon(root, await sdkEchoServer());
    Directory.current = root.path;

    final channel = await holons.connect('echo-server');
    try {
      final method = ClientMethod<String, String>(
        '/echo.v1.Echo/Ping',
        (request) => utf8.encode(request),
        (payload) => utf8.decode(payload),
      );

      final call = channel.createCall(
        method,
        Stream<String>.value('{"message":"hello-from-dart"}'),
        CallOptions(timeout: const Duration(seconds: 5)),
      );

      stdout.writeln(await call.response.single);
    } finally {
      await holons.disconnect(channel);
    }
  } finally {
    Directory.current = previousCwd.path;
    if (await root.exists()) {
      await root.delete(recursive: true);
    }
  }
}
