import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:grpc/grpc.dart';
import 'package:grpc/service_api.dart' as grpc_api;
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart' as describe_pb;
import 'package:holons/gen/holons/v1/observability.pbgrpc.dart' as obs_pb;

import 'grpcclient.dart';
import 'observability.dart' as observability;

const List<String> transportCoverageSequence = [
  'stdio',
  'stdio',
  'tcp',
  'unix',
  'tcp',
  'tcp',
  'stdio',
  'unix',
  'unix',
  'stdio',
];

/// Resolve a declared member's binary relative to the calling composite's own
/// executable.
String member(String id) {
  return memberFromExecutable(Platform.resolvedExecutable, id);
}

String memberFromExecutable(String selfPath, String id) {
  final trimmed = id.trim();
  if (trimmed.isEmpty) {
    throw ArgumentError.value(id, 'id', 'member id is required');
  }
  final self = File(selfPath).absolute;
  final memberDir = Directory(
    '${self.parent.path}${Platform.pathSeparator}holons${Platform.pathSeparator}$trimmed',
  );
  for (final entity in memberDir.listSync(followLinks: false)) {
    if (entity is File && _isExecutable(entity)) {
      return entity.absolute.path;
    }
  }
  throw FileSystemException('no executable found', memberDir.path);
}

bool _isExecutable(File file) {
  final stat = file.statSync();
  if (stat.type != FileSystemEntityType.file) {
    return false;
  }
  if (Platform.isWindows) {
    return file.uri.pathSegments.last.toLowerCase().endsWith('.exe');
  }
  return stat.mode & 0x49 != 0; // 0o111
}

class ChildSpec {
  const ChildSpec({required this.slug, required this.binary});

  final String slug;
  final String binary;
}

class SpawnOptions {
  const SpawnOptions({
    this.slug = '',
    required this.binaryPath,
    this.transport = 'stdio',
    this.instanceUid = '',
    this.downstreamChain = const [],
    this.extraEnv = const {},
    this.withTransitiveObservability = true,
  });

  final String slug;
  final String binaryPath;
  final String transport;
  final String instanceUid;
  final List<ChildSpec> downstreamChain;
  final Map<String, String> extraEnv;
  final bool withTransitiveObservability;
}

class SpawnedMember {
  SpawnedMember({
    required this.slug,
    required this.uid,
    required this.listenUri,
    required this.conn,
    required Process process,
    observability.MemberRelay? relay,
  })  : _process = process,
        _relay = relay;

  final String slug;
  final String uid;
  final String listenUri;
  final grpc_api.ClientChannel conn;
  final Process _process;
  final observability.MemberRelay? _relay;
  bool _stopped = false;

  Future<void> stop({Duration timeout = const Duration(seconds: 3)}) async {
    if (_stopped) {
      return;
    }
    _stopped = true;
    await _relay?.stop();
    try {
      await conn.shutdown().timeout(const Duration(seconds: 1));
    } on Object {
      // Process teardown below is authoritative.
    }
    _process.kill(ProcessSignal.sigterm);
    try {
      await _process.exitCode.timeout(timeout);
    } on TimeoutException {
      _process.kill(ProcessSignal.sigkill);
      await _process.exitCode.timeout(const Duration(seconds: 1));
    }
  }
}

Future<SpawnedMember> spawnMember({
  required String binaryPath,
  String slug = '',
  String transport = 'stdio',
  String instanceUid = '',
  List<ChildSpec> downstreamChain = const [],
  Map<String, String> extraEnv = const {},
  bool withTransitiveObservability = true,
}) {
  return spawnMemberWithOptions(SpawnOptions(
    slug: slug,
    binaryPath: binaryPath,
    transport: transport,
    instanceUid: instanceUid,
    downstreamChain: downstreamChain,
    extraEnv: extraEnv,
    withTransitiveObservability: withTransitiveObservability,
  ));
}

Future<SpawnedMember> spawnMemberWithOptions(SpawnOptions opts) async {
  final slug = opts.slug.trim().isNotEmpty
      ? opts.slug.trim()
      : _basename(opts.binaryPath.trim());
  if (slug.isEmpty) {
    throw ArgumentError('spawn member requires a slug');
  }
  final binary = opts.binaryPath.trim();
  if (binary.isEmpty) {
    throw ArgumentError('spawn member $slug requires a binary path');
  }
  final uid = opts.instanceUid.trim().isNotEmpty
      ? opts.instanceUid.trim()
      : _newInstanceUid();
  final transportName = opts.transport.trim().isEmpty
      ? 'stdio'
      : opts.transport.trim().toLowerCase();
  final listenUri = _listenUriForSpawn(transportName, slug, uid);
  if (listenUri.startsWith('unix://')) {
    final socket = File(listenUri.substring('unix://'.length));
    if (socket.existsSync()) {
      socket.deleteSync();
    }
  }

  final args = <String>[
    'serve',
    '--listen',
    listenUri,
    '--transport',
    transportName,
  ];
  for (final child in opts.downstreamChain) {
    if (child.slug.trim().isEmpty || child.binary.trim().isEmpty) {
      throw ArgumentError('downstream child requires slug and binary');
    }
    args.addAll(['--child', '${child.slug}=${child.binary}']);
  }

  final env = _spawnEnvironment(uid, opts.extraEnv);
  final process = await Process.start(
    binary,
    args,
    workingDirectory: File(binary).absolute.parent.path,
    environment: env,
  );
  unawaited(process.stderr.drain<void>());
  if (transportName != 'stdio') {
    unawaited(process.stdout.drain<void>());
  }

  late grpc_api.ClientChannel channel;
  var advertised = listenUri;
  if (transportName == 'stdio') {
    final connector = StdioTransportConnector.fromProcess(process);
    channel = ClientTransportConnectorChannel(
      connector,
      options: const ChannelOptions(
        credentials: ChannelCredentials.insecure(),
        idleTimeout: null,
      ),
    );
    advertised = 'stdio://';
  } else {
    try {
      final meta = await _waitMeta(env['OP_RUN_DIR']!, slug, uid);
      advertised = meta.address;
      channel = await _dialReady(_normalizeDialTarget(advertised));
    } on Object {
      process.kill(ProcessSignal.sigterm);
      await process.exitCode.timeout(
        const Duration(seconds: 2),
        onTimeout: () {
          process.kill(ProcessSignal.sigkill);
          return process.exitCode;
        },
      );
      rethrow;
    }
  }

  observability.MemberRelay? relay;
  if (opts.withTransitiveObservability) {
    relay = observability.MemberRelay(
      childSlug: slug,
      childUid: uid,
      channel: channel,
      observability: observability.current(),
      forceStreams: true,
    );
    await relay.start();
    await _waitRelayedReady(uid);
    await Future<void>.delayed(const Duration(milliseconds: 25));
  }

  return SpawnedMember(
    slug: slug,
    uid: uid,
    listenUri: advertised,
    conn: channel,
    process: process,
    relay: relay,
  );
}

class CascadeOptions {
  const CascadeOptions({
    required this.transport,
    required this.members,
    this.extraEnv = const {},
  });

  final String transport;
  final List<ChildSpec> members;
  final Map<String, String> extraEnv;
}

class Cascade {
  const Cascade({required this.top});

  final SpawnedMember top;

  Future<void> stop() => top.stop();
}

Future<Cascade> buildCascade(CascadeOptions opts) async {
  if (opts.members.isEmpty) {
    throw ArgumentError('buildCascade requires at least one member');
  }
  final top = opts.members.first;
  final spawned = await spawnMember(
    slug: top.slug,
    binaryPath: top.binary,
    transport: opts.transport,
    downstreamChain: opts.members.skip(1).toList(),
    extraEnv: opts.extraEnv,
  );
  return Cascade(top: spawned);
}

Map<String, String> _spawnEnvironment(
  String uid,
  Map<String, String> extra,
) {
  final env = Map<String, String>.from(Platform.environment);
  env['OP_INSTANCE_UID'] = uid;
  env['OP_RUN_DIR'] = _runRoot(env);
  env['HOLONS_PARENT_PID'] = '$pid';
  final families = _activeObservabilityFamilies();
  if (families.isNotEmpty) {
    env['OP_OBS'] = families;
  }
  env.addAll(extra);
  return env;
}

String _activeObservabilityFamilies() {
  final current = observability.current();
  return const <observability.Family>[
    observability.Family.logs,
    observability.Family.metrics,
    observability.Family.events,
    observability.Family.prom,
  ].where(current.enabled).map((family) => family.name).join(',');
}

String _runRoot(Map<String, String> env) {
  final configured = (env['OP_RUN_DIR'] ?? '').trim();
  if (configured.isNotEmpty) {
    return configured;
  }
  final opPath = (env['OPPATH'] ?? '').trim();
  if (opPath.isNotEmpty) {
    return '$opPath${Platform.pathSeparator}run';
  }
  final home = (env['HOME'] ?? '').trim();
  if (home.isNotEmpty) {
    return '$home${Platform.pathSeparator}.op${Platform.pathSeparator}run';
  }
  return '${Directory.systemTemp.path}${Platform.pathSeparator}.op${Platform.pathSeparator}run';
}

String _listenUriForSpawn(String transport, String _, String uid) {
  switch (transport) {
    case 'stdio':
      return 'stdio://';
    case 'tcp':
      return 'tcp://127.0.0.1:0';
    case 'unix':
      final name = 'op-${_cleanSocketToken(uid)}.sock';
      return 'unix://${Directory.systemTemp.path}${Platform.pathSeparator}$name';
    default:
      throw ArgumentError('unsupported transport "$transport"');
  }
}

class _Meta {
  const _Meta(this.address);

  final String address;
}

Future<_Meta> _waitMeta(
  String runRoot,
  String slug,
  String uid, {
  Duration timeout = const Duration(seconds: 10),
}) async {
  final file = File([
    runRoot,
    slug,
    uid,
    'meta.json',
  ].join(Platform.pathSeparator));
  final deadline = DateTime.now().add(timeout);
  Object? lastError;
  while (DateTime.now().isBefore(deadline)) {
    try {
      final payload =
          jsonDecode(await file.readAsString()) as Map<String, dynamic>;
      if (payload['uid'] == uid) {
        final address = payload['address'] as String? ?? '';
        if (address.isNotEmpty) {
          return _Meta(address);
        }
      }
    } on Object catch (error) {
      lastError = error;
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError(
      'meta not ready for $slug/$uid: ${lastError ?? 'not found'}');
}

Future<void> _waitRelayedReady(
  String uid, {
  Duration timeout = const Duration(seconds: 1),
}) async {
  final obs = observability.current();
  if (!obs.enabled(observability.Family.events) || obs.eventBus == null) {
    return;
  }
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    final ready = obs.eventBus!.drain().any((event) =>
        event.type == observability.EventType.instanceReady &&
        event.instanceUid == uid);
    if (ready) {
      return;
    }
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
  throw StateError('transitive observability not ready for $uid');
}

Future<grpc_api.ClientChannel> _dialReady(
  String target, {
  Duration timeout = const Duration(seconds: 10),
}) async {
  final deadline = DateTime.now().add(timeout);
  Object? lastError;
  while (DateTime.now().isBefore(deadline)) {
    final channel = _dialTarget(target);
    try {
      await describe_pb.HolonMetaClient(channel)
          .describe(describe_pb.DescribeRequest())
          .timeout(const Duration(milliseconds: 500));
      return channel;
    } on Object catch (error) {
      lastError = error;
      await channel.shutdown();
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('${lastError ?? 'dial timeout'}');
}

grpc_api.ClientChannel _dialTarget(String target) {
  if (target.startsWith('unix://')) {
    return ClientChannel(
      InternetAddress(
        target.substring('unix://'.length),
        type: InternetAddressType.unix,
      ),
      port: 0,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
  }
  final index = target.lastIndexOf(':');
  if (index <= 0 || index == target.length - 1) {
    throw ArgumentError('invalid host:port target: $target');
  }
  return ClientChannel(
    target.substring(0, index),
    port: int.parse(target.substring(index + 1)),
    options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
  );
}

String _normalizeDialTarget(String target) {
  if (target.startsWith('tcp://')) {
    final uri = Uri.parse(target);
    final host =
        uri.host.isEmpty || uri.host == '0.0.0.0' ? '127.0.0.1' : uri.host;
    return '$host:${uri.port}';
  }
  return target;
}

String _newInstanceUid() {
  final random = Random.secure();
  String hex(int value, int width) =>
      value.toRadixString(16).padLeft(width, '0');
  return [
    hex(DateTime.now().microsecondsSinceEpoch, 12),
    hex(random.nextInt(1 << 32), 8),
    hex(random.nextInt(1 << 32), 8),
  ].join('-');
}

String _basename(String path) {
  final normalized = path.replaceAll('\\', '/');
  final trimmed = normalized.endsWith('/') && normalized.length > 1
      ? normalized.substring(0, normalized.length - 1)
      : normalized;
  final index = trimmed.lastIndexOf('/');
  return index >= 0 ? trimmed.substring(index + 1) : trimmed;
}

String _cleanSocketToken(String value) {
  var out = value.trim();
  if (out.length > 24) {
    out = out.substring(0, 24);
  }
  return out
      .replaceAll('/', '-')
      .replaceAll('\\', '-')
      .replaceAll(':', '-')
      .replaceAll(' ', '-');
}

typedef ChainHop = observability.Hop;

class CheckOutcome {
  const CheckOutcome({this.pass = false, this.evidence = ''});

  final bool pass;
  final String evidence;
}

class LogCheckOptions {
  const LogCheckOptions({
    this.conn,
    required this.sender,
    required this.leafUid,
    required this.expectedChain,
    this.timeout = const Duration(seconds: 3),
    this.pollInterval = const Duration(milliseconds: 100),
    this.live = false,
  });

  final grpc_api.ClientChannel? conn;
  final String sender;
  final String leafUid;
  final List<ChainHop> expectedChain;
  final Duration timeout;
  final Duration pollInterval;
  final bool live;
}

class EventCheckOptions {
  const EventCheckOptions({
    this.conn,
    this.eventType = observability.EventType.instanceReady,
    required this.leafUid,
    required this.expectedChain,
    this.timeout = const Duration(seconds: 3),
    this.pollInterval = const Duration(milliseconds: 100),
    this.live = false,
  });

  final grpc_api.ClientChannel? conn;
  final observability.EventType eventType;
  final String leafUid;
  final List<ChainHop> expectedChain;
  final Duration timeout;
  final Duration pollInterval;
  final bool live;
}

Future<CheckOutcome> checkRelayedLog(LogCheckOptions opts) async {
  final deadline = DateTime.now().add(opts.timeout);
  var last = const CheckOutcome();
  while (true) {
    try {
      final entries = await _readLogEntries(opts.conn);
      last = _matchRelayedLog(entries, opts);
      if (last.pass) {
        return last;
      }
    } on Object catch (error) {
      last = CheckOutcome(evidence: _compactEvidence('$error'));
    }
    if (DateTime.now().isAfter(deadline)) {
      return last;
    }
    await Future<void>.delayed(opts.pollInterval);
  }
}

Future<CheckOutcome> checkRelayedEvent(EventCheckOptions opts) async {
  final deadline = DateTime.now().add(opts.timeout);
  var last = const CheckOutcome();
  while (true) {
    try {
      final events = await _readEventEntries(opts.conn);
      last = _matchRelayedEvent(events, opts);
      if (last.pass) {
        return last;
      }
    } on Object catch (error) {
      last = CheckOutcome(evidence: _compactEvidence('$error'));
    }
    if (DateTime.now().isAfter(deadline)) {
      return last;
    }
    await Future<void>.delayed(opts.pollInterval);
  }
}

Future<List<observability.LogEntry>> _readLogEntries(
    grpc_api.ClientChannel? conn) async {
  if (conn == null) {
    final ring = observability.current().logRing;
    if (ring == null) {
      throw StateError('logs family is not enabled');
    }
    return ring.drain();
  }
  final client = obs_pb.HolonObservabilityClient(conn);
  final entries = await client
      .logs(obs_pb.LogsRequest(minLevel: obs_pb.LogLevel.INFO, follow: false))
      .toList()
      .timeout(const Duration(seconds: 2));
  return [
    for (final entry in entries)
      observability.LogEntry(
        timestamp: DateTime.now(),
        level: observability.Level.values.firstWhere(
          (candidate) => candidate.value == entry.level.value,
          orElse: () => observability.Level.unset,
        ),
        slug: entry.slug,
        instanceUid: entry.instanceUid,
        sessionId: entry.sessionId,
        rpcMethod: entry.rpcMethod,
        message: entry.message,
        fields: Map.unmodifiable(entry.fields),
        caller: entry.caller,
        chain: [
          for (final hop in entry.chain)
            observability.Hop(slug: hop.slug, instanceUid: hop.instanceUid),
        ],
      ),
  ];
}

Future<List<observability.Event>> _readEventEntries(
    grpc_api.ClientChannel? conn) async {
  if (conn == null) {
    final bus = observability.current().eventBus;
    if (bus == null) {
      throw StateError('events family is not enabled');
    }
    return bus.drain();
  }
  final client = obs_pb.HolonObservabilityClient(conn);
  final events = await client
      .events(obs_pb.EventsRequest(follow: false))
      .toList()
      .timeout(const Duration(seconds: 2));
  return [
    for (final event in events)
      observability.Event(
        timestamp: DateTime.now(),
        type: observability.EventType.values.firstWhere(
          (candidate) => candidate.value == event.type.value,
          orElse: () => observability.EventType.unspecified,
        ),
        slug: event.slug,
        instanceUid: event.instanceUid,
        sessionId: event.sessionId,
        payload: Map.unmodifiable(event.payload),
        chain: [
          for (final hop in event.chain)
            observability.Hop(slug: hop.slug, instanceUid: hop.instanceUid),
        ],
      ),
  ];
}

CheckOutcome _matchRelayedLog(
  List<observability.LogEntry> entries,
  LogCheckOptions opts,
) {
  for (final entry in entries) {
    if (entry.message != 'tick received') {
      continue;
    }
    if (entry.fields['sender'] != opts.sender ||
        entry.fields['responder_uid'] != opts.leafUid) {
      continue;
    }
    final chainEvidence = _compareChain(entry.chain, opts.expectedChain);
    if (chainEvidence.isNotEmpty) {
      return CheckOutcome(
        evidence: _compactEvidence('matching log bad chain: $chainEvidence'),
      );
    }
    return const CheckOutcome(pass: true);
  }
  return CheckOutcome(
    evidence: _compactEvidence(
      'no relayed tick log sender=${opts.sender} leaf_uid=${opts.leafUid} entries=${entries.length}',
    ),
  );
}

CheckOutcome _matchRelayedEvent(
  List<observability.Event> events,
  EventCheckOptions opts,
) {
  for (final event in events) {
    if (event.type != opts.eventType || event.instanceUid != opts.leafUid) {
      continue;
    }
    final chainEvidence = _compareChain(event.chain, opts.expectedChain);
    if (chainEvidence.isNotEmpty) {
      return CheckOutcome(
        evidence: _compactEvidence('matching event bad chain: $chainEvidence'),
      );
    }
    return const CheckOutcome(pass: true);
  }
  return CheckOutcome(
    evidence: _compactEvidence(
      'no relayed ${opts.eventType.protoName} event leaf_uid=${opts.leafUid} events=${events.length}',
    ),
  );
}

String _compareChain(List<ChainHop> got, List<ChainHop> want) {
  if (got.length != want.length) {
    return 'chain length ${got.length} want ${want.length}';
  }
  for (var i = 0; i < want.length; i++) {
    if (got[i].slug != want[i].slug ||
        got[i].instanceUid != want[i].instanceUid) {
      return 'hop $i=${got[i].slug}/${got[i].instanceUid} want ${want[i].slug}/${want[i].instanceUid}';
    }
  }
  return '';
}

String _compactEvidence(String value) {
  final compact =
      value.trim().split(RegExp(r'\s+')).where((s) => s.isNotEmpty).join(' ');
  if (compact.length <= 240) {
    return compact;
  }
  return '${compact.substring(0, 240)}...';
}
