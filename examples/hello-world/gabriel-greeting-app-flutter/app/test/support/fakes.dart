import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';

import 'package:gabriel_greeting_app_flutter/src/gen/v1/greeting.pb.dart';
import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';
import 'package:gabriel_greeting_app_flutter/src/runtime/holon_catalog.dart';
import 'package:gabriel_greeting_app_flutter/src/runtime/holon_connector.dart';

class FakeHolonCatalog implements HolonCatalog {
  FakeHolonCatalog(this.holons);

  final List<GabrielHolonIdentity> holons;

  @override
  Future<List<GabrielHolonIdentity>> discover() async => holons;
}

class FakeHolonConnector implements HolonConnector {
  final List<(String slug, String transport)> connectCalls =
      <(String, String)>[];
  final Map<String, FakeGreetingHolonConnection Function(String transport)>
  factories;

  FakeHolonConnector({required this.factories});

  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    connectCalls.add((holon.slug, transport));
    final factory = factories[holon.slug];
    if (factory == null) {
      throw StateError('No fake connector registered for ${holon.slug}');
    }
    return factory(transport);
  }
}

class FakeGreetingHolonConnection implements GreetingHolonConnection {
  FakeGreetingHolonConnection({
    required this.languages,
    required this.greetingBuilder,
    this.listLanguagesError,
    this.sayHelloError,
  });

  final List<Language> languages;
  final String Function({required String name, required String langCode})
  greetingBuilder;
  final Object? listLanguagesError;
  final Object? sayHelloError;
  final List<(String name, String langCode)> sayHelloCalls =
      <(String, String)>[];
  bool closed = false;

  @override
  Future<List<Language>> listLanguages() async {
    if (listLanguagesError != null) {
      throw listLanguagesError!;
    }
    return languages;
  }

  @override
  Future<String> sayHello({
    required String name,
    required String langCode,
  }) async {
    sayHelloCalls.add((name, langCode));
    if (sayHelloError != null) {
      throw sayHelloError!;
    }
    return greetingBuilder(name: name, langCode: langCode);
  }

  @override
  Future<void> close() async {
    closed = true;
  }
}

Language language({
  required String code,
  required String name,
  required String native,
}) {
  return Language(code: code, name: name, native: native);
}

GabrielHolonIdentity holon(String slug, {int? rank}) {
  return GabrielHolonIdentity(
    slug: slug,
    familyName: GabrielHolonIdentity.displayNameFor(slug),
    binaryName: slug,
    buildRunner: 'dart',
    displayName: GabrielHolonIdentity.displayNameFor(slug),
    sortRank: rank ?? GabrielHolonIdentity.sortRankFor(slug),
    holonUuid: '$slug-uuid',
    born: '2026-04-04',
    sourceKind: 'source',
    discoveryPath: '/tmp/$slug.holon',
    hasSource: true,
  );
}

Future<int> reserveTcpPort() async {
  final socket = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
  final port = socket.port;
  await socket.close();
  return port;
}

ClientChannel clientChannelFromListenUri(String listenUri) {
  final uri = Uri.parse(listenUri);
  return ClientChannel(
    uri.host,
    port: uri.port,
    options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
  );
}

Future<void> waitForCoaxUpdate() {
  return Future<void>.delayed(const Duration(milliseconds: 250));
}
