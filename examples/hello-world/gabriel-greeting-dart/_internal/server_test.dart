import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:test/test.dart';

import '../gen/dart/greeting/v1/greeting.pb.dart';
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
}
