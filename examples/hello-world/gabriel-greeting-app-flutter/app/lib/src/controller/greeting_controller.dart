import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:holons/gen/holons/v1/coax.pb.dart';
import 'package:holons/gen/holons/v1/manifest.pb.dart';
import 'package:holons/holons.dart' as holons_obs;
import 'package:holons_app/holons_app.dart';

import '../gen/v1/greeting.pb.dart';
import '../model/app_model.dart';
import '../runtime/greeting_holon_connection.dart';

class GreetingController extends ChangeNotifier implements HolonManager {
  static const listLanguagesMethod =
      'greeting.v1.GreetingService/ListLanguages';
  static const sayHelloMethod = 'greeting.v1.GreetingService/SayHello';

  GreetingController({
    Holons<GabrielHolonIdentity>? holons,
    @Deprecated('Use holons') HolonCatalog<GabrielHolonIdentity>? catalog,
    required GreetingHolonConnectionFactory connector,
    AppPlatformCapabilities? capabilities,
    String? initialTransport,
  }) : assert(holons != null || catalog != null),
       _holons = holons ?? catalog!,
       _connector = connector,
       capabilities = capabilities ?? AppPlatformCapabilities.desktopCurrent(),
       transport = HolonTransportName.normalize(
         initialTransport ?? Platform.environment['OP_ASSEMBLY_TRANSPORT'],
       ).rawValue;

  final Holons<GabrielHolonIdentity> _holons;
  final GreetingHolonConnectionFactory _connector;
  final AppPlatformCapabilities capabilities;
  holons_obs.Logger? _logger;
  holons_obs.Counter? _sayHelloTotal;
  holons_obs.Histogram? _sayHelloDuration;

  GreetingHolonConnection? _connection;
  Future<void>? _startFuture;
  bool _initialized = false;
  bool _disposed = false;
  int _connectionGeneration = 0;
  int _loadGeneration = 0;
  int _greetGeneration = 0;

  bool isRunning = false;
  bool isLoading = true;
  bool isGreeting = false;
  String? connectionError;
  String? error;
  String greeting = '';
  String userName = '';
  String selectedLanguageCode = '';
  String transport;
  List<Language> availableLanguages = const <Language>[];
  List<GabrielHolonIdentity> availableHolons = const <GabrielHolonIdentity>[];
  GabrielHolonIdentity? selectedHolon;

  Holons<GabrielHolonIdentity> get holons => _holons;

  void attachObservability(holons_obs.Observability observability) {
    _logger = observability.logger('greeting-controller');
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

  Future<void> refreshHolons() async {
    final previousSelection = selectedHolon?.slug;
    try {
      final discovered = await _holons.list();
      availableHolons = discovered;
      if (availableHolons.isEmpty) {
        selectedHolon = null;
      } else {
        selectedHolon = availableHolons.firstWhere(
          (item) => item.slug == previousSelection,
          orElse: () =>
              preferredHolon(availableHolons) ?? availableHolons.first,
        );
      }
      connectionError = null;
    } on Object catch (error) {
      availableHolons = const <GabrielHolonIdentity>[];
      selectedHolon = null;
      connectionError = 'Failed to discover Gabriel holons: $error';
    }
    _safeNotify();
  }

  Future<void> selectHolonBySlug(String slug, {bool reload = true}) async {
    final identity = availableHolons.firstWhere(
      (item) => item.slug == slug,
      orElse: () => throw StateError("Holon '$slug' not found"),
    );
    if (selectedHolon == identity) {
      if (reload) {
        await loadLanguages();
      }
      return;
    }
    selectedHolon = identity;
    await stop();
    _safeNotify();
    if (reload) {
      await loadLanguages();
    }
  }

  Future<void> setTransport(String value, {bool reload = true}) async {
    final normalized = HolonTransportName.normalize(value);
    if (!capabilities.holonTransportNames.contains(normalized)) {
      throw StateError(
        'Transport "${normalized.rawValue}" is not available on this platform',
      );
    }
    if (normalized.rawValue == transport) {
      return;
    }
    transport = normalized.rawValue;
    await stop();
    _safeNotify();
    if (reload) {
      await loadLanguages();
    }
  }

  Future<void> setSelectedLanguage(String code, {bool greetNow = true}) async {
    if (code == selectedLanguageCode) {
      return;
    }
    selectedLanguageCode = code;
    _safeNotify();
    if (greetNow) {
      await greet();
    }
  }

  Future<void> setUserName(String value, {bool greetNow = true}) async {
    if (value == userName) {
      return;
    }
    userName = value;
    _safeNotify();
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
    _safeNotify();

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
        if (!isRunning || _connection == null) {
          throw StateError(connectionError ?? 'Holon did not become ready');
        }
        final languages = await _connection!.listLanguages();
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
        _safeNotify();
        if (greetAfterLoad && selectedLanguageCode.isNotEmpty) {
          unawaited(greet());
        }
        return;
      } on Object catch (loadError) {
        await _dropConnection();
        if (index == retryDelays.length - 1 && _loadGeneration == generation) {
          error =
              'Failed to load languages: ${connectionError ?? loadError.toString()}';
          isLoading = false;
          _safeNotify();
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
    _safeNotify();
    final startedAt = DateTime.now();
    _logger?.info('Greeting request started', {
      'method': sayHelloMethod,
      'lang': resolvedCode,
      'holon': selectedHolon?.slug ?? '',
    });

    try {
      await ensureStarted();
      final response = await _connection!.sayHello(
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
      _logger?.info('Greeting response received', {
        'method': sayHelloMethod,
        'lang': resolvedCode,
        'holon': selectedHolon?.slug ?? '',
      });
    } on Object catch (greetError) {
      if (_greetGeneration != requestGeneration) {
        return;
      }
      error = 'Greeting failed: $greetError';
      _logger?.error('Greeting request failed', {
        'method': sayHelloMethod,
        'lang': resolvedCode,
        'holon': selectedHolon?.slug ?? '',
        'error': greetError.toString(),
      });
    } finally {
      if (_greetGeneration == requestGeneration) {
        isGreeting = false;
        _safeNotify();
      }
    }
  }

  @override
  Future<List<MemberInfo>> listMembers() async {
    return availableHolons.map(_memberForIdentity).toList(growable: false);
  }

  @override
  Future<MemberInfo?> memberStatus(String slug) async {
    for (final identity in availableHolons) {
      if (identity.slug == slug) {
        return _memberForIdentity(identity);
      }
    }
    return null;
  }

  @override
  Future<MemberInfo> connectMember(String slug, {String transport = ''}) async {
    final identity = availableHolons.firstWhere(
      (item) => item.slug == slug,
      orElse: () => throw StateError("Member '$slug' not found"),
    );
    if (transport.trim().isNotEmpty) {
      await setTransport(transport, reload: false);
    }
    await selectHolonBySlug(identity.slug, reload: false);
    await loadLanguages(greetAfterLoad: false);
    return _memberForIdentity(
      identity,
      overrideState: isRunning && error == null
          ? MemberState.MEMBER_STATE_CONNECTED
          : MemberState.MEMBER_STATE_ERROR,
    );
  }

  @override
  Future<void> disconnectMember(String slug) async {
    if (slug.trim().isNotEmpty && selectedHolon?.slug != slug) {
      return;
    }
    await stop();
  }

  @override
  Future<Object?> tellMember({
    required String slug,
    required String method,
    Object? payloadJson,
  }) async {
    final canonicalMethod = _canonicalMethod(method);
    final decodedPayload = payloadJson ?? const <String, Object?>{};

    if (selectedHolon?.slug != slug) {
      await selectHolonBySlug(slug, reload: false);
    }

    switch (canonicalMethod) {
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
    final connection = _connection;
    if (connection == null) {
      throw StateError(connectionError ?? 'Holon did not become ready');
    }
    return connection.tell(method: canonicalMethod, payload: decodedPayload);
  }

  Future<void> ensureStarted() async {
    if (_connection != null) {
      return;
    }
    final pendingStart = _startFuture;
    if (pendingStart != null) {
      await pendingStart;
      if (_connection != null) {
        return;
      }
    }

    if (availableHolons.isEmpty) {
      await refreshHolons();
    }

    final holon = selectedHolon ?? preferredHolon(availableHolons);
    if (holon == null) {
      connectionError = 'No Gabriel holons found';
      isRunning = false;
      _safeNotify();
      throw StateError(connectionError!);
    }

    selectedHolon = holon;
    connectionError = null;
    final generation = ++_connectionGeneration;
    final future = _connect(generation, holon);
    _startFuture = future;

    try {
      await future;
    } finally {
      if (identical(_startFuture, future)) {
        _startFuture = null;
      }
    }
  }

  Future<void> _connect(int generation, GabrielHolonIdentity holon) async {
    final effectiveTransport = effectiveHolonTransport(
      requestedTransport: transport,
      buildRunner: holon.buildRunner,
    );
    final retryDelays = effectiveTransport == 'stdio'
        ? const <Duration>[Duration.zero]
        : const <Duration>[
            Duration.zero,
            Duration(milliseconds: 150),
            Duration(milliseconds: 400),
            Duration(milliseconds: 800),
          ];
    Object? lastError;

    try {
      for (var index = 0; index < retryDelays.length; index += 1) {
        final delay = retryDelays[index];
        if (delay > Duration.zero) {
          await Future<void>.delayed(delay);
        }
        final transportLabel = effectiveTransport == transport
            ? transport
            : '$transport -> $effectiveTransport';
        _log(
          '[HostUI] assembly=Gabriel-Greeting-App-Flutter holon=${holon.binaryName} transport=$transportLabel',
        );
        try {
          final connection = await _connector.connect(
            holon,
            transport: transport,
          );
          if (generation != _connectionGeneration || _disposed) {
            await connection.close();
            return;
          }
          _connection = connection;
          isRunning = true;
          connectionError = null;
          _log('[HostUI] connected to ${holon.binaryName} on $transportLabel');
          _safeNotify();
          return;
        } on Object catch (error) {
          lastError = error;
          if (index < retryDelays.length - 1) {
            _log(
              '[HostUI] retrying ${holon.binaryName} on $transportLabel after connect failure: $error',
            );
          }
        }
      }
      throw lastError ?? StateError('Holon connection failed');
    } on Object catch (error) {
      if (generation != _connectionGeneration || _disposed) {
        return;
      }
      _connection = null;
      isRunning = false;
      connectionError = 'Failed to start Gabriel holon: $error';
      _safeNotify();
      rethrow;
    }
  }

  Future<void> stop() async {
    _connectionGeneration += 1;
    _startFuture = null;
    try {
      await _dropConnection();
    } on Object catch (error) {
      connectionError = 'Failed to stop Gabriel holon connection: $error';
    }
    _safeNotify();
  }

  Future<void> _dropConnection() async {
    final currentConnection = _connection;
    _connection = null;
    isRunning = false;
    if (currentConnection == null) {
      return;
    }
    await currentConnection.close();
  }

  Future<void> shutdown() async {
    _disposed = true;
    await stop();
  }

  void _safeNotify() {
    if (!_disposed) {
      notifyListeners();
    }
  }

  static String _canonicalMethod(String method) {
    final trimmed = method.trim();
    if (trimmed.isEmpty) {
      throw ArgumentError.value(method, 'method', 'Method must not be empty');
    }
    return trimmed.startsWith('/') ? trimmed.substring(1) : trimmed;
  }

  MemberInfo _memberForIdentity(
    GabrielHolonIdentity identity, {
    MemberState? overrideState,
  }) {
    return MemberInfo(
      slug: identity.slug,
      identity: HolonManifest_Identity(
        familyName: identity.familyName,
        givenName: identity.displayName,
      ),
      state: overrideState ?? _memberStateFor(identity),
      isOrganism: false,
    );
  }

  MemberState _memberStateFor(GabrielHolonIdentity identity) {
    if (selectedHolon?.slug == identity.slug && isRunning) {
      return MemberState.MEMBER_STATE_CONNECTED;
    }
    return MemberState.MEMBER_STATE_AVAILABLE;
  }

  void _log(String message) {
    stderr.writeln(message);
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }
}

extension on String {
  String ifEmpty(String fallback) => trim().isEmpty ? fallback : this;
}
