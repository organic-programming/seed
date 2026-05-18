import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/observability.pb.dart' as obs_pb;
import 'package:holons/gen/holons/v1/observability.pbgrpc.dart' as obs_grpc;

import 'connect.dart' as connect_impl;
import 'describe.dart';
import 'observability.dart' as observability;
import 'reflection.dart';
import 'transport.dart';
import 'composite.dart' as composite;

String CurrentTransport = '';

void setCurrentTransport(String transport) {
  CurrentTransport = transport.trim();
}

class ParsedFlags {
  const ParsedFlags({
    required this.listenUri,
    required this.reflect,
  });

  final String listenUri;
  final bool reflect;
}

class ParsedChildFlags {
  const ParsedChildFlags({
    required this.children,
    required this.remaining,
  });

  final List<composite.ChildSpec> children;
  final List<String> remaining;
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

ParsedChildFlags parseChildFlags(List<String> args) {
  final children = <composite.ChildSpec>[];
  final remaining = <String>[];
  for (var i = 0; i < args.length; i++) {
    final arg = args[i];
    if (arg == '--child' && i + 1 < args.length) {
      final parsed = _parseChildSpec(args[i + 1]);
      if (parsed != null) {
        children.add(parsed);
      }
      i += 1;
    } else if (arg.startsWith('--child=')) {
      final parsed = _parseChildSpec(arg.substring('--child='.length));
      if (parsed != null) {
        children.add(parsed);
      }
    } else {
      remaining.add(arg);
    }
  }
  return ParsedChildFlags(children: children, remaining: remaining);
}

composite.ChildSpec? _parseChildSpec(String raw) {
  final index = raw.indexOf('=');
  if (index < 0) {
    return null;
  }
  final slug = raw.substring(0, index).trim();
  final binary = raw.substring(index + 1).trim();
  if (slug.isEmpty || binary.isEmpty) {
    return null;
  }
  return composite.ChildSpec(slug: slug, binary: binary);
}

class ServeOptions {
  const ServeOptions({
    this.describe = true,
    this.reflect = false,
    this.memberEndpoints = const [],
    this.onListen,
    this.logger = _defaultLogger,
    this.protoDir,
    this.environment,
  });

  final bool describe;
  final bool reflect;
  final List<MemberRef> memberEndpoints;
  final void Function(String publicUri)? onListen;
  final void Function(String message) logger;
  final String? protoDir;
  final Map<String, String>? environment;
}

class MemberRef {
  const MemberRef({
    required this.slug,
    this.uid = '',
    required this.address,
  });

  final String slug;
  final String uid;
  final String address;
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
  setCurrentTransport(parsed.scheme);
  try {
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
        return _withCurrentTransportLifecycle(
            await _finalizeObservabilityRuntime(
          running,
          obs,
          running.publicUri,
          parsed.scheme,
          options,
        ));
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
        final finalized = await _finalizeObservabilityRuntime(
          running,
          obs,
          'stdio://',
          parsed.scheme,
          options,
        );
        options.onListen?.call('stdio://');
        options.logger('gRPC server listening on stdio:// ($mode)');
        return _withCurrentTransportLifecycle(finalized);
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
        final running = RunningServer._(
          server: backing.server,
          publicUri: publicUri,
          completion: backing.completion,
          stopCallback: () async {
            await bridge.close();
            await backing.stop();
          },
        );
        final finalized = await _finalizeObservabilityRuntime(
          running,
          obs,
          publicUri,
          parsed.scheme,
          options,
        );
        options.onListen?.call(publicUri);
        options.logger('gRPC server listening on $publicUri ($mode)');
        return _withCurrentTransportLifecycle(finalized);
      default:
        throw ArgumentError.value(
          listenUri,
          'listenUri',
          'Serve.run(...) currently supports tcp://, unix://, and stdio:// only',
        );
    }
  } catch (_) {
    setCurrentTransport('');
    rethrow;
  }
}

RunningServer _withCurrentTransportLifecycle(RunningServer running) {
  return RunningServer._(
    server: running.server,
    publicUri: running.publicUri,
    completion: running.completion,
    stopCallback: () async {
      try {
        await running.stop();
      } finally {
        setCurrentTransport('');
      }
    },
  );
}

Future<RunningServer> _finalizeObservabilityRuntime(
  RunningServer running,
  observability.Observability? obs,
  String publicUri,
  String transportName,
  ServeOptions options,
) async {
  final promServer = await _startPromServer(obs, options.logger);
  _startObservabilityRuntime(
    obs,
    publicUri,
    transportName,
    promServer?.$2 ?? '',
  );
  final memberRelays = await _startMemberRelays(
    obs,
    options.memberEndpoints,
    options.logger,
  );
  if (promServer == null && memberRelays.isEmpty) {
    return running;
  }
  return RunningServer._(
    server: running.server,
    publicUri: running.publicUri,
    completion: running.completion,
    stopCallback: () async {
      await Future.wait<void>([
        for (final relay in memberRelays) relay.stop(),
      ]);
      if (promServer != null) {
        await promServer.$1.close();
      }
      await running.stop();
    },
  );
}

Future<(observability.PromServer, String)?> _startPromServer(
  observability.Observability? obs,
  void Function(String message) logger,
) async {
  if (obs == null || !obs.enabled(observability.Family.prom)) {
    return null;
  }
  final addr = obs.cfg.promAddr.isEmpty ? ':0' : obs.cfg.promAddr;
  final server = observability.PromServer(addr);
  try {
    final metricsAddr = await server.start();
    logger('Prometheus /metrics listening on $metricsAddr');
    return (server, metricsAddr);
  } on Object catch (error) {
    logger('warning: prom HTTP bind failed: $error');
    return null;
  }
}

Future<List<_StartedMemberRelay>> _startMemberRelays(
  observability.Observability? obs,
  List<MemberRef> members,
  void Function(String message) logger,
) async {
  if (obs == null ||
      (!obs.enabled(observability.Family.logs) &&
          !obs.enabled(observability.Family.events))) {
    return const [];
  }
  final relays = <_StartedMemberRelay>[];
  for (final raw in members) {
    var member = MemberRef(
      slug: raw.slug.trim(),
      uid: raw.uid.trim(),
      address: raw.address.trim(),
    );
    if (member.slug.isEmpty || member.address.isEmpty) {
      logger(
          'warning: observability relay skipped incomplete member ref: slug="${member.slug}" uid="${member.uid}" address="${member.address}"');
      continue;
    }
    ClientChannel? channel;
    try {
      channel = await connect_impl.connect(
        member.address,
        const connect_impl.ConnectOptions(start: false),
      ) as ClientChannel;
      member = await _resolveRelayMemberIdentity(channel, member);
      if (member.uid.isEmpty) {
        logger(
            'warning: observability relay uid unresolved for ${member.slug} at ${member.address}; chain hops will have empty uid');
      }
      final relay = observability.MemberRelay(
        childSlug: member.slug,
        childUid: member.uid,
        channel: channel,
        observability: obs,
      );
      await relay.start();
      relays.add(_StartedMemberRelay(relay, channel));
      channel = null;
    } on Object catch (error) {
      await channel?.shutdown();
      logger(
          'warning: observability relay start ${member.slug}/${member.uid}: $error');
    }
  }
  return relays;
}

Future<MemberRef> _resolveRelayMemberIdentity(
  ClientChannel channel,
  MemberRef member,
) async {
  if (member.uid.isNotEmpty) {
    return member;
  }
  final client = obs_grpc.HolonObservabilityClient(channel);
  try {
    final stream = client.events(
      obs_pb.EventsRequest(
        eventNames: [observability.eventInstanceReady],
        follow: false,
      ),
    );
    await for (final event in stream.timeout(const Duration(seconds: 2))) {
      final uid = observability.stringAttribute(
          event.attributes, observability.attrHolonsInstanceUid);
      if (uid.isEmpty || event.chain.isNotEmpty) {
        continue;
      }
      final slug = observability.stringAttribute(
          event.attributes, observability.attrHolonsSlug);
      return MemberRef(
        slug: slug.trim().isEmpty ? member.slug : slug.trim(),
        uid: uid.trim(),
        address: member.address,
      );
    }
  } on Object {
    // Fall back to Metrics below.
  }

  try {
    final metrics = await client
        .metrics(obs_pb.MetricsRequest())
        .toList()
        .timeout(const Duration(seconds: 2));
    for (final metric in metrics) {
      final attrs = _metricAttributes(metric);
      final uid = observability.stringAttribute(
          attrs, observability.attrHolonsInstanceUid);
      if (uid.isEmpty) {
        continue;
      }
      final slug =
          observability.stringAttribute(attrs, observability.attrHolonsSlug);
      return MemberRef(
          slug: slug.trim().isEmpty ? member.slug : slug.trim(),
          uid: uid.trim(),
          address: member.address);
    }
  } on Object {
    // Leave the UID empty and let the relay still run.
  }
  return member;
}

List<obs_pb.KeyValue> _metricAttributes(obs_pb.Metric metric) {
  if (metric.hasGauge() && metric.gauge.dataPoints.isNotEmpty) {
    return metric.gauge.dataPoints.first.attributes;
  }
  if (metric.hasSum() && metric.sum.dataPoints.isNotEmpty) {
    return metric.sum.dataPoints.first.attributes;
  }
  if (metric.hasHistogram() && metric.histogram.dataPoints.isNotEmpty) {
    return metric.histogram.dataPoints.first.attributes;
  }
  return const [];
}

class _StartedMemberRelay {
  _StartedMemberRelay(this.relay, this.channel);

  final observability.MemberRelay relay;
  final ClientChannel channel;

  Future<void> stop() async {
    await relay.stop();
    await channel.shutdown();
  }
}

observability.Observability? _resolveObservability(Map<String, String> env) {
  if ((env['OP_OBS'] ?? '').trim().isEmpty) {
    return null;
  }
  final current = observability.current();
  if (current.families.isNotEmpty && current.cfg.slug.trim().isNotEmpty) {
    return current;
  }
  return observability.fromEnv(
    observability.Config(slug: registeredManifestSlug()),
    env,
  );
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
  String metricsAddr,
) {
  if (obs == null || obs.families.isEmpty || obs.cfg.runDir.isEmpty) return;
  observability.enableDiskWriters(obs.cfg.runDir);
  if (obs.enabled(observability.Family.events)) {
    obs.emit(observability.eventInstanceReady,
        payload: {'listener': publicUri, 'metrics_addr': metricsAddr});
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
      metricsAddr: metricsAddr,
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
