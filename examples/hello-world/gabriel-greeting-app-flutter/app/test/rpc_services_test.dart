import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/coax.pbgrpc.dart';
import 'package:holons/holons.dart' as holons;
import 'package:holons_app/holons_app.dart';

import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/gen/v1/holon.pbgrpc.dart';
import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';
import 'package:gabriel_greeting_app_flutter/src/runtime/greeting_holon_connection.dart';

import 'support/fakes.dart';

void main() {
  test('COAX and GreetingApp RPC services drive the shared state', () async {
    holons.reset();
    final obs = holons.fromEnv(
      const holons.Config(slug: 'gabriel-greeting-app-flutter-test'),
      const {'OP_OBS': 'logs,metrics'},
    );
    addTearDown(holons.reset);

    final connection = FakeGreetingHolonConnection(
      languages: [
        language(code: 'en', name: 'English', native: 'English'),
        language(code: 'fr', name: 'French', native: 'Francais'),
      ],
      greetingBuilder: ({required name, required langCode}) {
        if (langCode == 'fr') {
          return 'Bonjour $name from Gabriel';
        }
        return 'Hello $name from Gabriel';
      },
    );
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([
        holon('gabriel-greeting-go'),
        holon('gabriel-greeting-swift'),
      ]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-go': (_) => connection,
          'gabriel-greeting-swift': (_) => connection,
        },
      ),
    );
    final coaxController = buildCoaxManager(
      greetingController: greetingController,
    );

    await greetingController.initialize();
    final port = await reserveTcpPort();
    await coaxController.setServerPortText(port.toString());
    await coaxController.setEnabled(true);

    final channel = clientChannelFromListenUri(coaxController.listenUri!);
    addTearDown(() async {
      await channel.shutdown();
      await coaxController.shutdown();
      await greetingController.shutdown();
    });

    final coaxClient = CoaxServiceClient(channel);
    final appClient = GreetingAppServiceClient(channel);
    final metaClient = HolonMetaClient(channel);

    final describe = await metaClient.describe(DescribeRequest());
    expect(describe.manifest.identity.familyName, 'Greeting-App-Flutter');
    expect(
      describe.services.map((service) => service.name),
      containsAll(<String>[
        'holons.v1.HolonMeta',
        'holons.v1.CoaxService',
        'greeting.v1.GreetingAppService',
      ]),
    );
    expect(
      describe.services
          .firstWhere((service) => service.name == 'holons.v1.HolonMeta')
          .methods
          .map((method) => method.name),
      contains('Describe'),
    );

    final members = await coaxClient.listMembers(ListMembersRequest());
    expect(
      members.members.map((member) => member.slug),
      containsAll(<String>['gabriel-greeting-go', 'gabriel-greeting-swift']),
    );

    final selectHolon = await appClient.selectHolon(
      SelectHolonRequest(slug: 'gabriel-greeting-go'),
    );
    expect(selectHolon.slug, 'gabriel-greeting-go');
    expect(greetingController.selectedHolon?.slug, 'gabriel-greeting-go');
    expect(greetingController.greeting, 'Hello World from Gabriel');

    final selectLanguage = await appClient.selectLanguage(
      SelectLanguageRequest(code: 'fr'),
    );
    expect(selectLanguage.code, 'fr');
    expect(greetingController.greeting, 'Bonjour World from Gabriel');

    final greeting = await appClient.greet(
      GreetRequest(name: 'Alice', langCode: 'fr'),
    );
    expect(greeting.greeting, 'Bonjour Alice from Gabriel');
    expect(greetingController.greeting, 'Bonjour Alice from Gabriel');

    final entry = obs.logRing!.drain().singleWhere(
      (log) => _logMessage(log) == 'Greeted Alice in French (fr)',
    );
    final fields = _logFields(entry);
    expect(
      fields.keys,
      containsAll(<String>[
        'lang_code',
        'language',
        'name',
        'greeting',
        'transport',
        'duration_ns',
      ]),
    );
    expect(fields['lang_code'], 'fr');
    expect(fields['language'], 'French');
    expect(fields['name'], 'Alice');
    expect(fields['greeting'], 'Bonjour Alice from Gabriel');
    expect(fields['transport'], 'unknown');
    final durationNs = fields['duration_ns'];
    expect(durationNs, isA<int>());
    expect(durationNs as int, greaterThanOrEqualTo(0));
    _expectDurationNsIntValue(entry);

    final counter = obs.registry!.listCounters().singleWhere(
      (metric) => metric.name == 'greeting_emitted_total',
    );
    expect(counter.value(), 1);
    expect(counter.labels, <String, String>{
      'lang_code': 'fr',
      'language': 'French',
      'transport': 'unknown',
    });

    final status = await coaxClient.memberStatus(
      MemberStatusRequest(slug: 'gabriel-greeting-go'),
    );
    expect(status.member.state, MemberState.MEMBER_STATE_CONNECTED);

    final tell = await coaxClient.tell(
      TellRequest(
        memberSlug: 'gabriel-greeting-go',
        method: 'greeting.v1.GreetingService/SayHello',
        payload: utf8.encode('{"name":"Alice","langCode":"fr"}'),
      ),
    );
    expect(jsonDecode(utf8.decode(tell.payload)), <String, Object?>{
      'greeting': 'Bonjour Alice from Gabriel',
    });
    expect(greetingController.userName, 'Alice');
    expect(greetingController.selectedLanguageCode, 'fr');
    expect(greetingController.greeting, 'Bonjour Alice from Gabriel');

    await coaxClient.disconnectMember(
      DisconnectMemberRequest(slug: 'gabriel-greeting-go'),
    );
    expect(greetingController.isRunning, isFalse);

    final disconnectedStatus = await coaxClient.memberStatus(
      MemberStatusRequest(slug: 'gabriel-greeting-go'),
    );
    expect(disconnectedStatus.member.state, MemberState.MEMBER_STATE_AVAILABLE);

    await coaxClient.turnOffCoax(TurnOffCoaxRequest());
    await waitForCoaxUpdate();

    expect(coaxController.isEnabled, isFalse);
  });

  test(
    'ConnectMember retries transient tcp startup and leaves the member ready',
    () async {
      var tcpAttempts = 0;
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([
          holon('gabriel-greeting-swift'),
          holon('gabriel-greeting-c'),
        ]),
        connector: _ScriptedHolonConnector((holon, transport) {
          if (holon.slug == 'gabriel-greeting-c' &&
              transport == 'tcp' &&
              tcpAttempts++ == 0) {
            throw StateError('temporary tcp startup race');
          }
          return FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
              language(code: 'fr', name: 'French', native: 'Francais'),
            ],
            greetingBuilder: ({required name, required langCode}) {
              return langCode == 'fr' ? 'Bonjour $name' : 'Hello $name';
            },
          );
        }),
        initialTransport: 'stdio',
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final coaxClient = CoaxServiceClient(channel);
      final appClient = GreetingAppServiceClient(channel);

      final member = await coaxClient.connectMember(
        ConnectMemberRequest(slug: 'gabriel-greeting-c', transport: 'tcp'),
      );
      expect(member.member.state, MemberState.MEMBER_STATE_CONNECTED);
      expect(greetingController.selectedHolon?.slug, 'gabriel-greeting-c');
      expect(greetingController.transport, 'tcp');
      expect(greetingController.availableLanguages, isNotEmpty);
      expect(greetingController.greeting, 'Hello World');
      expect(tcpAttempts, 2);

      final greeting = await appClient.greet(
        GreetRequest(name: 'Bob', langCode: 'fr'),
      );
      expect(greeting.greeting, 'Bonjour Bob');
    },
  );

  test(
    'SelectTransport switches the active transport and reloads the holon',
    () async {
      final connector = FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (transport) => FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
              language(code: 'fr', name: 'French', native: 'Francais'),
            ],
            greetingBuilder: ({required name, required langCode}) =>
                '$transport:$langCode:$name',
          ),
        },
      );
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
        connector: connector,
        initialTransport: 'stdio',
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      final response = await appClient.selectTransport(
        SelectTransportRequest(transport: 'tcp'),
      );

      expect(response.transport, 'tcp');
      expect(greetingController.transport, 'tcp');
      expect(greetingController.isRunning, isTrue);
      expect(greetingController.availableLanguages, isNotEmpty);
      expect(greetingController.greeting, 'tcp:en:World');
      expect(
        connector.connectCalls,
        containsAllInOrder(<(String, String)>[
          ('gabriel-greeting-swift', 'stdio'),
          ('gabriel-greeting-swift', 'tcp'),
        ]),
      );
    },
  );

  test('SelectTransport rejects invalid transport names', () async {
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
            ],
            greetingBuilder: ({required name, required langCode}) =>
                'Hello $name',
          ),
        },
      ),
    );
    final coaxController = buildCoaxManager(
      greetingController: greetingController,
    );

    await greetingController.initialize();
    final port = await reserveTcpPort();
    await coaxController.setServerPortText(port.toString());
    await coaxController.setEnabled(true);

    final channel = clientChannelFromListenUri(coaxController.listenUri!);
    addTearDown(() async {
      await channel.shutdown();
      await coaxController.shutdown();
      await greetingController.shutdown();
    });

    final appClient = GreetingAppServiceClient(channel);
    await expectLater(
      appClient.selectTransport(SelectTransportRequest(transport: '2')),
      throwsA(
        isA<GrpcError>().having(
          (error) => error.code,
          'code',
          StatusCode.invalidArgument,
        ),
      ),
    );
  });

  test(
    'SelectLanguage rejects invalid codes and preserves the current selection',
    () async {
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
        connector: FakeHolonConnector(
          factories: <String, FakeGreetingHolonConnection Function(String)>{
            'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
              languages: [
                language(code: 'en', name: 'English', native: 'English'),
                language(code: 'fr', name: 'French', native: 'Francais'),
              ],
              greetingBuilder: ({required name, required langCode}) =>
                  '$langCode:$name',
            ),
          },
        ),
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      final selected = await appClient.selectLanguage(
        SelectLanguageRequest(code: 'fr'),
      );

      expect(selected.code, 'fr');
      expect(greetingController.selectedLanguageCode, 'fr');
      expect(greetingController.greeting, 'fr:World');

      await expectLater(
        appClient.selectLanguage(SelectLanguageRequest(code: 'zz')),
        throwsA(
          isA<GrpcError>().having(
            (error) => error.code,
            'code',
            StatusCode.invalidArgument,
          ),
        ),
      );

      expect(greetingController.selectedLanguageCode, 'fr');
      expect(greetingController.greeting, 'fr:World');

      final greeting = await appClient.greet(GreetRequest(name: 'Alice'));
      expect(greeting.greeting, 'fr:Alice');
    },
  );

  test(
    'SelectTransport rejects unix when the platform does not support it',
    () async {
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
        connector: FakeHolonConnector(
          factories: <String, FakeGreetingHolonConnection Function(String)>{
            'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
              languages: [
                language(code: 'en', name: 'English', native: 'English'),
              ],
              greetingBuilder: ({required name, required langCode}) =>
                  'Hello $name',
            ),
          },
        ),
        capabilities: const AppPlatformCapabilities(supportsUnixSockets: false),
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      await expectLater(
        appClient.selectTransport(SelectTransportRequest(transport: 'unix')),
        throwsA(
          isA<GrpcError>().having(
            (error) => error.code,
            'code',
            StatusCode.invalidArgument,
          ),
        ),
      );
    },
  );

  test(
    'SelectTransport returns the connection error when reconnect fails',
    () async {
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
        connector: FakeHolonConnector(
          factories: <String, FakeGreetingHolonConnection Function(String)>{
            'gabriel-greeting-swift': (transport) {
              if (transport == 'tcp') {
                throw StateError('tcp dial failed');
              }
              return FakeGreetingHolonConnection(
                languages: [
                  language(code: 'en', name: 'English', native: 'English'),
                ],
                greetingBuilder: ({required name, required langCode}) =>
                    'Hello $name',
              );
            },
          },
        ),
        initialTransport: 'stdio',
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      await expectLater(
        appClient.selectTransport(SelectTransportRequest(transport: 'tcp')),
        throwsA(
          isA<GrpcError>()
              .having((error) => error.code, 'code', StatusCode.unavailable)
              .having(
                (error) => error.message,
                'message',
                contains('Failed to start Gabriel holon'),
              ),
        ),
      );
    },
  );
}

String _logMessage(dynamic entry) {
  final body = _recordBody(entry);
  if (body != null) {
    return body;
  }
  return entry.message as String;
}

Map<String, Object?> _logFields(dynamic entry) {
  final attributes = _recordAttributes(entry);
  if (attributes != null) {
    return attributes;
  }

  final fields = entry.fields as Map<String, String>;
  return <String, Object?>{
    for (final item in fields.entries)
      item.key: item.key == 'duration_ns' ? int.parse(item.value) : item.value,
  };
}

String? _recordBody(dynamic entry) {
  try {
    final dynamic record = entry.record;
    final dynamic body = record.body;
    return body.stringValue as String;
  } on Object {
    return null;
  }
}

Map<String, Object?>? _recordAttributes(dynamic entry) {
  try {
    final dynamic record = entry.record;
    final out = <String, Object?>{};
    for (final dynamic attribute in record.attributes) {
      out[attribute.key as String] = _anyValue(attribute.value);
    }
    return out;
  } on Object {
    return null;
  }
}

Object? _anyValue(dynamic value) {
  try {
    if (value.hasStringValue() as bool) return value.stringValue as String;
    if (value.hasBoolValue() as bool) return value.boolValue as bool;
    if (value.hasIntValue() as bool) return (value.intValue as dynamic).toInt();
    if (value.hasDoubleValue() as bool) return value.doubleValue as double;
  } on Object {
    return null;
  }
  return null;
}

void _expectDurationNsIntValue(dynamic entry) {
  final dynamic duration = _recordAttributeValue(entry, 'duration_ns');
  if (duration == null) {
    return;
  }
  expect(duration.hasIntValue() as bool, isTrue);
}

Object? _recordAttributeValue(dynamic entry, String key) {
  try {
    final dynamic record = entry.record;
    return record.attributes
        .singleWhere((dynamic attribute) => attribute.key == key)
        .value;
  } on Object {
    // The pre-OTLP Dart SDK stores in-memory fields as strings. This assertion
    // activates once the SDK worker lands the LogRecord-backed ring.
    return null;
  }
}

class _ScriptedHolonConnector implements GreetingHolonConnectionFactory {
  _ScriptedHolonConnector(this._onConnect);

  final GreetingHolonConnection Function(
    GabrielHolonIdentity holon,
    String transport,
  )
  _onConnect;

  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    return _onConnect(holon, transport);
  }
}
