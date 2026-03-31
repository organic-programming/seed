import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:holons/gen/holons/v1/describe.pb.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:holons/gen/holons/v1/manifest.pb.dart' as manifestpb;
import 'package:test/test.dart';

void main() {
  group('transport audit', () {
    tearDown(() {
      useStaticResponse(null);
    });

    test('tcp transport supports direct dial to a running server', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      useStaticResponse(_sampleDescribeResponse());
      final running = await startWithOptions(
        'tcp://127.0.0.1:0',
        const <Service>[],
      );
      addTearDown(() async {
        await running.stop();
      });

      final channel = await connect(running.publicUri);
      addTearDown(() async {
        disconnect(channel);
      });

      final response =
          await HolonMetaClient(channel).describe(DescribeRequest());
      expect(response.manifest.identity.givenName, equals('Static'));
      expect(response.services.single.name, equals('static.v1.Echo'));
    });

    test('stdio transport serves gRPC over process stdio', () async {
      final runner = _writeStdioRunner();
      addTearDown(() {
        if (runner.existsSync()) {
          runner.deleteSync();
        }
      });

      final stderrBuffer = StringBuffer();
      final process = await Process.start(
        Platform.resolvedExecutable,
        <String>['run', runner.path],
        workingDirectory: Directory.current.path,
      );
      addTearDown(() async {
        await _stopProcess(process);
      });

      final stderrSub =
          process.stderr.transform(utf8.decoder).listen(stderrBuffer.write);
      addTearDown(() async {
        await stderrSub.cancel();
      });

      final channel = ClientTransportConnectorChannel(
        StdioTransportConnector.fromProcess(process),
        options: const ChannelOptions(
          credentials: ChannelCredentials.insecure(),
          idleTimeout: null,
        ),
      );
      addTearDown(() async {
        await channel.shutdown();
      });

      await _waitForDescribe(channel);
      final response =
          await HolonMetaClient(channel).describe(DescribeRequest());
      expect(response.manifest.identity.givenName, equals('Static'));
      expect(response.services.single.methods.single.name, equals('Ping'));
      expect(stderrBuffer.toString(),
          contains('gRPC server listening on stdio://'));
    });

    test('wss transport supports HolonRPC dial', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }
      if (!_hasOpenSSL()) {
        return;
      }

      final certDir = Directory.systemTemp.createTempSync('dart-holons-wss-');
      addTearDown(() => certDir.deleteSync(recursive: true));
      final certPath = '${certDir.path}/cert.pem';
      final keyPath = '${certDir.path}/key.pem';

      final certGen = await Process.run(
        'openssl',
        <String>[
          'req',
          '-x509',
          '-newkey',
          'rsa:2048',
          '-nodes',
          '-keyout',
          keyPath,
          '-out',
          certPath,
          '-subj',
          '/CN=127.0.0.1',
          '-days',
          '1',
        ],
      );
      expect(
        certGen.exitCode,
        equals(0),
        reason:
            'openssl failed:\nstdout:\n${certGen.stdout}\nstderr:\n${certGen.stderr}',
      );

      final context = SecurityContext()
        ..useCertificateChain(certPath)
        ..usePrivateKey(keyPath);
      final server = await HttpServer.bindSecure(
        InternetAddress.loopbackIPv4.address,
        0,
        context,
      );
      addTearDown(() async {
        await server.close(force: true);
      });

      final serverSub = server.listen((request) async {
        if (request.uri.path != '/rpc') {
          request.response.statusCode = HttpStatus.notFound;
          await request.response.close();
          return;
        }

        final socket = await WebSocketTransformer.upgrade(
          request,
          protocolSelector: (protocols) {
            if (protocols.contains('holon-rpc')) {
              return 'holon-rpc';
            }
            return null;
          },
        );

        socket.listen((data) async {
          final payload = jsonDecode(data as String) as Map<String, dynamic>;
          final id = payload['id'];
          final method = payload['method'];
          if (id == null || method != 'echo.v1.Echo/Ping') {
            return;
          }

          socket.add(
            jsonEncode(<String, Object?>{
              'jsonrpc': '2.0',
              'id': id,
              'result': <String, Object?>{
                'message':
                    (payload['params'] as Map<String, dynamic>)['message'],
              },
            }),
          );
        });
      });
      addTearDown(() async {
        await serverSub.cancel();
      });

      final client = HolonRPCClient(
        heartbeatIntervalMs: 60000,
        heartbeatTimeoutMs: 5000,
      );
      addTearDown(() async {
        await client.close();
      });

      await HttpOverrides.runWithHttpOverrides(() async {
        await client.connect(
          'wss://127.0.0.1:${server.port}/rpc',
        );
        final result = await client.invoke(
          'echo.v1.Echo/Ping',
          params: const <String, dynamic>{'message': 'hello-wss'},
        );
        expect(result['message'], equals('hello-wss'));
      }, _TrustAllHttpOverrides());
    });

    test('rest+sse transport supports HTTP+SSE client dial', () async {
      final server = HolonRPCHTTPServer('http://127.0.0.1:0/api/v1/rpc');
      server.register(
        'echo.v1.Echo/Ping',
        (params) async => params,
      );
      addTearDown(() async {
        await server.close();
      });

      await server.start();

      final client = HolonRPCHTTPClient(server.address);
      addTearDown(client.close);

      final result = await client.invoke(
        'echo.v1.Echo/Ping',
        params: const <String, dynamic>{'message': 'hello-http'},
      );
      expect(result['message'], equals('hello-http'));
    });
  });
}

DescribeResponse _sampleDescribeResponse() {
  return DescribeResponse()
    ..manifest = (manifestpb.HolonManifest()
      ..identity = (manifestpb.HolonManifest_Identity()
        ..schema = 'holon/v1'
        ..uuid = 'static-holon-0000'
        ..givenName = 'Static'
        ..familyName = 'Holon'
        ..motto = 'Registered from generated code.'
        ..composer = 'transport-test'
        ..status = 'draft'
        ..born = '2026-03-23')
      ..lang = 'dart')
    ..services.add(
      ServiceDoc()
        ..name = 'static.v1.Echo'
        ..description = 'Static test service.'
        ..methods.add(
          MethodDoc()
            ..name = 'Ping'
            ..description = 'Replies with the payload.',
        ),
    );
}

File _writeStdioRunner() {
  final payload = base64Encode(_sampleDescribeResponse().writeToBuffer());
  final runner = File(
    '${Directory.current.path}/.dart_tool/stdio-transport-${DateTime.now().microsecondsSinceEpoch}.dart',
  );
  runner.parent.createSync(recursive: true);
  runner.writeAsStringSync('''
import 'dart:convert' show base64Decode;
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:holons/gen/holons/v1/describe.pb.dart';

Future<void> main() async {
  useStaticResponse(DescribeResponse.fromBuffer(base64Decode('$payload')));
  await runWithOptions(
    'stdio://',
    const <Service>[],
    options: ServeOptions(
      onListen: _discard,
      logger: _log,
    ),
  );
}

void _discard(String _) {}

void _log(String message) {
  stderr.writeln(message);
}
''');
  return runner;
}

Future<void> _waitForDescribe(ClientTransportConnectorChannel channel) async {
  final client = HolonMetaClient(channel);
  final deadline = DateTime.now().add(const Duration(seconds: 10));

  while (DateTime.now().isBefore(deadline)) {
    try {
      await client.describe(
        DescribeRequest(),
        options: CallOptions(timeout: const Duration(seconds: 1)),
      );
      return;
    } on Object {
      await Future<void>.delayed(const Duration(milliseconds: 100));
    }
  }

  throw StateError('timed out waiting for stdio describe readiness');
}

Future<bool> _canBindLoopbackTcp() async {
  try {
    final probe = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    await probe.close();
    return true;
  } on SocketException catch (error) {
    if (_isLocalBindDenied(error)) {
      return false;
    }
    rethrow;
  }
}

bool _isLocalBindDenied(Object error) {
  final text = error.toString().toLowerCase();
  return text.contains('operation not permitted') ||
      text.contains('permission denied') ||
      text.contains('errno = 1');
}

bool _hasOpenSSL() {
  try {
    final result = Process.runSync('openssl', const <String>['version']);
    return result.exitCode == 0;
  } on ProcessException {
    return false;
  }
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

class _TrustAllHttpOverrides extends HttpOverrides {
  @override
  HttpClient createHttpClient(SecurityContext? context) {
    final client = super.createHttpClient(context);
    client.badCertificateCallback = (_, __, ___) => true;
    return client;
  }
}
