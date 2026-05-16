import 'package:holons/gen/holons/v1/coax.pb.dart';
import 'package:holons/gen/holons/v1/manifest.pb.dart';
import 'package:holons/holons.dart' as holons_obs;
import 'package:holons_app/holons_app.dart';

import '../gen/v1/greeting.pb.dart';
import '../model/app_model.dart';
import '../runtime/greeting_holon_connection.dart';

class GreetingController
    extends
        HolonOrchestratorController<
          GabrielHolonIdentity,
          GreetingHolonConnection
        > {
  static const listLanguagesMethod =
      'greeting.v1.GreetingService/ListLanguages';
  static const sayHelloMethod = 'greeting.v1.GreetingService/SayHello';

  GreetingController({
    Holons<GabrielHolonIdentity>? holons,
    @Deprecated('Use holons') HolonCatalog<GabrielHolonIdentity>? catalog,
    required GreetingHolonConnectionFactory connector,
    super.capabilities,
    super.initialTransport,
  }) : assert(holons != null || catalog != null),
       super(
         holons: holons ?? catalog!,
         connector: connector.connect,
         connectionCloser: (connection) => connection.close(),
         slugOf: (holon) => holon.slug,
         buildRunnerOf: (holon) => holon.buildRunner,
         preferredHolon: preferredHolon,
         memberInfoBuilder: _gabrielMemberInfo,
         discoveryErrorPrefix: 'Failed to discover Gabriel holons',
         noHolonsMessage: 'No Gabriel holons found',
         connectionErrorPrefix: 'Failed to start Gabriel holon',
         stopErrorPrefix: 'Failed to stop Gabriel holon connection',
       );

  holons_obs.Logger? _logger;
  holons_obs.Logger? _responseLogger;
  holons_obs.Counter? _sayHelloTotal;
  holons_obs.Histogram? _sayHelloDuration;

  bool _initialized = false;
  int _loadGeneration = 0;
  int _greetGeneration = 0;

  bool isLoading = true;
  bool isGreeting = false;
  String greeting = '';
  String userName = '';
  String selectedLanguageCode = '';
  List<Language> availableLanguages = const <Language>[];

  void attachObservability(holons_obs.Observability observability) {
    _logger = observability.logger('greeting-controller');
    _responseLogger = observability.logger('');
    _sayHelloTotal = observability.counter(
      'gabriel_greeting_say_hello_total',
      help: 'Greeting requests sent from the Flutter app',
      labels: const {'origin': 'app'},
    );
    _sayHelloDuration = observability.histogram(
      'gabriel_greeting_say_hello_duration_seconds',
      help: 'Greeting request duration observed by the Flutter app',
      labels: const {'origin': 'app'},
    );
  }

  String get statusTitle {
    if (isLoading) {
      return 'Starting holon...';
    }
    if (isRunning) {
      return 'Ready';
    }
    return 'Offline';
  }

  @override
  Future<void> initialize() async {
    if (_initialized) {
      return;
    }
    _initialized = true;
    if (userName.isEmpty) {
      userName = 'World';
    }
    notifyListeners();
    await refreshHolons();
    await loadLanguages(greetAfterLoad: false);
    if (selectedLanguageCode.isNotEmpty) {
      await greet();
    }
  }

  Future<void> setSelectedLanguage(String code, {bool greetNow = true}) async {
    if (code == selectedLanguageCode) {
      return;
    }
    selectedLanguageCode = code;
    safeNotify();
    if (greetNow) {
      await greet();
    }
  }

  Future<void> setUserName(String value, {bool greetNow = true}) async {
    if (value == userName) {
      return;
    }
    userName = value;
    safeNotify();
    if (greetNow && selectedLanguageCode.isNotEmpty) {
      await greet();
    }
  }

  Future<void> loadLanguages({bool greetAfterLoad = true}) async {
    final generation = ++_loadGeneration;
    final effectiveTransport = selectedHolon == null
        ? transport
        : effectiveHolonTransport(
            requestedTransport: transport,
            buildRunner: selectedHolon!.buildRunner,
          );
    isLoading = true;
    error = null;
    greeting = '';
    availableLanguages = const <Language>[];
    safeNotify();

    final retryDelays = effectiveTransport == HolonTransportName.stdio.rawValue
        ? const <Duration>[Duration.zero, Duration(milliseconds: 400)]
        : const <Duration>[
            Duration.zero,
            Duration(milliseconds: 200),
            Duration(milliseconds: 800),
          ];

    for (var index = 0; index < retryDelays.length; index += 1) {
      try {
        final delay = retryDelays[index];
        if (delay > Duration.zero) {
          await Future<void>.delayed(delay);
        }
        await ensureStarted();
        final connection = activeConnection;
        if (!isRunning || connection == null) {
          throw StateError(connectionError ?? 'Holon did not become ready');
        }
        final languages = await connection.listLanguages();
        if (_loadGeneration != generation) {
          return;
        }
        availableLanguages = languages;
        final preferredCode = selectedLanguageCode;
        selectedLanguageCode = availableLanguages
            .firstWhere(
              (language) => language.code == preferredCode,
              orElse: () => Language(),
            )
            .code
            .ifEmpty(
              availableLanguages
                  .firstWhere(
                    (language) => language.code == 'en',
                    orElse: () => Language(),
                  )
                  .code
                  .ifEmpty(
                    availableLanguages.isEmpty
                        ? ''
                        : availableLanguages.first.code,
                  ),
            );
        error = null;
        isLoading = false;
        safeNotify();
        if (greetAfterLoad && selectedLanguageCode.isNotEmpty) {
          await greet();
        }
        return;
      } on Object catch (loadError) {
        await dropConnection();
        if (index == retryDelays.length - 1 && _loadGeneration == generation) {
          error =
              'Failed to load languages: ${connectionError ?? loadError.toString()}';
          isLoading = false;
          safeNotify();
        }
      }
    }
  }

  Future<void> greet({String? name, String? langCode}) async {
    final resolvedCode = langCode ?? selectedLanguageCode;
    if (resolvedCode.trim().isEmpty) {
      return;
    }

    final requestGeneration = ++_greetGeneration;
    isGreeting = true;
    safeNotify();
    final startedAt = DateTime.now();
    try {
      await ensureStarted();
      final connection = activeConnection;
      if (connection == null) {
        throw StateError(connectionError ?? 'Holon did not become ready');
      }
      final response = await connection.sayHello(
        name: name ?? userName,
        langCode: resolvedCode,
      );
      if (_greetGeneration != requestGeneration) {
        return;
      }
      greeting = response;
      error = null;
      _sayHelloTotal?.inc();
      _sayHelloDuration?.observe(
        DateTime.now().difference(startedAt).inMicroseconds /
            Duration.microsecondsPerSecond,
      );
      (_responseLogger ?? _logger)?.info(
        'Greeting response received',
        fields: {
          'method': sayHelloMethod,
          'lang': resolvedCode,
          'holon': selectedHolon?.slug ?? '',
        },
      );
    } on Object catch (greetError) {
      if (_greetGeneration != requestGeneration) {
        return;
      }
      error = 'Greeting failed: $greetError';
      _logger?.error(
        'Greeting request failed',
        fields: {
          'method': sayHelloMethod,
          'lang': resolvedCode,
          'holon': selectedHolon?.slug ?? '',
          'error': greetError.toString(),
        },
      );
    } finally {
      if (_greetGeneration == requestGeneration) {
        isGreeting = false;
        safeNotify();
      }
    }
  }

  @override
  Future<void> reloadSelectedHolon() {
    return loadLanguages();
  }

  @override
  Future<Object?> handleMemberTell({
    required String slug,
    required String method,
    Object? payloadJson,
  }) async {
    final decodedPayload = payloadJson ?? const <String, Object?>{};

    switch (method) {
      case listLanguagesMethod:
        await loadLanguages(greetAfterLoad: false);
        return ListLanguagesResponse(
          languages: availableLanguages,
        ).toProto3Json();

      case sayHelloMethod:
        final request = SayHelloRequest()..mergeFromProto3Json(decodedPayload);
        if (request.name.isNotEmpty) {
          await setUserName(request.name, greetNow: false);
        }
        if (request.langCode.isNotEmpty) {
          await setSelectedLanguage(request.langCode, greetNow: false);
        }
        await greet(
          name: request.name.isEmpty ? null : request.name,
          langCode: request.langCode.isEmpty ? null : request.langCode,
        );
        return SayHelloResponse(greeting: greeting).toProto3Json();
    }

    await ensureStarted();
    final connection = activeConnection;
    if (connection == null) {
      throw StateError(connectionError ?? 'Holon did not become ready');
    }
    return connection.tell(method: method, payload: decodedPayload);
  }

  @override
  void didConnectHolon(GabrielHolonIdentity holon, String transportLabel) {
    _logger?.info(
      'Holon connection ready',
      fields: {'holon': holon.slug, 'transport': transportLabel},
    );
  }

  @override
  void didFailHolonConnection(
    GabrielHolonIdentity holon,
    String effectiveTransport,
    Object error,
  ) {
    _logger?.error(
      'Holon connection failed',
      fields: {
        'holon': holon.slug,
        'transport': effectiveTransport,
        'error': error.toString(),
      },
    );
  }
}

MemberInfo _gabrielMemberInfo(
  GabrielHolonIdentity identity,
  MemberState state,
) {
  return MemberInfo(
    slug: identity.slug,
    identity: HolonManifest_Identity(
      familyName: identity.familyName,
      givenName: identity.displayName,
    ),
    state: state,
    isOrganism: false,
  );
}

extension on String {
  String ifEmpty(String fallback) => trim().isEmpty ? fallback : this;
}
