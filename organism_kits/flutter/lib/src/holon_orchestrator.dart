import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:holons/gen/holons/v1/coax.pb.dart';
import 'package:holons/holons.dart'
    show AppPlatformCapabilities, HolonTransportName, Holons;

import 'coax_service.dart';
import 'holon_connector.dart';

typedef HolonConnectionFactory<T, C> =
    Future<C> Function(T holon, {required String transport});

typedef HolonConnectionCloser<C> = Future<void> Function(C connection);

typedef HolonMemberInfoBuilder<T> =
    MemberInfo Function(T holon, MemberState state);

abstract class HolonOrchestratorController<T, C extends Object>
    extends ChangeNotifier
    implements HolonManager, HolonSelectionController<T> {
  HolonOrchestratorController({
    required Holons<T> holons,
    required HolonConnectionFactory<T, C> connector,
    required HolonConnectionCloser<C> connectionCloser,
    required String Function(T holon) slugOf,
    required HolonMemberInfoBuilder<T> memberInfoBuilder,
    String Function(T holon)? buildRunnerOf,
    T? Function(Iterable<T> holons)? preferredHolon,
    AppPlatformCapabilities? capabilities,
    String? initialTransport,
    String? environmentTransport,
    this.discoveryErrorPrefix = 'Failed to discover holons',
    this.noHolonsMessage = 'No holons found',
    this.connectionErrorPrefix = 'Failed to start holon',
    this.stopErrorPrefix = 'Failed to stop holon connection',
  }) : holons = holons,
       _connector = connector,
       _connectionCloser = connectionCloser,
       _slugOf = slugOf,
       _buildRunnerOf = buildRunnerOf,
       _memberInfoBuilder = memberInfoBuilder,
       _preferredHolon = preferredHolon,
       capabilities = capabilities ?? AppPlatformCapabilities.desktopCurrent(),
       transport = HolonTransportName.normalize(
         initialTransport ??
             environmentTransport ??
             Platform.environment['OP_ASSEMBLY_TRANSPORT'],
       ).rawValue;

  final Holons<T> holons;
  final HolonConnectionFactory<T, C> _connector;
  final HolonConnectionCloser<C> _connectionCloser;
  final String Function(T holon) _slugOf;
  final String Function(T holon)? _buildRunnerOf;
  final HolonMemberInfoBuilder<T> _memberInfoBuilder;
  final T? Function(Iterable<T> holons)? _preferredHolon;

  @override
  final AppPlatformCapabilities capabilities;
  final String discoveryErrorPrefix;
  final String noHolonsMessage;
  final String connectionErrorPrefix;
  final String stopErrorPrefix;

  C? _connection;
  Future<void>? _startFuture;
  bool _disposed = false;
  int _connectionGeneration = 0;

  bool isRunning = false;

  @override
  String? connectionError;

  @override
  String? error;

  String transport;

  @override
  List<T> availableHolons = List<T>.empty(growable: false);

  @override
  T? selectedHolon;

  @protected
  C? get activeConnection => _connection;

  @override
  String slugOf(T holon) => _slugOf(holon);

  @protected
  String? buildRunnerOf(T holon) => _buildRunnerOf?.call(holon);

  Future<void> initialize() async {
    await refreshHolons();
    await reloadSelectedHolon();
  }

  Future<void> refreshHolons() async {
    final selected = selectedHolon;
    final previousSelection = selected == null ? null : _slugOf(selected);
    try {
      final discovered = await holons.list();
      availableHolons = discovered;
      if (availableHolons.isEmpty) {
        selectedHolon = null;
      } else {
        selectedHolon = availableHolons.firstWhere(
          (item) => _slugOf(item) == previousSelection,
          orElse: () =>
              _preferredHolon?.call(availableHolons) ?? availableHolons.first,
        );
      }
      connectionError = null;
    } on Object catch (error) {
      availableHolons = List<T>.empty(growable: false);
      selectedHolon = null;
      connectionError = '$discoveryErrorPrefix: $error';
    }
    safeNotify();
  }

  @override
  Future<void> selectHolonBySlug(String slug, {bool reload = true}) async {
    final identity = availableHolons.firstWhere(
      (item) => _slugOf(item) == slug,
      orElse: () => throw StateError("Holon '$slug' not found"),
    );
    if (selectedHolon == identity) {
      if (reload) {
        await reloadSelectedHolon();
      }
      return;
    }
    selectedHolon = identity;
    await stop();
    safeNotify();
    if (reload) {
      await reloadSelectedHolon();
    }
  }

  @override
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
    safeNotify();
    if (reload) {
      await reloadSelectedHolon();
    }
  }

  @override
  Future<List<MemberInfo>> listMembers() async {
    return availableHolons.map(_memberForIdentity).toList(growable: false);
  }

  @override
  Future<MemberInfo?> memberStatus(String slug) async {
    for (final identity in availableHolons) {
      if (_slugOf(identity) == slug) {
        return _memberForIdentity(identity);
      }
    }
    return null;
  }

  @override
  Future<MemberInfo> connectMember(String slug, {String transport = ''}) async {
    final identity = availableHolons.firstWhere(
      (item) => _slugOf(item) == slug,
      orElse: () => throw StateError("Member '$slug' not found"),
    );
    if (transport.trim().isNotEmpty) {
      await setTransport(transport, reload: false);
    }
    await selectHolonBySlug(_slugOf(identity), reload: false);
    await reloadSelectedHolon();
    return _memberForIdentity(
      identity,
      overrideState: isRunning && error == null
          ? MemberState.MEMBER_STATE_CONNECTED
          : MemberState.MEMBER_STATE_ERROR,
    );
  }

  @override
  Future<void> disconnectMember(String slug) async {
    final selected = selectedHolon;
    if (slug.trim().isNotEmpty &&
        (selected == null || _slugOf(selected) != slug)) {
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
    final canonicalMethod = canonicalMemberMethod(method);
    final decodedPayload = payloadJson ?? const <String, Object?>{};

    final selected = selectedHolon;
    if (selected == null || _slugOf(selected) != slug) {
      await selectHolonBySlug(slug, reload: false);
    }

    return handleMemberTell(
      slug: slug,
      method: canonicalMethod,
      payloadJson: decodedPayload,
    );
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

    final holon = selectedHolon ?? _preferredHolon?.call(availableHolons);
    if (holon == null) {
      connectionError = noHolonsMessage;
      isRunning = false;
      safeNotify();
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

  Future<void> stop() async {
    _connectionGeneration += 1;
    _startFuture = null;
    try {
      await dropConnection();
    } on Object catch (error) {
      connectionError = '$stopErrorPrefix: $error';
    }
    safeNotify();
  }

  @protected
  Future<void> dropConnection() async {
    final currentConnection = _connection;
    _connection = null;
    isRunning = false;
    if (currentConnection == null) {
      return;
    }
    await _connectionCloser(currentConnection);
  }

  Future<void> shutdown() async {
    _disposed = true;
    await stop();
  }

  @protected
  Future<void> reloadSelectedHolon();

  @protected
  Future<Object?> handleMemberTell({
    required String slug,
    required String method,
    Object? payloadJson,
  });

  @protected
  List<Duration> connectionRetryDelays(String effectiveTransport) {
    return effectiveTransport == HolonTransportName.stdio.rawValue
        ? const <Duration>[Duration.zero]
        : const <Duration>[
            Duration.zero,
            Duration(milliseconds: 150),
            Duration(milliseconds: 400),
            Duration(milliseconds: 800),
          ];
  }

  @protected
  void didConnectHolon(T holon, String transportLabel) {}

  @protected
  void didFailHolonConnection(
    T holon,
    String effectiveTransport,
    Object error,
  ) {}

  @protected
  void safeNotify() {
    if (!_disposed) {
      notifyListeners();
    }
  }

  Future<void> _connect(int generation, T holon) async {
    final effectiveTransport = effectiveHolonTransport(
      requestedTransport: transport,
      buildRunner: _buildRunnerOf?.call(holon),
    );
    Object? lastError;

    try {
      for (final delay in connectionRetryDelays(effectiveTransport)) {
        if (delay > Duration.zero) {
          await Future<void>.delayed(delay);
        }
        final transportLabel = effectiveTransport == transport
            ? transport
            : '$transport -> $effectiveTransport';
        try {
          final connection = await _connector(holon, transport: transport);
          if (generation != _connectionGeneration || _disposed) {
            await _connectionCloser(connection);
            return;
          }
          _connection = connection;
          isRunning = true;
          connectionError = null;
          didConnectHolon(holon, transportLabel);
          safeNotify();
          return;
        } on Object catch (error) {
          lastError = error;
        }
      }
      throw lastError ?? StateError('Holon connection failed');
    } on Object catch (error) {
      if (generation != _connectionGeneration || _disposed) {
        return;
      }
      _connection = null;
      isRunning = false;
      connectionError = '$connectionErrorPrefix: $error';
      didFailHolonConnection(holon, effectiveTransport, error);
      safeNotify();
      rethrow;
    }
  }

  MemberInfo _memberForIdentity(T identity, {MemberState? overrideState}) {
    return _memberInfoBuilder(
      identity,
      overrideState ?? _memberStateFor(identity),
    );
  }

  MemberState _memberStateFor(T identity) {
    final selected = selectedHolon;
    if (selected != null &&
        _slugOf(selected) == _slugOf(identity) &&
        isRunning) {
      return MemberState.MEMBER_STATE_CONNECTED;
    }
    return MemberState.MEMBER_STATE_AVAILABLE;
  }

  static String canonicalMemberMethod(String method) {
    final trimmed = method.trim();
    if (trimmed.isEmpty) {
      throw ArgumentError.value(method, 'method', 'Method must not be empty');
    }
    return trimmed.startsWith('/') ? trimmed.substring(1) : trimmed;
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }
}
