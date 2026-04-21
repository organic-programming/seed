import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:test/test.dart';

void main() {
  late String echoServerPath;

  setUpAll(() {
    final goHolonsDir = '${Directory.current.path}/../go-holons';
    if (!Directory(goHolonsDir).existsSync()) {
      fail('go-holons SDK not found at $goHolonsDir');
    }

    echoServerPath = '${Directory.systemTemp.path}/echo-server-interop-test';
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
    // Clean up binary.
    try {
      File(echoServerPath).deleteSync();
    } catch (_) {}
  });

  test(
    'dialStdio connects to Go echo-server and Echo/Ping works over stdio',
    () async {
      final (channel, process) = await dialStdio(echoServerPath);

      try {
        expect(process.pid, greaterThan(0));

        // Give the server a moment to start its HTTP/2 listener.
        await Future<void>.delayed(const Duration(milliseconds: 200));

        final response = await _ping(channel, 'hello-interop');
        expect(response['message'], equals('hello-interop'));
      } finally {
        await channel.shutdown();
        await _terminateProcess(process);
      }
    },
    timeout: const Timeout(Duration(seconds: 15)),
  );

  test(
    'dialStdio channel terminates cleanly when process is killed',
    () async {
      final (channel, process) = await dialStdio(echoServerPath);

      // Kill the process first.
      process.kill(ProcessSignal.sigterm);
      final exitCode =
          await process.exitCode.timeout(const Duration(seconds: 5));

      // Channel shutdown after process death should not throw.
      await channel.shutdown();
      expect(exitCode, isNotNull);
    },
    timeout: const Timeout(Duration(seconds: 15)),
  );
}

Future<Map<String, dynamic>> _ping(
  ClientTransportConnectorChannel channel,
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

Future<void> _terminateProcess(Process process) async {
  final exited = process.kill(ProcessSignal.sigterm);
  if (!exited) {
    await process.exitCode;
    return;
  }

  try {
    await process.exitCode.timeout(const Duration(seconds: 5));
  } on TimeoutException {
    process.kill(ProcessSignal.sigkill);
    await process.exitCode.timeout(const Duration(seconds: 5));
  }
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
    environment['GOCACHE'] = '/tmp/go-cache';
  }
  return environment;
}
