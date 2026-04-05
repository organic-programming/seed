import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

import '../model/app_model.dart';
import '../rpc/coax_service.dart';
import '../rpc/greeting_app_service.dart';
import '../runtime/describe_registration.dart';
import '../settings_store.dart';
import 'greeting_controller.dart';

class CoaxController extends ChangeNotifier {
  CoaxController({
    required GreetingController greetingController,
    required SettingsStore settingsStore,
    AppPlatformCapabilities? capabilities,
  }) : _greetingController = greetingController,
       _settingsStore = settingsStore,
       capabilities = capabilities ?? AppPlatformCapabilities.desktopCurrent(),
       _isEnabled = settingsStore.readBool(_enabledKey),
       _snapshot = _loadSnapshot(settingsStore) {
    if (!this.capabilities.supportsUnixSockets &&
        _snapshot.serverTransport == CoaxServerTransport.unix) {
      _snapshot = CoaxSettingsSnapshot.defaults;
    }
  }

  static const _enabledKey = 'coax.server.enabled';
  static const _settingsKey = 'coax.server.settings';

  final GreetingController _greetingController;
  final SettingsStore _settingsStore;
  final AppPlatformCapabilities capabilities;

  holons.RunningServer? _runningServer;
  bool _disposed = false;
  bool _isEnabled;
  CoaxSettingsSnapshot _snapshot;
  String? listenUri;
  String? statusDetail;
  int _startGeneration = 0;

  bool get isEnabled => _isEnabled;
  bool get serverEnabled => _snapshot.serverEnabled;
  CoaxServerTransport get serverTransport => _snapshot.serverTransport;
  String get serverHost => _snapshot.serverHost;
  String get serverPortText => _snapshot.serverPortText;
  String get serverUnixPath => _snapshot.serverUnixPath;

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
            ? CoaxSettingsSnapshot.defaultHost
            : serverHost.trim();
        return 'tcp://$host:$serverPort';
      case CoaxServerTransport.unix:
        final path = serverUnixPath.trim().isEmpty
            ? CoaxSettingsSnapshot.defaultUnixPath
            : serverUnixPath.trim();
        return 'unix://$path';
    }
  }

  CoaxSurfaceStatus get serverStatus {
    return CoaxSurfaceStatus(
      id: 'server',
      title: 'Server',
      endpoint: serverEnabled
          ? (listenUri ?? serverPreviewEndpoint)
          : serverPreviewEndpoint,
      state: _serverSurfaceState,
    );
  }

  CoaxSurfaceState get _serverSurfaceState {
    if (!serverEnabled) {
      return CoaxSurfaceState.off;
    }
    if (statusDetail != null && isEnabled) {
      return CoaxSurfaceState.error;
    }
    if (listenUri != null) {
      return CoaxSurfaceState.live;
    }
    if (!isEnabled) {
      return CoaxSurfaceState.saved;
    }
    return CoaxSurfaceState.announced;
  }

  Future<void> startIfEnabled() async {
    if (!_isEnabled) {
      return;
    }
    await _reconfigureRuntime();
  }

  Future<void> setIsEnabled(bool value) async {
    if (_isEnabled == value) {
      return;
    }
    _isEnabled = value;
    await _settingsStore.writeBool(_enabledKey, value);
    _safeNotify();
    await _reconfigureRuntime();
  }

  Future<void> setServerEnabled(bool value) async {
    if (serverEnabled == value) {
      return;
    }
    _snapshot = CoaxSettingsSnapshot(
      serverEnabled: value,
      serverTransport: serverTransport,
      serverHost: serverHost,
      serverPortText: serverPortText,
      serverUnixPath: serverUnixPath,
    );
    await _persistSnapshot();
    _safeNotify();
    await _reconfigureRuntime();
  }

  Future<void> setServerTransport(CoaxServerTransport value) async {
    if (value == serverTransport) {
      return;
    }
    final nextPortText = _usesDefaultServerPort(serverTransport)
        ? value.defaultPort.toString()
        : serverPortText;
    _snapshot = CoaxSettingsSnapshot(
      serverEnabled: serverEnabled,
      serverTransport: value,
      serverHost: serverHost,
      serverPortText: nextPortText,
      serverUnixPath: serverUnixPath,
    );
    await _persistSnapshot();
    _safeNotify();
    if (serverEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> setServerHost(String value) async {
    if (value == serverHost) {
      return;
    }
    _snapshot = CoaxSettingsSnapshot(
      serverEnabled: serverEnabled,
      serverTransport: serverTransport,
      serverHost: value,
      serverPortText: serverPortText,
      serverUnixPath: serverUnixPath,
    );
    await _persistSnapshot();
    _safeNotify();
    if (serverEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> setServerPortText(String value) async {
    if (value == serverPortText) {
      return;
    }
    _snapshot = CoaxSettingsSnapshot(
      serverEnabled: serverEnabled,
      serverTransport: serverTransport,
      serverHost: serverHost,
      serverPortText: value,
      serverUnixPath: serverUnixPath,
    );
    await _persistSnapshot();
    _safeNotify();
    if (serverEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> setServerUnixPath(String value) async {
    if (value == serverUnixPath) {
      return;
    }
    _snapshot = CoaxSettingsSnapshot(
      serverEnabled: serverEnabled,
      serverTransport: serverTransport,
      serverHost: serverHost,
      serverPortText: serverPortText,
      serverUnixPath: value,
    );
    await _persistSnapshot();
    _safeNotify();
    if (serverEnabled) {
      await _reconfigureRuntime();
    }
  }

  Future<void> disableAfterRpc() async {
    await Future<void>.delayed(const Duration(milliseconds: 100));
    await setIsEnabled(false);
  }

  Future<void> shutdown() async {
    _disposed = true;
    await _stopServer(clearStatus: true);
  }

  Future<void> _reconfigureRuntime() async {
    _startGeneration += 1;
    final generation = _startGeneration;

    if (!_isEnabled || !serverEnabled) {
      await _stopServer(clearStatus: true);
      return;
    }

    await _stopServer(clearStatus: false);
    await _startServer(generation);
  }

  Future<void> _startServer(int generation) async {
    final appService = GreetingAppRpcService(_greetingController);
    final coaxService = CoaxRpcService(
      greetingController: _greetingController,
      coaxController: this,
    );
    listenUri = null;
    statusDetail = null;
    _safeNotify();

    try {
      ensureAppDescribeRegistered();
      final server = await holons.startWithOptions(
        _runtimeListenUri(),
        <Service>[coaxService, appService],
        options: const holons.ServeOptions(describe: true, logger: _ignoreLog),
      );
      if (generation != _startGeneration || !_isEnabled || !serverEnabled) {
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
    await _settingsStore.writeString(_settingsKey, _snapshot.encode());
  }

  String _runtimeListenUri() {
    switch (serverTransport) {
      case CoaxServerTransport.tcp:
        final host = serverHost.trim().isEmpty
            ? CoaxSettingsSnapshot.defaultHost
            : serverHost.trim();
        return 'tcp://$host:$serverPort';
      case CoaxServerTransport.unix:
        final path = serverUnixPath.trim().isEmpty
            ? CoaxSettingsSnapshot.defaultUnixPath
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

  static CoaxSettingsSnapshot _loadSnapshot(SettingsStore store) {
    final rawValue = store.readString(_settingsKey);
    if (rawValue.trim().isEmpty) {
      return CoaxSettingsSnapshot.defaults;
    }
    try {
      return CoaxSettingsSnapshot.decode(rawValue);
    } on Object {
      return CoaxSettingsSnapshot.defaults;
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
