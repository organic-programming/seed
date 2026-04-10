import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons_app/holons_app.dart';

import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/gen/v1/greeting.pb.dart';
import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';
import 'package:gabriel_greeting_app_flutter/src/rpc/greeting_app_service.dart';
import 'package:gabriel_greeting_app_flutter/src/runtime/describe_registration.dart';
import 'package:gabriel_greeting_app_flutter/src/runtime/greeting_holon_connection.dart';

class FakeHolonCatalog implements Holons<GabrielHolonIdentity> {
  FakeHolonCatalog(this.holons);

  final List<GabrielHolonIdentity> holons;

  @override
  Future<List<GabrielHolonIdentity>> list() async => holons;

  @override
  Future<List<GabrielHolonIdentity>> discover() async => holons;
}

class FakeHolonConnector implements GreetingHolonConnectionFactory {
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
    this.tellHandler,
    this.closeFuture,
  });

  final List<Language> languages;
  final String Function({required String name, required String langCode})
  greetingBuilder;
  final Object? listLanguagesError;
  final Object? sayHelloError;
  final FutureOr<Object?> Function(String method, Object? payload)? tellHandler;
  final Future<void>? closeFuture;
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
  Future<Object?> tell({required String method, Object? payload}) async {
    if (tellHandler != null) {
      return await tellHandler!(method, payload);
    }

    final canonical = method.startsWith('/') ? method.substring(1) : method;
    switch (canonical) {
      case 'greeting.v1.GreetingService/ListLanguages':
        return <String, Object?>{
          'languages': languages
              .map(
                (language) => <String, Object?>{
                  'code': language.code,
                  'name': language.name,
                  'native': language.native,
                },
              )
              .toList(growable: false),
        };
      case 'greeting.v1.GreetingService/SayHello':
        final json = payload is Map<String, Object?>
            ? payload
            : (payload as Map).cast<String, Object?>();
        final greeting = await sayHello(
          name: (json['name'] ?? '') as String,
          langCode: (json['langCode'] ?? json['lang_code'] ?? '') as String,
        );
        return <String, Object?>{'greeting': greeting};
      default:
        throw UnsupportedError('No fake Tell handler registered for $method');
    }
  }

  @override
  Future<void> close() async {
    closed = true;
    await closeFuture;
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

CoaxManager buildCoaxManager({
  required GreetingController greetingController,
  SettingsStore? settingsStore,
  AppPlatformCapabilities? capabilities,
  CoaxSettingsDefaults? defaults,
}) {
  final coaxDefaults =
      defaults ??
      CoaxSettingsDefaults.standard(socketName: 'gabriel-greeting-coax.sock');
  late final CoaxManager coaxManager;
  coaxManager = CoaxManager(
    settingsStore: settingsStore ?? MemorySettingsStore(),
    defaults: coaxDefaults,
    capabilities: capabilities,
    prepareDescribe: () async {
      ensureAppDescribeRegistered();
    },
    serviceFactory: () => [
      CoaxRpcService(
        holonManager: greetingController,
        coaxManager: coaxManager,
      ),
      GreetingAppRpcService(greetingController),
    ],
  );
  return coaxManager;
}

@Deprecated('Use buildCoaxManager')
CoaxController buildCoaxController({
  required GreetingController greetingController,
  SettingsStore? settingsStore,
  AppPlatformCapabilities? capabilities,
  CoaxSettingsDefaults? defaults,
}) {
  return buildCoaxManager(
    greetingController: greetingController,
    settingsStore: settingsStore,
    capabilities: capabilities,
    defaults: defaults,
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
