import 'dart:async';
import 'dart:io';

import 'package:fixnum/fixnum.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:holons/gen/holons/v1/observability.pbgrpc.dart' as obsgrpc;
import 'package:holons/src/connect.dart' as connect_impl;
import 'package:holons/src/gen/grpc/reflection/v1alpha/reflection.pbgrpc.dart';
import 'package:holons/src/observability.dart' as obs;
import 'package:test/test.dart';

void main() {
  group('serve', () {
    tearDown(() {
      useStaticResponse(null);
    });

    test('CurrentTransport tracks stdio serve lifecycle', () async {
      setCurrentTransport('');
      expect(CurrentTransport, isEmpty);

      final running = await startWithOptions(
        'stdio://',
        const <Service>[],
        options: const ServeOptions(
          describe: false,
          logger: _ignoreLog,
        ),
      );
      expect(CurrentTransport, equals('stdio'));

      await running.stop();
      expect(CurrentTransport, isEmpty);
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

    test('startWithOptions relays configured member observability', () async {
      final root = _writeEchoHolon();
      final fake = await _startFakeMemberObservability();

      try {
        useStaticResponse(
          buildDescribeResponse(protoDir: '${root.path}/protos'),
        );
        final running = await startWithOptions(
          'tcp://127.0.0.1:0',
          const <Service>[],
          options: ServeOptions(
            environment: const {
              'OP_OBS': 'logs,events',
              'OP_INSTANCE_UID': 'parent-uid',
            },
            logger: (_) {},
            memberEndpoints: [
              MemberRef(
                slug: 'child-x',
                address: 'tcp://127.0.0.1:${fake.server.port}',
              ),
            ],
          ),
        );

        try {
          await _waitFor(() =>
              fake.service.logsOpened == 1 && fake.service.eventsOpened == 2);
          fake.service.emitLog('relayed', {'k': 'v'});

          await _waitFor(() => obs.current().logRing!.drain().any(
                (entry) => entry.message == 'relayed',
              ));
          final entry = obs
              .current()
              .logRing!
              .drain()
              .firstWhere((entry) => entry.message == 'relayed');
          expect(entry.fields['k'], equals('v'));
          expect(entry.chain, hasLength(1));
          expect(entry.chain.single.slug, equals('child-x'));
          expect(entry.chain.single.instanceUid, equals('child-uid'));
        } finally {
          await running.stop();
        }
      } finally {
        obs.reset();
        await fake.close();
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

class _FakeMemberObservability {
  _FakeMemberObservability(this.service, this.server);

  final _FakeMemberObservabilityService service;
  final Server server;

  Future<void> close() async {
    await service.close();
    await server.shutdown();
  }
}

class _FakeMemberObservabilityService
    extends obsgrpc.HolonObservabilityServiceBase {
  final _logs = StreamController<obsgrpc.LogRecord>.broadcast();
  final _events = StreamController<obsgrpc.LogRecord>.broadcast();
  var logsOpened = 0;
  var eventsOpened = 0;

  @override
  Stream<obsgrpc.LogRecord> logs(
      ServiceCall call, obsgrpc.LogsRequest request) {
    logsOpened += 1;
    return _logs.stream;
  }

  @override
  Stream<obsgrpc.Metric> metrics(
      ServiceCall call, obsgrpc.MetricsRequest request) async* {
    yield obsgrpc.Metric(
      name: 'child_identity',
      gauge: obsgrpc.Gauge(
        dataPoints: [
          obsgrpc.NumberDataPoint(
            asInt: Int64(1),
            attributes: obs.resourceAttributes(
              const obs.Config(slug: 'child-x', instanceUid: 'child-uid'),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Stream<obsgrpc.LogRecord> events(
      ServiceCall call, obsgrpc.EventsRequest request) {
    eventsOpened += 1;
    if (!request.follow) {
      return Stream<obsgrpc.LogRecord>.value(_readyEvent());
    }
    return _events.stream;
  }

  void emitLog(String message, Map<String, String> fields) {
    _logs.add(_logRecord(message, fields));
  }

  obsgrpc.LogRecord _readyEvent() {
    final now = Int64(DateTime.now().microsecondsSinceEpoch) * Int64(1000);
    return obsgrpc.LogRecord(
      timeUnixNano: now,
      observedTimeUnixNano: now,
      severityNumber: obsgrpc.SeverityNumber.SEVERITY_NUMBER_INFO,
      severityText: 'INFO',
      body: obs.anyValue(obs.eventInstanceReady),
      attributes: obs.resourceAttributes(
        const obs.Config(slug: 'child-x', instanceUid: 'child-uid'),
      ),
      eventName: obs.eventInstanceReady,
    );
  }

  obsgrpc.LogRecord _logRecord(String message, Map<String, String> fields) {
    final now = Int64(DateTime.now().microsecondsSinceEpoch) * Int64(1000);
    return obsgrpc.LogRecord(
      timeUnixNano: now,
      observedTimeUnixNano: now,
      severityNumber: obsgrpc.SeverityNumber.SEVERITY_NUMBER_INFO,
      severityText: 'INFO',
      body: obs.anyValue(message),
      attributes: [
        ...obs.resourceAttributes(
          const obs.Config(slug: 'child-x', instanceUid: 'child-uid'),
        ),
        for (final entry in fields.entries)
          obs.keyValue(entry.key, entry.value),
      ],
    );
  }

  Future<void> close() async {
    await _logs.close();
    await _events.close();
  }
}

Future<_FakeMemberObservability> _startFakeMemberObservability() async {
  final service = _FakeMemberObservabilityService();
  final server = Server.create(services: [service]);
  await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
  return _FakeMemberObservability(service, server);
}

void _ignoreLog(String _) {}

Future<void> _waitFor(
  bool Function() condition, {
  Duration timeout = const Duration(seconds: 2),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (condition()) return;
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
  fail('condition was not met within $timeout');
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
