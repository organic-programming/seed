import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';

import 'describe.dart';
import 'observability.dart' as observability;
import 'reflection.dart';
import 'transport.dart';

class ParsedFlags {
  const ParsedFlags({
    required this.listenUri,
    required this.reflect,
  });

  final String listenUri;
  final bool reflect;
}

/// Parse --listen or --port from command-line args.
String parseFlags(List<String> args) => parseOptions(args).listenUri;

ParsedFlags parseOptions(List<String> args) {
  var listenUri = defaultUri;
  var reflect = false;
  for (var i = 0; i < args.length; i++) {
    if (args[i] == '--listen' && i + 1 < args.length) {
      listenUri = args[i + 1];
    }
    if (args[i] == '--port' && i + 1 < args.length) {
      listenUri = 'tcp://:${args[i + 1]}';
    }
    if (args[i] == '--reflect') {
      reflect = true;
    }
  }
  return ParsedFlags(listenUri: listenUri, reflect: reflect);
}

class ServeOptions {
  const ServeOptions({
    this.describe = true,
    this.reflect = false,
    this.onListen,
    this.logger = _defaultLogger,
    this.protoDir,
    this.environment,
  });

  final bool describe;
  final bool reflect;
  final void Function(String publicUri)? onListen;
  final void Function(String message) logger;
  final String? protoDir;
  final Map<String, String>? environment;
}

class RunningServer {
  RunningServer._({
    required this.server,
    required this.publicUri,
    required Future<void> completion,
    required Future<void> Function() stopCallback,
  })  : completion = completion,
        _stopCallback = stopCallback;

  final Server server;
  final String publicUri;
  final Future<void> completion;
  final Future<void> Function() _stopCallback;
  bool _stopped = false;

  Future<void> stop() async {
    if (_stopped) {
      await completion;
      return;
    }
    _stopped = true;
    await _stopCallback();
  }
}

Future<void> run(
  String listenUri,
  List<Service> services, {
  ServeOptions options = const ServeOptions(),
}) {
  return runWithOptions(listenUri, services, options: options);
}

Future<void> runWithOptions(
  String listenUri,
  List<Service> services, {
  ServeOptions options = const ServeOptions(),
}) async {
  final running = await startWithOptions(listenUri, services, options: options);

  late final StreamSubscription<ProcessSignal> sigintSub;
  sigintSub = ProcessSignal.sigint.watch().listen((_) async {
    options.logger('shutting down gRPC server');
    await running.stop();
  });

  StreamSubscription<ProcessSignal>? sigtermSub;
  try {
    sigtermSub = ProcessSignal.sigterm.watch().listen((_) async {
      options.logger('shutting down gRPC server');
      await running.stop();
    });
  } on UnsupportedError {
    sigtermSub = null;
  }

  try {
    await running.completion;
  } finally {
    await sigintSub.cancel();
    await sigtermSub?.cancel();
  }
}

Future<RunningServer> startWithOptions(
  String listenUri,
  List<Service> services, {
  ServeOptions options = const ServeOptions(),
}) async {
  final parsed = parseUri(listenUri.isEmpty ? defaultUri : listenUri);
  final resolvedServices = List<Service>.from(services);
  final env = options.environment ?? Platform.environment;
  observability.checkEnv(env);
  final obs = _resolveObservability(env);
  final describeEnabled = _maybeAddDescribe(resolvedServices, options);
  if (obs != null && obs.families.isNotEmpty) {
    resolvedServices.add(observability.registerService(obs));
  }
  final reflectionEnabled = _maybeAddReflection(resolvedServices, options);

  switch (parsed.scheme) {
    case 'tcp':
      final host = parsed.host ?? '0.0.0.0';
      final port = parsed.port ?? 9090;
      final running = await _startTcpServer(
        host: host,
        port: port,
        publicUri: null,
        services: resolvedServices,
        describeEnabled: describeEnabled,
        reflectionEnabled: reflectionEnabled,
        options: options,
      );
      _startObservabilityRuntime(obs, running.publicUri, parsed.scheme);
      return running;
    case 'stdio':
      final backing = await _startTcpServer(
        host: '127.0.0.1',
        port: 0,
        publicUri: null,
        services: resolvedServices,
        describeEnabled: describeEnabled,
        reflectionEnabled: reflectionEnabled,
        options: options,
        suppressAnnouncement: true,
      );
      final port = int.parse(backing.publicUri.split(':').last);
      late final RunningServer running;
      final bridge = await _StdioServerBridge.connect(
        host: '127.0.0.1',
        port: port,
        onDisconnect: () {
          unawaited(running.stop());
        },
      );
      running = RunningServer._(
        server: backing.server,
        publicUri: 'stdio://',
        completion: backing.completion,
        stopCallback: () async {
          await bridge.close();
          await backing.stop();
        },
      );
      bridge.start();
      final mode = _formatMode(describeEnabled, reflectionEnabled);
      _startObservabilityRuntime(obs, 'stdio://', parsed.scheme);
      options.onListen?.call('stdio://');
      options.logger('gRPC server listening on stdio:// ($mode)');
      return running;
    case 'unix':
      final path = parsed.path ?? '';
      final backing = await _startTcpServer(
        host: '127.0.0.1',
        port: 0,
        publicUri: null,
        services: resolvedServices,
        describeEnabled: describeEnabled,
        reflectionEnabled: reflectionEnabled,
        options: options,
        suppressAnnouncement: true,
      );
      final port = int.parse(backing.publicUri.split(':').last);
      final bridge = await _UnixServerBridge.bind(
        path: path,
        host: '127.0.0.1',
        port: port,
      );
      final publicUri = 'unix://$path';
      final mode = _formatMode(describeEnabled, reflectionEnabled);
      _startObservabilityRuntime(obs, publicUri, parsed.scheme);
      options.onListen?.call(publicUri);
      options.logger('gRPC server listening on $publicUri ($mode)');
      return RunningServer._(
        server: backing.server,
        publicUri: publicUri,
        completion: backing.completion,
        stopCallback: () async {
          await bridge.close();
          await backing.stop();
        },
      );
    default:
      throw ArgumentError.value(
        listenUri,
        'listenUri',
        'Serve.run(...) currently supports tcp://, unix://, and stdio:// only',
      );
  }
}

observability.Observability? _resolveObservability(Map<String, String> env) {
  if ((env['OP_OBS'] ?? '').trim().isEmpty) {
    return null;
  }
  final current = observability.current();
  if (current.families.isNotEmpty) {
    return current;
  }
  return observability.fromEnv(const observability.Config(), env);
}

Future<RunningServer> _startTcpServer({
  required String host,
  required int port,
  required String? publicUri,
  required List<Service> services,
  required bool describeEnabled,
  required bool reflectionEnabled,
  required ServeOptions options,
  bool suppressAnnouncement = false,
}) async {
  final server = Server.create(services: services);
  final completion = Completer<void>();

  await server.serve(
    address: _bindAddress(host),
    port: port,
  );

  final advertised =
      publicUri ?? 'tcp://${_advertisedHost(host)}:${server.port!}';
  final mode = _formatMode(describeEnabled, reflectionEnabled);
  if (!suppressAnnouncement) {
    options.onListen?.call(advertised);
    options.logger('gRPC server listening on $advertised ($mode)');
  }

  return RunningServer._(
    server: server,
    publicUri: advertised,
    completion: completion.future,
    stopCallback: () async {
      if (!completion.isCompleted) {
        await server.shutdown();
        completion.complete();
      }
    },
  );
}

void _startObservabilityRuntime(
  observability.Observability? obs,
  String publicUri,
  String transportName,
) {
  if (obs == null || obs.families.isEmpty || obs.cfg.runDir.isEmpty) return;
  observability.enableDiskWriters(obs.cfg.runDir);
  if (obs.enabled(observability.Family.events)) {
    obs.emit(observability.EventType.instanceReady,
        payload: {'listener': publicUri});
  }
  observability.writeMetaJson(
    obs.cfg.runDir,
    observability.MetaJson(
      slug: obs.cfg.slug,
      uid: obs.cfg.instanceUid,
      pid: pid,
      startedAt: DateTime.now(),
      transport: transportName,
      address: publicUri,
      logPath: obs.enabled(observability.Family.logs)
          ? '${obs.cfg.runDir}${Platform.pathSeparator}stdout.log'
          : '',
      organismUid: obs.cfg.organismUid,
      organismSlug: obs.cfg.organismSlug,
    ),
  );
}

bool _maybeAddDescribe(List<Service> services, ServeOptions options) {
  if (!options.describe) {
    return false;
  }

  try {
    services.add(register());
    return true;
  } on Object catch (error) {
    options.logger('HolonMeta registration failed: $error');
    rethrow;
  }
}

bool _maybeAddReflection(List<Service> services, ServeOptions options) {
  if (!options.reflect) {
    return false;
  }

  services.add(
    reflectionService(
      protoDir: options.protoDir ?? '.',
    ),
  );
  return true;
}

String _formatMode(bool describeEnabled, bool reflectionEnabled) {
  final describeMode = describeEnabled ? 'Describe ON' : 'Describe OFF';
  final reflectionMode = reflectionEnabled ? 'reflection ON' : 'reflection OFF';
  return '$describeMode, $reflectionMode';
}

InternetAddress _bindAddress(String host) {
  switch (host) {
    case '':
    case '0.0.0.0':
      return InternetAddress.anyIPv4;
    case '::':
      return InternetAddress.anyIPv6;
    default:
      return InternetAddress(host);
  }
}

String _advertisedHost(String host) {
  switch (host) {
    case '':
    case '0.0.0.0':
      return '127.0.0.1';
    case '::':
      return '::1';
    default:
      return host;
  }
}

void _defaultLogger(String message) {
  stderr.writeln(message);
}

class _StdioServerBridge {
  _StdioServerBridge._({
    required Socket socket,
    required void Function() onDisconnect,
  })  : _socket = socket,
        _onDisconnect = onDisconnect;

  final Socket _socket;
  final void Function() _onDisconnect;
  bool _closed = false;
  int _pendingPumps = 2;
  StreamSubscription<List<int>>? _stdinSub;
  StreamSubscription<List<int>>? _socketSub;

  static Future<_StdioServerBridge> connect({
    required String host,
    required int port,
    required void Function() onDisconnect,
  }) async {
    final socket = await Socket.connect(host, port);
    return _StdioServerBridge._(socket: socket, onDisconnect: onDisconnect);
  }

  void start() {
    _stdinSub = stdin.listen(
      (data) {
        if (_closed) {
          return;
        }
        _socket.add(data);
      },
      onError: (_) => _markPumpDone(),
      onDone: () async {
        try {
          await _socket.close();
        } catch (_) {}
        _markPumpDone();
      },
      cancelOnError: true,
    );

    _socketSub = _socket.listen(
      (data) async {
        if (_closed) {
          return;
        }
        stdout.add(data);
        await stdout.flush();
      },
      onError: (_) => _markPumpDone(),
      onDone: _markPumpDone,
      cancelOnError: true,
    );
  }

  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;
    await _stdinSub?.cancel();
    await _socketSub?.cancel();
    _socket.destroy();
  }

  void _markPumpDone() {
    if (_pendingPumps <= 0) {
      return;
    }
    _pendingPumps -= 1;
    if (_pendingPumps == 0) {
      _onDisconnect();
    }
  }
}

class _UnixServerBridge {
  _UnixServerBridge._({
    required ServerSocket listener,
    required String path,
    required String targetHost,
    required int targetPort,
  })  : _listener = listener,
        _path = path,
        _targetHost = targetHost,
        _targetPort = targetPort;

  final ServerSocket _listener;
  final String _path;
  final String _targetHost;
  final int _targetPort;
  final Set<Socket> _activeSockets = <Socket>{};
  StreamSubscription<Socket>? _acceptSub;
  bool _closed = false;

  static Future<_UnixServerBridge> bind({
    required String path,
    required String host,
    required int port,
  }) async {
    final socketFile = File(path);
    if (socketFile.existsSync()) {
      try {
        socketFile.deleteSync();
      } catch (_) {}
    }

    final listener = await ServerSocket.bind(
      InternetAddress(path, type: InternetAddressType.unix),
      0,
    );
    final bridge = _UnixServerBridge._(
      listener: listener,
      path: path,
      targetHost: host,
      targetPort: port,
    );
    bridge.start();
    return bridge;
  }

  void start() {
    _acceptSub = _listener.listen(
      (client) async {
        if (_closed) {
          client.destroy();
          return;
        }

        Socket? upstream;
        try {
          upstream = await Socket.connect(_targetHost, _targetPort);
          _track(client);
          _track(upstream);
          _pipePair(client, upstream);
        } catch (_) {
          client.destroy();
          upstream?.destroy();
        }
      },
      onError: (_) {},
      cancelOnError: false,
    );
  }

  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;
    await _acceptSub?.cancel();
    try {
      await _listener.close();
    } catch (_) {}
    for (final socket in _activeSockets.toList()) {
      socket.destroy();
    }
    _activeSockets.clear();
    final socketFile = File(_path);
    if (socketFile.existsSync()) {
      try {
        socketFile.deleteSync();
      } catch (_) {}
    }
  }

  void _pipePair(Socket client, Socket upstream) {
    StreamSubscription<List<int>>? clientSub;
    StreamSubscription<List<int>>? upstreamSub;
    var shuttingDown = false;

    Future<void> shutdownPair() async {
      if (shuttingDown) {
        return;
      }
      shuttingDown = true;
      await clientSub?.cancel();
      await upstreamSub?.cancel();
      _untrack(client);
      _untrack(upstream);
      client.destroy();
      upstream.destroy();
    }

    clientSub = client.listen(
      (data) {
        if (_closed) {
          return;
        }
        unawaited(_forwardSocketData(upstream, data, shutdownPair));
      },
      onError: (_) => unawaited(shutdownPair()),
      onDone: () => unawaited(shutdownPair()),
      cancelOnError: true,
    );

    upstreamSub = upstream.listen(
      (data) {
        if (_closed) {
          return;
        }
        unawaited(_forwardSocketData(client, data, shutdownPair));
      },
      onError: (_) => unawaited(shutdownPair()),
      onDone: () => unawaited(shutdownPair()),
      cancelOnError: true,
    );
  }

  Future<void> _forwardSocketData(
    Socket target,
    List<int> data,
    Future<void> Function() shutdownPair,
  ) async {
    try {
      target.add(data);
      await target.flush();
    } catch (_) {
      await shutdownPair();
    }
  }

  void _track(Socket socket) {
    if (_closed) {
      socket.destroy();
      return;
    }
    _activeSockets.add(socket);
  }

  void _untrack(Socket socket) {
    _activeSockets.remove(socket);
  }
}
