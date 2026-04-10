import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

import 'coax_configuration.dart';
import 'platform_capabilities.dart';
import 'settings_store.dart';

class CoaxManager extends ChangeNotifier {
  CoaxManager({
    required SettingsStore settingsStore,
    required List<Service> Function() serviceFactory,
    Future<void> Function()? prepareDescribe,
    AppPlatformCapabilities? capabilities,
    CoaxSettingsDefaults? defaults,
    this.enabledKey = 'coax.server.enabled',
    this.settingsKey = 'coax.server.settings',
  }) : _settingsStore = settingsStore,
       _serviceFactory = serviceFactory,
       _prepareDescribe = prepareDescribe,
       capabilities = capabilities ?? AppPlatformCapabilities.desktopCurrent(),
       defaults = defaults ?? CoaxSettingsDefaults.standard(),
       _isEnabled = settingsStore.readBool(enabledKey),
       _snapshot = _loadSnapshot(
         settingsStore,
         settingsKey,
         defaults ?? CoaxSettingsDefaults.standard(),
       ) {
    if (!this.capabilities.supportsUnixSockets &&
        _snapshot.serverTransport == CoaxServerTransport.unix) {
      _snapshot = this.defaults.snapshot();
    }
  }

  final SettingsStore _settingsStore;
  final List<Service> Function() _serviceFactory;
  final Future<void> Function()? _prepareDescribe;
  final AppPlatformCapabilities capabilities;
  final CoaxSettingsDefaults defaults;
  final String enabledKey;
  final String settingsKey;

  holons.RunningServer? _runningServer;
  bool _disposed = false;
  bool _isEnabled;
  CoaxSettingsSnapshot _snapshot;
  String? listenUri;
  String? statusDetail;
  int _startGeneration = 0;

  bool get isEnabled => _isEnabled;
  CoaxServerTransport get serverTransport => _snapshot.serverTransport;
  String get serverHost => _snapshot.serverHost;
  String get serverPortText => _snapshot.serverPortText;
  String get serverUnixPath => _snapshot.serverUnixPath;
  String get defaultUnixPath => defaults.serverUnixPath;

  int get serverPort =>
      sanitizedPort(serverPortText, serverTransport.defaultPort);

  String? get serverPortValidationMessage {
    if (serverTransport != CoaxServerTransport.tcp) {
      return null;
    }
    final trimmed = serverPortText.trim();
    if (trimmed.isEmpty) {
      return 'Empty port. Falling back to ${serverTransport.defaultPort}.';
    }
    final parsed = int.tryParse(trimmed);
    if (parsed == null || parsed < 1 || parsed > 65535) {
      return 'Invalid port. Falling back to ${serverTransport.defaultPort}.';
    }
    return null;
  }

  String get serverPreviewEndpoint {
    switch (serverTransport) {
      case CoaxServerTransport.tcp:
        final host = serverHost.trim().isEmpty
            ? defaults.serverHost
            : serverHost.trim();
        return 'tcp://$host:$serverPort';
      case CoaxServerTransport.unix:
        final path = serverUnixPath.trim().isEmpty
            ? defaults.serverUnixPath
            : serverUnixPath.trim();
        return 'unix://$path';
    }
  }

  CoaxSurfaceStatus get serverStatus {
    return CoaxSurfaceStatus(
      id: 'server',
      title: 'Server',
      endpoint: isEnabled
          ? (listenUri ?? serverPreviewEndpoint)
          : serverPreviewEndpoint,
      state: _serverSurfaceState,
    );
  }

  CoaxSurfaceState get _serverSurfaceState {
    if (!isEnabled) {
      return CoaxSurfaceState.off;
    }
    if (statusDetail != null) {
      return CoaxSurfaceState.error;
    }
    if (listenUri != null) {
      return CoaxSurfaceState.live;
    }
    return CoaxSurfaceState.announced;
  }

  Future<void> startIfEnabled() async {
    if (!_isEnabled) {
      return;
    }
    await _reconfigureRuntime();
  }

  Future<void> setEnabled(bool value) async {
    if (_isEnabled == value) {
      return;
    }
    _isEnabled = value;
    await _settingsStore.writeBool(enabledKey, value);
    _safeNotify();
    await _reconfigureRuntime();
  }

  @Deprecated('Use setEnabled()')
  Future<void> setIsEnabled(bool value) => setEnabled(value);

  Future<void> setServerTransport(CoaxServerTransport value) async {
    if (value == serverTransport) {
      return;
    }
    final nextPortText = _usesDefaultServerPort(serverTransport)
        ? value.defaultPort.toString()
        : serverPortText;
    _snapshot = CoaxSettingsSnapshot(
      serverTransport: value,
      serverHost: serverHost,
      serverPortText: nextPortText,
      serverUnixPath: serverUnixPath,
    );
    await _persistSnapshot();
    _safeNotify();
    if (isEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> setServerHost(String value) async {
    if (value == serverHost) {
      return;
    }
    _snapshot = CoaxSettingsSnapshot(
      serverTransport: serverTransport,
      serverHost: value,
      serverPortText: serverPortText,
      serverUnixPath: serverUnixPath,
    );
    await _persistSnapshot();
    _safeNotify();
    if (isEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> setServerPortText(String value) async {
    if (value == serverPortText) {
      return;
    }
    _snapshot = CoaxSettingsSnapshot(
      serverTransport: serverTransport,
      serverHost: serverHost,
      serverPortText: value,
      serverUnixPath: serverUnixPath,
    );
    await _persistSnapshot();
    _safeNotify();
    if (isEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> setServerUnixPath(String value) async {
    if (value == serverUnixPath) {
      return;
    }
    _snapshot = CoaxSettingsSnapshot(
      serverTransport: serverTransport,
      serverHost: serverHost,
      serverPortText: serverPortText,
      serverUnixPath: value,
    );
    await _persistSnapshot();
    _safeNotify();
    if (isEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> turnOffAfterRpc() async {
    await Future<void>.delayed(const Duration(milliseconds: 100));
    await setEnabled(false);
  }

  @Deprecated('Use turnOffAfterRpc()')
  Future<void> disableAfterRpc() => turnOffAfterRpc();

  Future<void> shutdown() async {
    _disposed = true;
    await _stopServer(clearStatus: true);
  }

  Future<void> _reconfigureRuntime() async {
    _startGeneration += 1;
    final generation = _startGeneration;

    if (!_isEnabled) {
      await _stopServer(clearStatus: true);
      return;
    }

    await _stopServer(clearStatus: false);
    await _startServer(generation);
  }

  Future<void> _startServer(int generation) async {
    listenUri = null;
    statusDetail = null;
    _safeNotify();

    try {
      await _prepareDescribe?.call();
      final server = await holons.startWithOptions(
        _runtimeListenUri(),
        _serviceFactory(),
        options: const holons.ServeOptions(describe: true, logger: _ignoreLog),
      );
      if (generation != _startGeneration || !_isEnabled) {
        await server.stop();
        return;
      }
      _runningServer = server;
      listenUri = server.publicUri;
      statusDetail = null;
      _log('[COAX] server listening on ${server.publicUri}');
    } on Object catch (error) {
      if (generation != _startGeneration) {
        return;
      }
      listenUri = null;
      statusDetail = 'Server surface failed to start: $error';
      _log('[COAX] failed to start server: $error');
    }

    _safeNotify();
  }

  Future<void> _stopServer({required bool clearStatus}) async {
    final server = _runningServer;
    _runningServer = null;
    listenUri = null;
    if (clearStatus) {
      statusDetail = null;
    }
    _safeNotify();
    if (server == null) {
      return;
    }
    _log('[COAX] server stopped');
    await Future<void>.delayed(const Duration(milliseconds: 250));
    await server.stop();
  }

  Future<void> _persistSnapshot() async {
    await _settingsStore.writeString(settingsKey, _snapshot.encode());
  }

  String _runtimeListenUri() {
    switch (serverTransport) {
      case CoaxServerTransport.tcp:
        final host = serverHost.trim().isEmpty
            ? defaults.serverHost
            : serverHost.trim();
        return 'tcp://$host:$serverPort';
      case CoaxServerTransport.unix:
        final path = serverUnixPath.trim().isEmpty
            ? defaults.serverUnixPath
            : serverUnixPath.trim();
        return 'unix://$path';
    }
  }

  bool _usesDefaultServerPort(CoaxServerTransport transport) {
    if (transport == CoaxServerTransport.unix) {
      return serverPortText.trim().isEmpty;
    }
    final trimmed = serverPortText.trim();
    if (trimmed.isEmpty) {
      return true;
    }
    return sanitizedPort(trimmed, transport.defaultPort) ==
        transport.defaultPort;
  }

  static CoaxSettingsSnapshot _loadSnapshot(
    SettingsStore store,
    String settingsKey,
    CoaxSettingsDefaults defaults,
  ) {
    final rawValue = store.readString(settingsKey);
    if (rawValue.trim().isEmpty) {
      return defaults.snapshot();
    }
    try {
      return CoaxSettingsSnapshot.decode(rawValue);
    } on Object {
      return defaults.snapshot();
    }
  }

  void _safeNotify() {
    if (!_disposed) {
      notifyListeners();
    }
  }

  void _log(String message) {
    stderr.writeln(message);
  }

  static void _ignoreLog(String _) {}

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }
}

@Deprecated('Use CoaxManager')
typedef CoaxController = CoaxManager;
