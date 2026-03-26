import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/describe.pb.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:holons/gen/holons/v1/manifest.pb.dart';
import 'package:test/test.dart';

void main() {
  test(
    'compiled binary serves static describe without adjacent proto files',
    () async {
      if (Platform.isWindows) {
        return;
      }
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final sandbox =
          Directory.systemTemp.createTempSync('dart-holons-protoless-');
      addTearDown(() => sandbox.deleteSync(recursive: true));

      final projectDir = Directory('${sandbox.path}/fixture')..createSync();
      final runtimeDir = Directory('${sandbox.path}/runtime')..createSync();
      final sdkPath = Directory.current.absolute.path.replaceAll('\\', '/');
      _writeFixture(projectDir, sdkPath);

      final pubGet = await Process.run(
        'dart',
        const <String>['pub', 'get'],
        workingDirectory: projectDir.path,
      );
      expect(
        pubGet.exitCode,
        equals(0),
        reason:
            'dart pub get failed:\nstdout:\n${pubGet.stdout}\nstderr:\n${pubGet.stderr}',
      );

      final binaryName =
          Platform.isWindows ? 'describe-static.exe' : 'describe-static';
      final binaryPath = '${runtimeDir.path}/$binaryName';
      final compile = await Process.run(
        'dart',
        <String>['compile', 'exe', 'bin/main.dart', '-o', binaryPath],
        workingDirectory: projectDir.path,
      );
      expect(
        compile.exitCode,
        equals(0),
        reason:
            'dart compile exe failed:\nstdout:\n${compile.stdout}\nstderr:\n${compile.stderr}',
      );

      final protoFiles = runtimeDir
          .listSync(recursive: true)
          .whereType<File>()
          .where((file) => file.path.endsWith('.proto'))
          .toList();
      expect(protoFiles, isEmpty);

      final stderrBuffer = StringBuffer();
      final process = await Process.start(
        binaryPath,
        const <String>[],
        workingDirectory: runtimeDir.path,
      );
      addTearDown(() async {
        await _stopProcess(process);
      });

      final stderrSub =
          process.stderr.transform(utf8.decoder).listen(stderrBuffer.write);
      addTearDown(() async {
        await stderrSub.cancel();
      });

      final uri = await process.stdout
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .firstWhere((line) => line.startsWith('tcp://'))
          .timeout(const Duration(seconds: 30));

      final channel = ClientChannel(
        '127.0.0.1',
        port: int.parse(uri.split(':').last),
        options: const ChannelOptions(
          credentials: ChannelCredentials.insecure(),
        ),
      );
      addTearDown(() async {
        await channel.shutdown();
      });

      final response =
          await HolonMetaClient(channel).describe(DescribeRequest());
      expect(response.manifest.identity.givenName, equals('Static'));
      expect(response.manifest.identity.familyName, equals('Holon'));
      expect(response.services, hasLength(1));
      expect(response.services.single.name, equals('static.v1.Echo'));
      expect(response.services.single.methods.single.name, equals('Ping'));

      expect(
        stderrBuffer.toString(),
        contains('gRPC server listening on tcp://127.0.0.1:'),
      );
    },
    timeout: const Timeout(Duration(seconds: 120)),
  );
}

void _writeFixture(Directory projectDir, String sdkPath) {
  Directory('${projectDir.path}/bin').createSync(recursive: true);
  Directory('${projectDir.path}/gen').createSync(recursive: true);

  File('${projectDir.path}/pubspec.yaml').writeAsStringSync('''
name: describe_static_fixture
environment:
  sdk: ">=3.0.0 <4.0.0"

dependencies:
  grpc: ^5.1.0
  holons:
    path: $sdkPath
''');

  File('${projectDir.path}/bin/main.dart').writeAsStringSync('''
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

import '../gen/describe_generated.dart' as gen;

Future<void> main() async {
  useStaticResponse(gen.staticDescribeResponse());
  await runWithOptions(
    'tcp://127.0.0.1:0',
    const <Service>[],
    options: ServeOptions(
      onListen: _announce,
      logger: _log,
    ),
  );
}

void _announce(String uri) {
  stdout.writeln(uri);
}

void _log(String message) {
  stderr.writeln(message);
}
''');

  final payload = base64Encode(_sampleResponse().writeToBuffer());
  File('${projectDir.path}/gen/describe_generated.dart').writeAsStringSync('''
import 'dart:convert' show base64Decode;

import 'package:holons/gen/holons/v1/describe.pb.dart';

DescribeResponse staticDescribeResponse() {
  return DescribeResponse.fromBuffer(
    base64Decode('$payload'),
  );
}
''');
}

DescribeResponse _sampleResponse() {
  return DescribeResponse()
    ..manifest = (HolonManifest()
      ..identity = (HolonManifest_Identity()
        ..schema = 'holon/v1'
        ..uuid = 'static-holon-0000'
        ..givenName = 'Static'
        ..familyName = 'Holon'
        ..motto = 'Registered from generated code.'
        ..composer = 'describe-test'
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
