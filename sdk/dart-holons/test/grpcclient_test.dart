import 'dart:io';

import 'package:holons/holons.dart';
import 'package:test/test.dart';

void main() {
  group('StdioTransportConnector', () {
    test('spawn starts a process', () async {
      // Use `cat` as a trivial subprocess that reads stdin and echoes stdout.
      // This tests the spawn/kill lifecycle without needing a real gRPC server.
      final connector = await StdioTransportConnector.spawn(
        'cat',
        args: const <String>[],
      );
      expect(connector.process.pid, greaterThan(0));
      expect(connector.authority, equals('localhost'));

      connector.shutdown();
      final exitCode = await connector.process.exitCode;
      // cat exits with a non-zero code after SIGTERM.
      expect(exitCode, isNot(0));
    });

    test('shutdown kills the process', () async {
      final connector = await StdioTransportConnector.spawn(
        'sleep',
        args: const <String>['60'],
      );
      connector.shutdown();
      final exitCode =
          await connector.process.exitCode.timeout(const Duration(seconds: 5));
      expect(exitCode, isNot(0));
    });

    test('done completes when process exits', () async {
      final connector = await StdioTransportConnector.spawn(
        'true',
        args: const <String>[],
      );
      await connector.done.timeout(const Duration(seconds: 5));
      await connector.process.exitCode.timeout(const Duration(seconds: 5));
    });

    test('spawn throws on bad binary path', () async {
      expect(
        () => StdioTransportConnector.spawn('/nonexistent/binary'),
        throwsA(isA<ProcessException>()),
      );
    });
  });

  group('dialStdio', () {
    test(
      'returns channel and process',
      () async {
        // This test requires the Go echo-server binary.
        // Skip if unavailable.
        final echoServerBinary = _buildEchoServer();
        if (echoServerBinary == null) {
          return;
        }

        final (channel, process) = await dialStdio(echoServerBinary);
        expect(process.pid, greaterThan(0));

        // Clean up.
        process.kill(ProcessSignal.sigterm);
        await process.exitCode.timeout(const Duration(seconds: 5));
        await channel.shutdown().timeout(const Duration(seconds: 5));
      },
      timeout: const Timeout(Duration(seconds: 60)),
    );
  });
}

/// Build the Go echo-server and return its path, or null if Go is unavailable.
String? _buildEchoServer() {
  final goHolonsDir = '${Directory.current.path}/../go-holons';
  if (!Directory(goHolonsDir).existsSync()) {
    return null;
  }

  final outputPath = '${Directory.systemTemp.path}/echo-server-holons-test';
  ProcessResult result;
  try {
    result = Process.runSync(
      'go',
      <String>['build', '-o', outputPath, './cmd/echo-server'],
      workingDirectory: goHolonsDir,
      environment: _withGoCache(),
    );
  } on ProcessException {
    return null;
  }
  return result.exitCode == 0 ? outputPath : null;
}

Map<String, String> _withGoCache() {
  final environment = Map<String, String>.from(Platform.environment);
  if ((environment['GOCACHE'] ?? '').trim().isEmpty) {
    environment['GOCACHE'] = '${Directory.systemTemp.path}/go-cache';
  }
  return environment;
}
