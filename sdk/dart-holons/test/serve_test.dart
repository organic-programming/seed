import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:holons/src/connect.dart' as connect_impl;
import 'package:holons/src/gen/grpc/reflection/v1alpha/reflection.pbgrpc.dart';
import 'package:test/test.dart';

void main() {
  group('serve', () {
    tearDown(() {
      useStaticResponse(null);
    });

    test(
        'startWithOptions advertises ephemeral tcp and auto-registers describe',
        () async {
      final root = _writeEchoHolon();

      try {
        useStaticResponse(
          buildDescribeResponse(protoDir: '${root.path}/protos'),
        );
        final running = await startWithOptions(
          'tcp://127.0.0.1:0',
          const <Service>[],
        );
        final port = int.parse(running.publicUri.split(':').last);
        final channel = ClientChannel(
          '127.0.0.1',
          port: port,
          options: const ChannelOptions(
            credentials: ChannelCredentials.insecure(),
          ),
        );

        try {
          final client = HolonMetaClient(channel);
          final response = await client.describe(DescribeRequest());

          expect(response.manifest.identity.givenName, equals('Echo'));
          expect(response.services, hasLength(1));
          expect(response.services.single.name, equals('echo.v1.Echo'));
        } finally {
          await channel.shutdown();
          await running.stop();
        }
      } finally {
        root.deleteSync(recursive: true);
      }
    });

    test('startWithOptions advertises unix and auto-registers describe',
        () async {
      final root = _writeEchoHolon();

      try {
        useStaticResponse(
          buildDescribeResponse(protoDir: '${root.path}/protos'),
        );
        final socketPath = connect_impl
            .defaultUnixSocketURIForTest(
              'serve-test',
              '${root.path}/serve.port',
            )
            .substring('unix://'.length);
        final running = await startWithOptions(
          'unix://$socketPath',
          const <Service>[],
        );
        final channel = await connect('unix://$socketPath');

        try {
          final client = HolonMetaClient(channel);
          final response = await client.describe(DescribeRequest());

          expect(running.publicUri, equals('unix://$socketPath'));
          expect(response.manifest.identity.givenName, equals('Echo'));
          expect(response.services, hasLength(1));
          expect(response.services.single.name, equals('echo.v1.Echo'));
        } finally {
          disconnect(channel);
          await running.stop();
        }
      } finally {
        root.deleteSync(recursive: true);
      }
    });

    test('startWithOptions registers reflection when enabled', () async {
      final root = _writeEchoHolon();

      try {
        useStaticResponse(
          buildDescribeResponse(protoDir: '${root.path}/protos'),
        );
        final running = await startWithOptions(
          'tcp://127.0.0.1:0',
          const <Service>[],
          options: ServeOptions(
            reflect: true,
            protoDir: '${root.path}/protos',
          ),
        );
        final port = int.parse(running.publicUri.split(':').last);
        final channel = ClientChannel(
          '127.0.0.1',
          port: port,
          options: const ChannelOptions(
            credentials: ChannelCredentials.insecure(),
          ),
        );

        try {
          final client = ServerReflectionClient(channel);
          final responses = await client
              .serverReflectionInfo(
                Stream<ServerReflectionRequest>.value(
                  ServerReflectionRequest()..listServices = '*',
                ),
              )
              .toList();

          final services = responses
              .expand((response) => response.listServicesResponse.service)
              .map((service) => service.name)
              .toList();
          expect(services, contains('echo.v1.Echo'));
        } finally {
          await channel.shutdown();
          await running.stop();
        }
      } finally {
        root.deleteSync(recursive: true);
      }
    });

    test('startWithOptions fails loudly when no static describe is registered',
        () async {
      final logs = <String>[];

      await expectLater(
        () => startWithOptions(
          'tcp://127.0.0.1:0',
          const <Service>[],
          options: ServeOptions(
            logger: logs.add,
          ),
        ),
        throwsA(
          isA<DescribeRegistrationException>().having(
            (error) => error.message,
            'message',
            equals(errNoIncodeDescription),
          ),
        ),
      );

      expect(
        logs,
        contains('HolonMeta registration failed: $errNoIncodeDescription'),
      );
    });
  });
}

Directory _writeEchoHolon() {
  final root = Directory.systemTemp.createTempSync('dart-holons-serve');
  Directory('${root.path}/protos/echo/v1').createSync(recursive: true);
  File('${root.path}/holon.proto').writeAsStringSync(
    '''
syntax = "proto3";

package holons.test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "echo-server-0000"
    given_name: "Echo"
    family_name: "Server"
    motto: "Reply precisely."
    composer: "serve-test"
    status: "draft"
    born: "2026-03-17"
  }
  lang: "dart"
};
''',
  );
  File('${root.path}/protos/echo/v1/echo.proto').writeAsStringSync(
    '''
syntax = "proto3";
package echo.v1;

service Echo {
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  string message = 1;
}

message PingResponse {
  string message = 1;
}
''',
  );
  return root;
}
