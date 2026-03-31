import 'dart:io';

import 'package:test/test.dart';
import 'package:holons/holons.dart';
import 'package:holons/src/identity.dart' as identity;

void main() {
  group('transport', () {
    test('scheme extracts transport scheme', () {
      expect(scheme('tcp://:9090'), equals('tcp'));
      expect(scheme('unix:///tmp/x.sock'), equals('unix'));
      expect(scheme('stdio://'), equals('stdio'));
      expect(scheme('ws://127.0.0.1:8080/grpc'), equals('ws'));
      expect(scheme('wss://example.com:443/grpc'), equals('wss'));
    });

    test('defaultUri is tcp://:9090', () {
      expect(defaultUri, equals('tcp://:9090'));
    });

    test('listen tcp', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final listener = await listen('tcp://127.0.0.1:0');
      expect(listener, isA<TcpTransportListener>());
      final tcp = listener as TcpTransportListener;
      expect(tcp.socket.port, greaterThan(0));
      await tcp.socket.close();
    });

    test('parseUri wss defaults', () {
      final parsed = parseUri('wss://example.com:8443');
      expect(parsed.scheme, equals('wss'));
      expect(parsed.host, equals('example.com'));
      expect(parsed.port, equals(8443));
      expect(parsed.path, equals('/grpc'));
      expect(parsed.secure, isTrue);
    });

    test('stdio variant', () async {
      final stdio = await listen('stdio://');
      expect(stdio, isA<StdioTransportListener>());
      expect((stdio as StdioTransportListener).address, equals('stdio://'));
    });

    test('ws variant', () async {
      final listener = await listen('ws://127.0.0.1:8080/holon');
      expect(listener, isA<WsTransportListener>());
      final ws = listener as WsTransportListener;
      expect(ws.host, equals('127.0.0.1'));
      expect(ws.port, equals(8080));
      expect(ws.path, equals('/holon'));
      expect(ws.secure, isFalse);
    });

    test('unsupported uri throws', () {
      expect(listen('ftp://host'), throwsArgumentError);
    });
  });

  group('runtime transport', () {
    test('runtime tcp roundtrip', () async {
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final runtime = await listenRuntime('tcp://127.0.0.1:0');
      expect(runtime, isA<TcpRuntimeListener>());
      final tcp = runtime as TcpRuntimeListener;

      final acceptedFuture = tcp.accept();
      final client = await Socket.connect('127.0.0.1', tcp.socket.port);
      final server = await acceptedFuture;

      client.add('ping'.codeUnits);
      await client.flush();

      final received = await server.read(maxBytes: 4);
      expect(String.fromCharCodes(received), equals('ping'));

      await server.close();
      await client.close();
      await tcp.close();
    });

    test('runtime unix roundtrip', () async {
      if (Platform.isWindows) {
        return;
      }
      if (!await _canBindLoopbackTcp()) {
        return;
      }

      final socketPath =
          '${Directory.systemTemp.path}/holons_dart_${DateTime.now().microsecondsSinceEpoch}.sock';
      RuntimeTransportListener runtime;
      try {
        runtime = await listenRuntime('unix://$socketPath');
      } on SocketException catch (error) {
        if (_isLocalBindDenied(error)) {
          return;
        }
        rethrow;
      }
      expect(runtime, isA<UnixRuntimeListener>());
      final unix = runtime as UnixRuntimeListener;

      final acceptedFuture = unix.accept();
      final client = await Socket.connect(
          InternetAddress(socketPath, type: InternetAddressType.unix), 0);
      final server = await acceptedFuture;

      client.add('unix'.codeUnits);
      await client.flush();

      final received = await server.read(maxBytes: 4);
      expect(String.fromCharCodes(received), equals('unix'));

      await server.close();
      await client.close();
      await unix.close();
    });

    test('runtime stdio only accepts once', () async {
      final runtime = await listenRuntime('stdio://');
      expect(runtime, isA<StdioRuntimeListener>());
      final stdio = runtime as StdioRuntimeListener;

      final conn = await stdio.accept();
      await conn.close();

      expect(stdio.accept(), throwsStateError);
      await stdio.close();
    });

    test('runtime ws unsupported', () {
      expect(listenRuntime('ws://127.0.0.1:8080/grpc'),
          throwsA(isA<UnsupportedError>()));
    });
  });

  group('serve', () {
    test('parseFlags --listen', () {
      expect(parseFlags(['--listen', 'tcp://:8080']), equals('tcp://:8080'));
    });

    test('parseFlags --port', () {
      expect(parseFlags(['--port', '3000']), equals('tcp://:3000'));
    });

    test('parseFlags default', () {
      expect(parseFlags([]), equals(defaultUri));
    });

    test('parseOptions captures --reflect', () {
      final parsed = parseOptions(['--listen', 'tcp://:8080', '--reflect']);
      expect(parsed.listenUri, equals('tcp://:8080'));
      expect(parsed.reflect, isTrue);
    });
  });

  group('identity', () {
    test('resolve and resolveProtoFile expose the resolved manifest', () {
      final root =
          Directory.systemTemp.createTempSync('holon_identity_resolve_');
      addTearDown(() => root.deleteSync(recursive: true));

      final holonDir = Directory('${root.path}/protoholon')..createSync();
      final manifest = File('${holonDir.path}/holon.proto');
      manifest.writeAsStringSync(
        'syntax = "proto3";\n'
        '\n'
        'package test.v1;\n'
        '\n'
        'option (holons.v1.manifest) = {\n'
        '  identity: {\n'
        '    uuid: "test-uuid-1234"\n'
        '    given_name: "gabriel"\n'
        '    family_name: "Greeting-Go"\n'
        '    motto: "Test greeting holon."\n'
        '    proto_status: "draft"\n'
        '  }\n'
        '  lineage: {\n'
        '    parents: ["parent-a"]\n'
        '    reproduction: "assisted"\n'
        '    generated_by: "op"\n'
        '  }\n'
        '  lang: "go"\n'
        '};\n',
      );

      final resolved = identity.resolve(holonDir.path);
      final direct = resolveProtoFile(manifest.path);

      expect(resolved.identity.uuid, equals('test-uuid-1234'));
      expect(resolved.identity.givenName, equals('gabriel'));
      expect(resolved.identity.familyName, equals('Greeting-Go'));
      expect(resolved.identity.motto, equals('Test greeting holon.'));
      expect(resolved.identity.lang, equals('go'));
      expect(resolved.identity.reproduction, equals('assisted'));
      expect(resolved.identity.generatedBy, equals('op'));
      expect(resolved.identity.parents, equals(<String>['parent-a']));
      expect(resolved.identity.slug(), equals('gabriel-greeting-go'));
      expect(
        resolved.sourcePath,
        equals(manifest.absolute.path.replaceAll('\\', '/')),
      );

      expect(direct.sourcePath, equals(resolved.sourcePath));
      expect(direct.identity.slug(), equals('gabriel-greeting-go'));
    });

    test('parseHolon parses holon.proto', () {
      final tmp = File('${Directory.systemTemp.path}/holon_dart.proto');
      tmp.writeAsStringSync(
        'syntax = "proto3";\n'
        '\n'
        'package test.v1;\n'
        '\n'
        'option (holons.v1.manifest) = {\n'
        '  identity: {\n'
        '    uuid: "abc-123"\n'
        '    given_name: "test"\n'
        '    family_name: "Test"\n'
        '    proto_status: "draft"\n'
        '    aliases: ["d1"]\n'
        '  }\n'
        '  lineage: {\n'
        '    parents: ["a", "b"]\n'
        '    generated_by: "dummy-test"\n'
        '  }\n'
        '  lang: "dart"\n'
        '};\n',
      );

      final id = parseHolon(tmp.path);
      expect(id.uuid, equals('abc-123'));
      expect(id.givenName, equals('test'));
      expect(id.lang, equals('dart'));
      expect(id.parents, equals(<String>['a', 'b']));
      expect(id.generatedBy, equals('dummy-test'));
      expect(id.protoStatus, equals('draft'));
      expect(id.aliases, equals(<String>['d1']));

      tmp.deleteSync();
    });

    test('parseHolon throws when holon.proto is missing the manifest option',
        () {
      final tmp = File('${Directory.systemTemp.path}/invalid_holon_dart.proto');
      tmp.writeAsStringSync('syntax = "proto3";\n\npackage test.v1;\n');
      expect(() => parseHolon(tmp.path), throwsFormatException);
      tmp.deleteSync();
    });

    test('slug trims a trailing question mark', () {
      const identity = HolonIdentity(
        givenName: 'Rob',
        familyName: 'Go?',
      );

      expect(identity.slug(), equals('rob-go'));
    });
  });
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
