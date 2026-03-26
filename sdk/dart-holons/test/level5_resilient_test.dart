import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:holons/src/echo_cli.dart';
import 'package:test/test.dart';

void main() {
  group('level 5 resilient certification', () {
    test('echo-server drains concurrent startup burst on SIGTERM', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final serverStderr = StringBuffer();
      final server = await Process.start(
        './bin/echo-server',
        const <String>[
          '--listen',
          'tcp://127.0.0.1:0',
        ],
        workingDirectory: Directory.current.path,
        environment: _withGoCache(),
      );

      final stderrSub =
          server.stderr.transform(utf8.decoder).listen(serverStderr.write);
      final stdoutLines =
          server.stdout.transform(utf8.decoder).transform(const LineSplitter());

      try {
        final uri =
            await stdoutLines.first.timeout(const Duration(seconds: 20));
        expect(uri.startsWith('tcp://'), isTrue);

        final warmup = await Process.run(
          resolveGoBinary(),
          <String>[
            'run',
            './cmd/echo-client',
            '--server-sdk',
            'dart-holons',
            '--message',
            'l5-1-warmup',
            uri,
          ],
          workingDirectory: _goHolonsDir.path,
          environment: _withGoCache(),
        ).timeout(const Duration(seconds: 25));

        final warmupStderr = warmup.stderr.toString();
        if (_isBindDenied(warmupStderr) || _isGoCacheDenied(warmupStderr)) {
          return;
        }
        expect(warmup.exitCode, equals(0), reason: warmupStderr);

        const totalClients = 5;
        final clients = <Future<ProcessResult>>[];
        for (var i = 0; i < totalClients; i++) {
          clients.add(
            Process.run(
              resolveGoBinary(),
              <String>[
                'run',
                './cmd/echo-client',
                '--timeout-ms',
                '10000',
                '--server-sdk',
                'dart-holons',
                '--message',
                'l5-1-concurrent-$i',
                uri,
              ],
              workingDirectory: _goHolonsDir.path,
              environment: _withGoCache(),
            ).timeout(const Duration(seconds: 25)),
          );
        }

        await Future<void>.delayed(const Duration(milliseconds: 50));
        final stopwatch = Stopwatch()..start();
        server.kill(ProcessSignal.sigterm);
        final results = await Future.wait(clients);
        final exitCode =
            await server.exitCode.timeout(const Duration(seconds: 10));
        stopwatch.stop();

        for (final result in results) {
          final stderr = result.stderr.toString();
          if (_isBindDenied(stderr) || _isGoCacheDenied(stderr)) {
            return;
          }
        }

        expect(exitCode, equals(0), reason: serverStderr.toString());
        expect(
          stopwatch.elapsed,
          lessThanOrEqualTo(const Duration(seconds: 10)),
          reason: 'shutdown elapsed ${stopwatch.elapsed.inMilliseconds}ms',
        );

        final failed = <String>[];
        for (var i = 0; i < results.length; i++) {
          if (results[i].exitCode == 0) {
            continue;
          }
          failed.add(
            'client[$i] rc=${results[i].exitCode} stderr=${results[i].stderr}',
          );
        }
        expect(failed, isEmpty, reason: failed.join('\n'));
      } finally {
        await _stopProcess(server);
        await stderrSub.cancel();
      }
    });

    test('echo-server supports --handler-delay-ms and propagates deadline',
        () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final serverStderr = StringBuffer();
      final server = await Process.start(
        './bin/echo-server',
        const <String>[
          '--listen',
          'tcp://127.0.0.1:0',
          '--handler-delay-ms',
          '5000',
        ],
        workingDirectory: Directory.current.path,
        environment: _withGoCache(),
      );

      final stderrSub =
          server.stderr.transform(utf8.decoder).listen(serverStderr.write);
      final stdoutLines =
          server.stdout.transform(utf8.decoder).transform(const LineSplitter());

      try {
        final uri =
            await stdoutLines.first.timeout(const Duration(seconds: 20));
        expect(uri.startsWith('tcp://'), isTrue);

        final timeoutRun = await Process.run(
          resolveGoBinary(),
          <String>[
            'run',
            './cmd/echo-client',
            '--server-sdk',
            'dart-holons',
            '--message',
            'timeout-check',
            '--timeout-ms',
            '2000',
            uri,
          ],
          workingDirectory: _goHolonsDir.path,
          environment: _withGoCache(),
        ).timeout(const Duration(seconds: 25));

        final timeoutStdout = timeoutRun.stdout.toString();
        final timeoutStderr = timeoutRun.stderr.toString();
        if (_isBindDenied(timeoutStderr) || _isGoCacheDenied(timeoutStderr)) {
          return;
        }

        expect(
          timeoutRun.exitCode,
          isNot(equals(0)),
          reason: '$timeoutStdout\n$timeoutStderr',
        );
        expect(
          timeoutStderr.toLowerCase(),
          anyOf(contains('deadlineexceeded'), contains('deadline exceeded')),
          reason: timeoutStderr,
        );

        final followupRun = await Process.run(
          resolveGoBinary(),
          <String>[
            'run',
            './cmd/echo-client',
            '--server-sdk',
            'dart-holons',
            '--message',
            'timeout-followup',
            '--timeout-ms',
            '7000',
            uri,
          ],
          workingDirectory: _goHolonsDir.path,
          environment: _withGoCache(),
        ).timeout(const Duration(seconds: 25));

        final followupStdout = followupRun.stdout.toString();
        final followupStderr = followupRun.stderr.toString();
        expect(followupRun.exitCode, equals(0), reason: followupStderr);
        expect(followupStdout, contains('"status":"pass"'));
      } finally {
        await _stopProcess(server);
        await stderrSub.cancel();
      }
    });

    test('echo-server rejects 2MB payload and stays healthy', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final serverStderr = StringBuffer();
      final server = await Process.start(
        './bin/echo-server',
        const <String>[
          '--listen',
          'tcp://127.0.0.1:0',
        ],
        workingDirectory: Directory.current.path,
        environment: _withGoCache(),
      );

      final stderrSub =
          server.stderr.transform(utf8.decoder).listen(serverStderr.write);
      final stdoutLines =
          server.stdout.transform(utf8.decoder).transform(const LineSplitter());

      final probeFile = File(
        '${_goHolonsDir.path}/tmp-dart-l5-7-${DateTime.now().microsecondsSinceEpoch}.go',
      );
      await probeFile.writeAsString(_goOversizeProbeSource);

      try {
        final uri =
            await stdoutLines.first.timeout(const Duration(seconds: 20));
        expect(uri.startsWith('tcp://'), isTrue);

        final probe = await Process.run(
          resolveGoBinary(),
          <String>[
            'run',
            probeFile.path,
            uri,
          ],
          workingDirectory: _goHolonsDir.path,
          environment: _withGoCache(),
        ).timeout(const Duration(seconds: 30));

        final probeStdout = probe.stdout.toString();
        final probeStderr = probe.stderr.toString();
        if (_isBindDenied(probeStderr) || _isGoCacheDenied(probeStderr)) {
          return;
        }

        expect(probe.exitCode, equals(0), reason: '$probeStdout\n$probeStderr');
        expect(probeStdout, contains('RESULT=RESOURCE_EXHAUSTED'),
            reason: '$probeStdout\n$probeStderr');
        expect(probeStdout, contains('SMALL=OK'),
            reason: '$probeStdout\n$probeStderr');
      } finally {
        if (await probeFile.exists()) {
          await probeFile.delete();
        }
        await _stopProcess(server);
        await stderrSub.cancel();
      }
    });
  });
}

Directory get _goHolonsDir =>
    Directory('${Directory.current.parent.path}/go-holons');

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

const String _goOversizeProbeSource = r'''
package main

import (
  "context"
  "encoding/json"
  "fmt"
  "os"
  "strings"
  "time"

  "google.golang.org/grpc"
  "google.golang.org/grpc/credentials/insecure"
)

type PingRequest struct { Message string `json:"message"` }
type PingResponse struct { Message string `json:"message"`; SDK string `json:"sdk"` }
type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func main() {
  if len(os.Args) != 2 {
    fmt.Println("RESULT=BAD_ARGS")
    os.Exit(2)
  }

  target := strings.TrimPrefix(os.Args[1], "tcp://")
  dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancelDial()

  conn, err := grpc.DialContext(dialCtx, target,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithBlock(),
    grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
  )
  if err != nil {
    fmt.Printf("RESULT=DIAL_ERROR err=%v\n", err)
    os.Exit(1)
  }
  defer conn.Close()

  big := strings.Repeat("x", 2*1024*1024)
  reqCtx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancelReq()

  var largeOut PingResponse
  err = conn.Invoke(reqCtx, "/echo.v1.Echo/Ping", &PingRequest{Message: big}, &largeOut, grpc.ForceCodec(jsonCodec{}))
  if err != nil {
    low := strings.ToLower(err.Error())
    if strings.Contains(low, "resource_exhausted") || strings.Contains(low, "resourceexhausted") {
      fmt.Println("RESULT=RESOURCE_EXHAUSTED")
    } else {
      fmt.Printf("RESULT=ERROR err=%v\n", err)
    }
  } else {
    fmt.Printf("RESULT=OK RESP_LEN=%d SDK=%s\n", len(largeOut.Message), largeOut.SDK)
  }

  var smallOut PingResponse
  err = conn.Invoke(context.Background(), "/echo.v1.Echo/Ping", &PingRequest{Message: "ok"}, &smallOut, grpc.ForceCodec(jsonCodec{}))
  if err != nil {
    fmt.Printf("SMALL=ERROR err=%v\n", err)
    return
  }

  fmt.Printf("SMALL=OK SDK=%s\n", smallOut.SDK)
}
''';
