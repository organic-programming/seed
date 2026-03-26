import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:test/test.dart';

void main() {
  group('describe', () {
    tearDown(() {
      useStaticResponse(null);
    });

    test('buildDescribeResponse parses echo proto', () {
      final root = _writeEchoHolon();
      try {
        final response = buildDescribeResponse(
          protoDir: '${root.path}/protos',
        );
        final identity = response.manifest.identity;

        expect(identity.givenName, equals('Echo'));
        expect(identity.familyName, equals('Server'));
        expect(identity.motto, equals('Reply precisely.'));
        expect(response.services, hasLength(1));

        final service = response.services.single;
        expect(service.name, equals('echo.v1.Echo'));
        expect(
          service.description,
          equals('Echo echoes request payloads for documentation tests.'),
        );

        final method = service.methods.single;
        expect(method.name, equals('Ping'));
        expect(method.inputType, equals('echo.v1.PingRequest'));
        expect(method.outputType, equals('echo.v1.PingResponse'));
        expect(
          method.exampleInput,
          equals('{"message":"hello","sdk":"go-holons"}'),
        );

        final field = method.inputFields.first;
        expect(field.name, equals('message'));
        expect(field.type, equals('string'));
        expect(field.number, equals(1));
        expect(field.description, equals('Message to echo back.'));
        expect(field.label, equals(FieldLabel.FIELD_LABEL_OPTIONAL));
        expect(field.required, isTrue);
        expect(field.example, equals('"hello"'));
      } finally {
        root.deleteSync(recursive: true);
      }
    });

    test('HolonMeta service returns Describe response', () async {
      final root = _writeEchoHolon();
      useStaticResponse(
        buildDescribeResponse(
          protoDir: '${root.path}/protos',
        ),
      );
      final server = Server.create(
        services: <Service>[
          describeService(),
        ],
      );

      try {
        await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
        final channel = ClientChannel(
          '127.0.0.1',
          port: server.port!,
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
          expect(response.services.single.methods.single.name, equals('Ping'));
        } finally {
          await channel.shutdown();
        }
      } finally {
        await server.shutdown();
        root.deleteSync(recursive: true);
      }
    });

    test('buildDescribeResponse handles missing proto directory', () {
      final root = Directory.systemTemp.createTempSync('dart-holons-empty');
      try {
        File('${root.path}/holon.proto').writeAsStringSync(_holonProto(
          givenName: 'Silent',
          familyName: 'Holon',
          motto: 'Quietly available.',
        ));

        final response = buildDescribeResponse(
          protoDir: '${root.path}/protos',
        );
        final identity = response.manifest.identity;

        expect(identity.givenName, equals('Silent'));
        expect(identity.familyName, equals('Holon'));
        expect(identity.motto, equals('Quietly available.'));
        expect(response.services, isEmpty);
      } finally {
        root.deleteSync(recursive: true);
      }
    });

    test('describeService requires a registered static response', () {
      expect(
        describeService,
        throwsA(
          isA<DescribeRegistrationException>().having(
            (error) => error.message,
            'message',
            equals(errNoIncodeDescription),
          ),
        ),
      );
    });
  });
}

Directory _writeEchoHolon() {
  final root = Directory.systemTemp.createTempSync('dart-holons-describe');
  Directory('${root.path}/protos/echo/v1').createSync(recursive: true);
  File('${root.path}/holon.proto').writeAsStringSync(
    _holonProto(
      givenName: 'Echo',
      familyName: 'Server',
      motto: 'Reply precisely.',
    ),
  );
  File('${root.path}/protos/echo/v1/echo.proto').writeAsStringSync(
    '''
syntax = "proto3";
package echo.v1;

// Echo echoes request payloads for documentation tests.
service Echo {
  // Ping echoes the inbound message.
  // @example {"message":"hello","sdk":"go-holons"}
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  // Message to echo back.
  // @required
  // @example "hello"
  string message = 1;

  // SDK marker included in the response.
  // @example "go-holons"
  string sdk = 2;
}

message PingResponse {
  // Echoed message.
  string message = 1;

  // SDK marker from the server.
  string sdk = 2;
}
''',
  );
  return root;
}

String _holonProto({
  required String givenName,
  required String familyName,
  required String motto,
}) {
  return '''
syntax = "proto3";

package holons.test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "${givenName.toLowerCase()}-${familyName.toLowerCase()}-0000"
    given_name: "$givenName"
    family_name: "$familyName"
    motto: "$motto"
    composer: "describe-test"
    status: "draft"
    born: "2026-03-17"
  }
  lang: "dart"
};
''';
}
