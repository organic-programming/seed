import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';

import '../gen/v1/greeting.pb.dart';
import '../model/app_model.dart';
import '../runtime/holon_catalog.dart';
import '../runtime/holon_connector.dart';

class GreetingController extends ChangeNotifier {
  GreetingController({
    required HolonCatalog catalog,
    required HolonConnector connector,
    AppPlatformCapabilities? capabilities,
    String? initialTransport,
  }) : _catalog = catalog,
       _connector = connector,
       capabilities = capabilities ?? AppPlatformCapabilities.desktopCurrent(),
       transport = normalizedTransportSelection(
         initialTransport ?? Platform.environment['OP_ASSEMBLY_TRANSPORT'],
       );

  final HolonCatalog _catalog;
  final HolonConnector _connector;
  final AppPlatformCapabilities capabilities;

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
      final discovered = await _catalog.discover();
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
    final normalized = normalizedTransportSelection(value);
    if (!capabilities.appTransports.contains(normalized)) {
      throw StateError(
        'Transport "$normalized" is not available on this platform',
      );
    }
    if (normalized == transport) {
      return;
    }
    transport = normalized;
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
        : effectiveHolonTransport(selectedHolon!, transport);
    isLoading = true;
    error = null;
    greeting = '';
    availableLanguages = const <Language>[];
    _safeNotify();

    final retryDelays = effectiveTransport == 'stdio'
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
    } on Object catch (greetError) {
      if (_greetGeneration != requestGeneration) {
        return;
      }
      error = 'Greeting failed: $greetError';
    } finally {
      if (_greetGeneration == requestGeneration) {
        isGreeting = false;
        _safeNotify();
      }
    }
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
    final effectiveTransport = effectiveHolonTransport(holon, transport);
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
            transport: effectiveTransport,
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
