import 'dart:convert';
import 'dart:io';

import 'package:holons/holons.dart';
import 'package:test/test.dart';

void main() {
  group('holon-rpc http', () {
    test('invoke uses unary POST and decodes JSON-RPC result', () async {
      final server = await _startHTTPServer((server) {
        server.register('echo.v1.Echo/Ping', (params) async => params);
      });
      addTearDown(() async {
        await server.close();
      });

      final rawClient = HttpClient();
      addTearDown(() {
        rawClient.close(force: true);
      });

      final request = await rawClient
          .postUrl(Uri.parse('${server.address}/echo.v1.Echo/Ping'));
      request.headers.contentType = ContentType.json;
      request.headers.set(HttpHeaders.acceptHeader, 'application/json');
      request.write('{"message":"hello"}');

      final response = await request.close();
      expect(response.statusCode, equals(HttpStatus.ok));
      expect(
        response.headers.contentType?.mimeType,
        equals('application/json'),
      );

      final payload = jsonDecode(
        await utf8.decoder.bind(response).join(),
      ) as Map<String, dynamic>;
      expect(
        (payload['result'] as Map<String, dynamic>)['message'],
        equals('hello'),
      );

      final client = HolonRPCHTTPClient(server.address);
      addTearDown(client.close);

      final result = await client.invoke(
        'echo.v1.Echo/Ping',
        params: const <String, dynamic>{'message': 'hola'},
      );
      expect(result['message'], equals('hola'));
    });

    test('stream uses POST SSE', () async {
      final server = await _startHTTPServer((server) {
        server.registerStream('build.v1.Build/Watch', (params, send) async {
          expect(params['project'], equals('myapp'));
          await send(<String, dynamic>{
            'status': 'building',
            'progress': 42,
          });
          await send(<String, dynamic>{
            'status': 'done',
            'progress': 100,
          });
        });
      });
      addTearDown(() async {
        await server.close();
      });

      final client = HolonRPCHTTPClient(server.address);
      addTearDown(client.close);

      final events = await client.stream(
        'build.v1.Build/Watch',
        params: const <String, dynamic>{'project': 'myapp'},
      );

      expect(events, hasLength(3));
      expect(events[0].event, equals('message'));
      expect(events[0].id, equals('1'));
      expect(events[0].result['status'], equals('building'));
      expect(events[1].result['status'], equals('done'));
      expect(events[2].event, equals('done'));
    });

    test('streamQuery uses GET SSE', () async {
      final server = await _startHTTPServer((server) {
        server.registerStream('build.v1.Build/Watch', (params, send) async {
          expect(params['project'], equals('myapp'));
          await send(<String, dynamic>{'status': 'watching'});
        });
      });
      addTearDown(() async {
        await server.close();
      });

      final client = HolonRPCHTTPClient(server.address);
      addTearDown(client.close);

      final events = await client.streamQuery(
        'build.v1.Build/Watch',
        params: const <String, String>{'project': 'myapp'},
      );

      expect(events, hasLength(2));
      expect(events[0].result['status'], equals('watching'));
      expect(events[1].event, equals('done'));
    });

    test('server answers CORS preflight', () async {
      final server = await _startHTTPServer(null);
      addTearDown(() async {
        await server.close();
      });

      final client = HttpClient();
      addTearDown(() {
        client.close(force: true);
      });

      final request = await client.openUrl(
          'OPTIONS', Uri.parse('${server.address}/echo.v1.Echo/Ping'));
      request.headers.set('origin', 'https://example.test');

      final response = await request.close();
      expect(response.statusCode, equals(HttpStatus.noContent));
      expect(
        response.headers.value('Access-Control-Allow-Origin'),
        equals('https://example.test'),
      );
      expect(
        response.headers.value('Access-Control-Allow-Methods'),
        equals('GET, POST, OPTIONS'),
      );
    });

    test('method not found returns code 5', () async {
      final server = await _startHTTPServer(null);
      addTearDown(() async {
        await server.close();
      });

      final client = HolonRPCHTTPClient(server.address);
      addTearDown(client.close);

      await expectLater(
        () => client.invoke('missing.v1.Service/Method'),
        throwsA(
          isA<HolonRPCResponseException>()
              .having((error) => error.code, 'code', equals(5)),
        ),
      );
    });
  });
}

Future<HolonRPCHTTPServer> _startHTTPServer(
  void Function(HolonRPCHTTPServer server)? register,
) async {
  final server = HolonRPCHTTPServer('http://127.0.0.1:0/api/v1/rpc');
  register?.call(server);
  await server.start();
  return server;
}
