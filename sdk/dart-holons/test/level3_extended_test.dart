import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:holons/holons.dart';
import 'package:holons/src/echo_cli.dart';
import 'package:test/test.dart';

void main() {
  group('level 3 extended certification', () {
    test('echo client supports websocket grpc dial against Go server',
        () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final sdkDir = Directory.current.parent;
      final goHolonsDir = Directory('${sdkDir.path}/go-holons');
      final stderrBuffer = StringBuffer();
      final process = await Process.start(
        resolveGoBinary(),
        <String>[
          'run',
          './cmd/echo-server',
          '--listen',
          'ws://127.0.0.1:0/grpc',
          '--sdk',
          'go-holons',
        ],
        workingDirectory: goHolonsDir.path,
        environment: _withGoCache(),
      );

      final stderrSubscription =
          process.stderr.transform(utf8.decoder).listen(stderrBuffer.write);
      final stdoutLines = process.stdout
          .transform(utf8.decoder)
          .transform(const LineSplitter());

      try {
        final wsURI =
            await stdoutLines.first.timeout(const Duration(seconds: 20));
        expect(wsURI.startsWith('ws://'), isTrue);

        final payload = await runEchoClient(
          <String>[
            '--message',
            'cert-l3-ws',
            wsURI,
          ],
          environment: _withGoCache(),
        );
        final decoded = jsonDecode(payload) as Map<String, dynamic>;
        expect(decoded['status'], equals('pass'));
        expect(decoded['response_sdk'], equals('go-holons'));
      } on TimeoutException {
        final details = stderrBuffer.toString();
        if (_isBindDenied(details) || _isGoCacheDenied(details)) {
          return;
        }
        rethrow;
      } finally {
        await _stopProcess(process);
        await stderrSubscription.cancel();
      }
    });

    test('echo client accepts ws://:0 certification URI', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      try {
        final payload = await runEchoClient(
          const <String>[
            '--message',
            'cert-l3-ws-0',
            'ws://127.0.0.1:0/grpc',
          ],
          environment: _withGoCache(),
        );
        final decoded = jsonDecode(payload) as Map<String, dynamic>;
        expect(decoded['status'], equals('pass'));
        expect(decoded['response_sdk'], equals('go-holons'));
      } on ProcessException catch (error) {
        final text = error.message.toLowerCase();
        if (_isBindDenied(text) || _isGoCacheDenied(text)) {
          return;
        }
        rethrow;
      }
    });

    test('holonrpc_server entrypoint handles echo roundtrip', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final stderrBuffer = StringBuffer();
      final process = await Process.start(
        'dart',
        const <String>[
          '--suppress-analytics',
          'run',
          'bin/holonrpc_server.dart',
          '--once',
        ],
        workingDirectory: Directory.current.path,
        environment: _withGoCache(),
      );

      final stderrSubscription =
          process.stderr.transform(utf8.decoder).listen(stderrBuffer.write);
      final stdoutLines = process.stdout
          .transform(utf8.decoder)
          .transform(const LineSplitter());

      HolonRPCClient? client;
      var skip = false;
      try {
        final wsURL =
            await stdoutLines.first.timeout(const Duration(seconds: 20));
        expect(wsURL.startsWith('ws://'), isTrue);

        // Defaults (interval 15s, timeout 5s) are appropriate here — this
        // test exercises the echo roundtrip, not heartbeat behavior. The
        // previous 250ms/250ms heartbeat timeout raced with `--once` mode
        // teardown on loaded CI runners, closing the client-side connection
        // before the invoke response arrived.
        client = HolonRPCClient();
        await client.connect(wsURL);

        final out = await client.invoke(
          'echo.v1.Echo/Ping',
          params: const <String, dynamic>{'message': 'hello'},
        );
        expect(out['message'], equals('hello'));
        expect(out['sdk'], equals('dart-holons'));
      } on TimeoutException {
        final details = stderrBuffer.toString();
        if (_isBindDenied(details) || _isGoCacheDenied(details)) {
          skip = true;
          return;
        }
        rethrow;
      } finally {
        await client?.close();
        if (skip) {
          await _stopProcess(process);
          await stderrSubscription.cancel();
        }
      }

      final code = await process.exitCode.timeout(const Duration(seconds: 20));
      final stderrText = stderrBuffer.toString();
      expect(code, equals(0), reason: stderrText);
      await _stopProcess(process);
      await stderrSubscription.cancel();
    });
  });
}

Future<bool> _canBindLoopbackTcp() async {
  try {
    final probe = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    await probe.close();
    return true;
  } on SocketException catch (error) {
    if (_isBindDenied(error)) {
      return false;
    }
    rethrow;
  }
}

Map<String, String> _withGoCache() {
  final environment = Map<String, String>.from(Platform.environment);
  if ((environment['GOCACHE'] ?? '').trim().isEmpty) {
    environment['GOCACHE'] = '/tmp/go-cache';
  }
  return environment;
}

bool _isBindDenied(Object value) {
  final text = value.toString().toLowerCase();
  return text.contains('operation not permitted') ||
      text.contains('permission denied') ||
      text.contains('errno = 1');
}

bool _isGoCacheDenied(String value) {
  final text = value.toLowerCase();
  return text.contains('failed to trim cache') &&
      text.contains('operation not permitted');
}

Future<void> _stopProcess(Process process) async {
  if (process.kill(ProcessSignal.sigterm)) {
    try {
      await process.exitCode.timeout(const Duration(seconds: 5));
      return;
    } on TimeoutException {
      process.kill(ProcessSignal.sigkill);
      await process.exitCode.timeout(const Duration(seconds: 5));
      return;
    }
  }

  try {
    await process.exitCode.timeout(const Duration(seconds: 5));
  } on TimeoutException {
    process.kill(ProcessSignal.sigkill);
    await process.exitCode.timeout(const Duration(seconds: 5));
  }
}
