import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/observability.pb.dart' as obspb;
import 'package:holons/holons.dart' as holons;
import 'package:test/test.dart';

import '../gen/dart/greeting/v1/greeting.pbgrpc.dart';
import 'server.dart';

void main() {
  test('RPC ListLanguages returns all languages', () async {
    final server = Server.create(services: <Service>[GreetingService()]);
    await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
    addTearDown(server.shutdown);

    final channel = ClientChannel(
      '127.0.0.1',
      port: server.port!,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
    addTearDown(channel.shutdown);

    final client = GreetingServiceClient(channel);
    final response = await client.listLanguages(ListLanguagesRequest());

    expect(response.languages, hasLength(56));
  });

  test('RPC ListLanguages populates required fields', () async {
    final server = Server.create(services: <Service>[GreetingService()]);
    await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
    addTearDown(server.shutdown);

    final channel = ClientChannel(
      '127.0.0.1',
      port: server.port!,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
    addTearDown(channel.shutdown);

    final client = GreetingServiceClient(channel);
    final response = await client.listLanguages(ListLanguagesRequest());

    for (final language in response.languages) {
      expect(language.code, isNotEmpty);
      expect(language.name, isNotEmpty);
      expect(language.native, isNotEmpty);
    }
  });

  test('RPC SayHello uses requested language', () async {
    final server = Server.create(services: <Service>[GreetingService()]);
    await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
    addTearDown(server.shutdown);

    final channel = ClientChannel(
      '127.0.0.1',
      port: server.port!,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
    addTearDown(channel.shutdown);

    final client = GreetingServiceClient(channel);
    final response = await client.sayHello(
      SayHelloRequest()
        ..name = 'Bob'
        ..langCode = 'fr',
    );

    expect(response.greeting, equals('Bonjour Bob'));
    expect(response.language, equals('French'));
    expect(response.langCode, equals('fr'));
  });

  test('RPC SayHello uses localized default name', () async {
    final server = Server.create(services: <Service>[GreetingService()]);
    await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
    addTearDown(server.shutdown);

    final channel = ClientChannel(
      '127.0.0.1',
      port: server.port!,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
    addTearDown(channel.shutdown);

    final client = GreetingServiceClient(channel);
    final response = await client.sayHello(
      SayHelloRequest()..langCode = 'fr',
    );

    expect(response.greeting, equals('Bonjour Marie'));
    expect(response.langCode, equals('fr'));
  });

  test('RPC SayHello falls back to English', () async {
    final server = Server.create(services: <Service>[GreetingService()]);
    await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
    addTearDown(server.shutdown);

    final channel = ClientChannel(
      '127.0.0.1',
      port: server.port!,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
    addTearDown(channel.shutdown);

    final client = GreetingServiceClient(channel);
    final response = await client.sayHello(
      SayHelloRequest()
        ..name = 'Bob'
        ..langCode = 'xx',
    );

    expect(response.greeting, equals('Hello Bob'));
    expect(response.langCode, equals('en'));
  });

  test('RPC SayHello emits greeting observability at the handler boundary',
      () async {
    holons.reset();
    final obs = holons.fromEnv(
      const holons.Config(
        slug: 'gabriel-greeting-dart-test',
        instanceUid: 'greeting-test-uid',
      ),
      const {'OP_OBS': 'logs,metrics'},
    );
    holons.setCurrentTransport('stdio');
    addTearDown(() {
      holons.setCurrentTransport('');
      holons.reset();
    });

    final server = Server.create(services: <Service>[GreetingService()]);
    await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
    addTearDown(server.shutdown);

    final channel = ClientChannel(
      '127.0.0.1',
      port: server.port!,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
    addTearDown(channel.shutdown);

    final client = GreetingServiceClient(channel);
    final response = await client.sayHello(
      SayHelloRequest()
        ..name = 'Bob'
        ..langCode = 'fr',
    );

    expect(response.greeting, equals('Bonjour Bob'));

    final entry = obs.logRing!.drain().singleWhere(
          (log) => log.message == 'Greeted Bob in French (fr)',
        );
    final record = holons.toProtoLogRecord(entry);
    final attrs = _attrs(record.attributes);
    expect(record.body.stringValue, equals('Greeted Bob in French (fr)'));
    expect(record.severityNumber,
        equals(obspb.SeverityNumber.SEVERITY_NUMBER_INFO));
    expect(attrs[holons.attrHolonsSlug]?.stringValue,
        equals('gabriel-greeting-dart-test'));
    expect(attrs[holons.attrServiceName]?.stringValue,
        equals('gabriel-greeting-dart-test'));
    expect(attrs[holons.attrHolonsInstanceUid]?.stringValue,
        equals('greeting-test-uid'));
    expect(attrs[holons.attrServiceInstanceId]?.stringValue,
        equals('greeting-test-uid'));
    expect(attrs[holons.attrHolonsSessionId]?.stringValue, equals(''));
    expect(
      attrs.keys.where(
          (key) => !key.startsWith('holons.') && !key.startsWith('service.')),
      unorderedEquals(<String>[
        'lang_code',
        'language',
        'name',
        'greeting',
        'transport',
        'duration_ns',
        holons.attrLoggerName,
        holons.attrCodeCaller,
      ]),
    );
    expect(attrs['lang_code']?.stringValue, equals('fr'));
    expect(attrs['language']?.stringValue, equals('French'));
    expect(attrs['name']?.stringValue, equals('Bob'));
    expect(attrs['greeting']?.stringValue, equals('Bonjour Bob'));
    expect(attrs['transport']?.stringValue, equals('stdio'));
    expect(attrs['duration_ns']?.whichValue(),
        equals(obspb.AnyValue_Value.intValue));
    expect(attrs['duration_ns']!.intValue.toInt(), greaterThanOrEqualTo(0));

    final counter = obs.registry!.listCounters().singleWhere(
          (metric) => metric.name == 'greeting_emitted_total',
        );
    expect(counter.value(), equals(1));
    expect(
      counter.labels,
      equals(<String, String>{
        'lang_code': 'fr',
        'language': 'French',
        'transport': 'stdio',
      }),
    );
  });
}

Map<String, obspb.AnyValue> _attrs(Iterable<obspb.KeyValue> attrs) => {
      for (final attr in attrs) attr.key: attr.value,
    };
