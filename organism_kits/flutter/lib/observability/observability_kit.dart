import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/observability.pb.dart' as obs_pb;
import 'package:holons/holons.dart' as holons;
import 'package:holons/holons.dart' show SettingsStore;
import 'package:path/path.dart' as p;

const _masterKey = 'observability.master.enabled';
const _logsKey = 'observability.family.logs';
const _metricsKey = 'observability.family.metrics';
const _eventsKey = 'observability.family.events';
const _promKey = 'observability.family.prom';
const _promAddrKey = 'observability.prom.addr';
const _memberPrefix = 'observability.member.';

enum GateOverride { defaultValue, on, off }

class ObservabilityMemberRef {
  const ObservabilityMemberRef({
    required this.slug,
    required this.uid,
    this.address = '',
  });

  final String slug;
  final String uid;
  final String address;
}

enum AnyValueBranch { stringValue, boolValue, intValue, doubleValue, notSet }

class ObservabilityAnyValue {
  const ObservabilityAnyValue._({
    this.stringValue = '',
    this.boolValue = false,
    this.intValue = 0,
    this.doubleValue = 0,
    required this.branch,
  });

  const ObservabilityAnyValue.string(String value)
    : this._(stringValue: value, branch: AnyValueBranch.stringValue);

  const ObservabilityAnyValue.bool(bool value)
    : this._(boolValue: value, branch: AnyValueBranch.boolValue);

  const ObservabilityAnyValue.int(int value)
    : this._(intValue: value, branch: AnyValueBranch.intValue);

  const ObservabilityAnyValue.double(double value)
    : this._(doubleValue: value, branch: AnyValueBranch.doubleValue);

  const ObservabilityAnyValue.notSet() : this._(branch: AnyValueBranch.notSet);

  factory ObservabilityAnyValue.fromProto(obs_pb.AnyValue value) {
    return switch (value.whichValue()) {
      obs_pb.AnyValue_Value.stringValue => ObservabilityAnyValue.string(
        value.stringValue,
      ),
      obs_pb.AnyValue_Value.boolValue => ObservabilityAnyValue.bool(
        value.boolValue,
      ),
      obs_pb.AnyValue_Value.intValue => ObservabilityAnyValue.int(
        value.intValue.toInt(),
      ),
      obs_pb.AnyValue_Value.doubleValue => ObservabilityAnyValue.double(
        value.doubleValue,
      ),
      obs_pb.AnyValue_Value.notSet => const ObservabilityAnyValue.notSet(),
    };
  }

  final String stringValue;
  final bool boolValue;
  final int intValue;
  final double doubleValue;
  final AnyValueBranch branch;
}

class ObservabilityKeyValue {
  const ObservabilityKeyValue({required this.key, required this.value});

  final String key;
  final ObservabilityAnyValue value;
}

class ObservabilityLogRecord {
  const ObservabilityLogRecord({
    required this.timestamp,
    required this.level,
    required this.slug,
    required this.instanceUid,
    required this.body,
    this.severityText = '',
    this.loggerName = '',
    this.sessionId = '',
    this.rpcMethod = '',
    this.attributes = const [],
    this.caller = '',
    this.chain = const [],
    this.eventName = '',
    this.private = false,
  });

  factory ObservabilityLogRecord.fromLogRecord(
    obs_pb.LogRecord record, {
    bool private = false,
  }) {
    return ObservabilityLogRecord(
      timestamp: _dateTimeFromUnixNano(record.timeUnixNano.toInt()),
      level: _levelFromSeverity(record.severityNumber),
      severityText: record.severityText,
      slug: _attributeString(record.attributes, 'holons.slug'),
      instanceUid: _attributeString(record.attributes, 'holons.instance_uid'),
      sessionId: _attributeString(record.attributes, 'holons.session_id'),
      rpcMethod: _attributeString(record.attributes, 'rpc.method'),
      body: record.hasBody()
          ? ObservabilityAnyValue.fromProto(record.body)
          : const ObservabilityAnyValue.notSet(),
      attributes: [
        for (final attribute in record.attributes)
          if (!_wellKnownAttributeKeys.contains(attribute.key))
            ObservabilityKeyValue(
              key: attribute.key,
              value: attribute.hasValue()
                  ? ObservabilityAnyValue.fromProto(attribute.value)
                  : const ObservabilityAnyValue.notSet(),
            ),
      ],
      caller: _attributeString(record.attributes, 'code.caller'),
      chain: [
        for (final hop in record.chain) holons.Hop(slug: hop, instanceUid: ''),
      ],
      eventName: record.eventName,
      private: private,
    );
  }

  factory ObservabilityLogRecord.fromLegacyLogEntry(dynamic entry) {
    return ObservabilityLogRecord(
      timestamp: entry.timestamp,
      level: entry.level,
      severityText: _levelLabel(entry.level),
      loggerName: entry.loggerName,
      slug: entry.slug,
      instanceUid: entry.instanceUid,
      sessionId: entry.sessionId,
      rpcMethod: entry.rpcMethod,
      body: ObservabilityAnyValue.string(entry.message),
      attributes: [
        for (final item in entry.fields.entries)
          ObservabilityKeyValue(
            key: item.key,
            value: ObservabilityAnyValue.string(item.value),
          ),
      ],
      caller: entry.caller,
      chain: entry.chain,
      private: entry.private,
    );
  }

  factory ObservabilityLogRecord.fromLegacyEvent(dynamic event) {
    return ObservabilityLogRecord(
      timestamp: event.timestamp,
      level: holons.Level.info,
      severityText: _levelLabel(holons.Level.info),
      slug: event.slug,
      instanceUid: event.instanceUid,
      sessionId: event.sessionId,
      body: ObservabilityAnyValue.string(event.type.name),
      attributes: [
        for (final item in event.payload.entries)
          ObservabilityKeyValue(
            key: item.key,
            value: ObservabilityAnyValue.string(item.value),
          ),
      ],
      chain: event.chain,
      eventName: event.type.name,
      private: event.private,
    );
  }

  String get message => switch (body.branch) {
    AnyValueBranch.stringValue => body.stringValue,
    AnyValueBranch.boolValue => body.boolValue.toString(),
    AnyValueBranch.intValue => body.intValue.toString(),
    AnyValueBranch.doubleValue => body.doubleValue.toString(),
    AnyValueBranch.notSet => '',
  };

  final DateTime timestamp;
  final holons.Level level;
  final String severityText;
  final String loggerName;
  final String slug;
  final String instanceUid;
  final String sessionId;
  final String rpcMethod;
  final ObservabilityAnyValue body;
  final List<ObservabilityKeyValue> attributes;
  final String caller;
  final List<holons.Hop> chain;
  final String eventName;
  final bool private;
}

const _wellKnownAttributeKeys = {
  'holons.slug',
  'service.name',
  'holons.instance_uid',
  'service.instance.id',
  'holons.session_id',
  'rpc.method',
  'code.caller',
};

ObservabilityLogRecord _logRecordFromObject(Object entry) {
  if (entry is obs_pb.LogRecord) {
    return ObservabilityLogRecord.fromLogRecord(entry);
  }
  if (entry is holons.LogRecord) {
    return ObservabilityLogRecord.fromLogRecord(
      entry.record,
      private: entry.private,
    );
  }
  return ObservabilityLogRecord.fromLegacyLogEntry(entry);
}

ObservabilityLogRecord _eventRecordFromObject(Object event) {
  if (event is obs_pb.LogRecord) {
    return ObservabilityLogRecord.fromLogRecord(event);
  }
  if (event is holons.LogRecord) {
    return ObservabilityLogRecord.fromLogRecord(
      event.record,
      private: event.private,
    );
  }
  return ObservabilityLogRecord.fromLegacyEvent(event);
}

DateTime _dateTimeFromUnixNano(int ns) {
  return DateTime.fromMicrosecondsSinceEpoch(ns ~/ 1000, isUtc: true);
}

holons.Level _levelFromSeverity(obs_pb.SeverityNumber severity) {
  return switch (severity) {
    obs_pb.SeverityNumber.SEVERITY_NUMBER_TRACE => holons.Level.trace,
    obs_pb.SeverityNumber.SEVERITY_NUMBER_DEBUG => holons.Level.debug,
    obs_pb.SeverityNumber.SEVERITY_NUMBER_INFO => holons.Level.info,
    obs_pb.SeverityNumber.SEVERITY_NUMBER_WARN => holons.Level.warn,
    obs_pb.SeverityNumber.SEVERITY_NUMBER_ERROR => holons.Level.error,
    obs_pb.SeverityNumber.SEVERITY_NUMBER_FATAL => holons.Level.fatal,
    _ => holons.Level.unset,
  };
}

String _levelLabel(holons.Level level) {
  if (level == holons.Level.trace) return 'TRACE';
  if (level == holons.Level.debug) return 'DEBUG';
  if (level == holons.Level.info) return 'INFO';
  if (level == holons.Level.warn) return 'WARN';
  if (level == holons.Level.error) return 'ERROR';
  if (level == holons.Level.fatal) return 'FATAL';
  return 'UNSPECIFIED';
}

String _attributeString(Iterable<obs_pb.KeyValue> attributes, String key) {
  for (final attribute in attributes) {
    if (attribute.key != key || !attribute.hasValue()) continue;
    final value = ObservabilityAnyValue.fromProto(attribute.value);
    return switch (value.branch) {
      AnyValueBranch.stringValue => value.stringValue,
      AnyValueBranch.boolValue => value.boolValue.toString(),
      AnyValueBranch.intValue => value.intValue.toString(),
      AnyValueBranch.doubleValue => value.doubleValue.toString(),
      AnyValueBranch.notSet => '',
    };
  }
  return '';
}

class RuntimeGate extends ChangeNotifier {
  RuntimeGate({
    required this.settings,
    Iterable<ObservabilityMemberRef> members = const [],
  }) : members = List.unmodifiable(members) {
    masterEnabled = settings.readBool(_masterKey, defaultValue: true);
    logsEnabled = settings.readBool(_logsKey, defaultValue: true);
    metricsEnabled = settings.readBool(_metricsKey, defaultValue: true);
    eventsEnabled = settings.readBool(_eventsKey, defaultValue: true);
    promEnabled = settings.readBool(_promKey, defaultValue: false);
    promAddress = settings.readString(_promAddrKey);
    for (final member in this.members) {
      final raw = settings.readString('$_memberPrefix${member.uid}');
      _memberOverrides[member.uid] = switch (raw) {
        'on' => GateOverride.on,
        'off' => GateOverride.off,
        _ => GateOverride.defaultValue,
      };
    }
  }

  final SettingsStore settings;
  final List<ObservabilityMemberRef> members;
  final Map<String, GateOverride> _memberOverrides = {};

  late bool masterEnabled;
  late bool logsEnabled;
  late bool metricsEnabled;
  late bool eventsEnabled;
  late bool promEnabled;
  late String promAddress;

  bool familyEnabled(holons.Family family) {
    if (!masterEnabled) return false;
    return switch (family) {
      holons.Family.logs => logsEnabled,
      holons.Family.metrics => metricsEnabled,
      holons.Family.events => eventsEnabled,
      holons.Family.prom => promEnabled,
      holons.Family.otel => false,
    };
  }

  GateOverride memberOverride(String uid) =>
      _memberOverrides[uid] ?? GateOverride.defaultValue;

  bool memberEnabled(String uid) {
    if (!masterEnabled) return false;
    return switch (memberOverride(uid)) {
      GateOverride.defaultValue => true,
      GateOverride.on => true,
      GateOverride.off => false,
    };
  }

  Future<void> setMaster(bool value) async {
    masterEnabled = value;
    await settings.writeBool(_masterKey, value);
    notifyListeners();
  }

  Future<void> setFamily(holons.Family family, bool value) async {
    switch (family) {
      case holons.Family.logs:
        logsEnabled = value;
        await settings.writeBool(_logsKey, value);
      case holons.Family.metrics:
        metricsEnabled = value;
        await settings.writeBool(_metricsKey, value);
      case holons.Family.events:
        eventsEnabled = value;
        await settings.writeBool(_eventsKey, value);
      case holons.Family.prom:
        promEnabled = value;
        await settings.writeBool(_promKey, value);
      case holons.Family.otel:
        return;
    }
    notifyListeners();
  }

  Future<void> setPromAddress(String value) async {
    promAddress = value;
    await settings.writeString(_promAddrKey, value);
    notifyListeners();
  }

  Future<void> setMemberOverride(String uid, GateOverride value) async {
    _memberOverrides[uid] = value;
    await settings.writeString('$_memberPrefix$uid', switch (value) {
      GateOverride.defaultValue => '',
      GateOverride.on => 'on',
      GateOverride.off => 'off',
    });
    notifyListeners();
  }
}

class LogConsoleController extends ChangeNotifier {
  LogConsoleController(this.obs, this.gate) {
    _entries.addAll(
      (obs.logRing?.drain() ?? const <Object>[]).map(_logRecordFromObject),
    );
    _sub = obs.logRing?.watch().listen((entry) {
      _entries.add(_logRecordFromObject(entry));
      notifyListeners();
    });
    gate.addListener(notifyListeners);
  }

  final holons.Observability obs;
  final RuntimeGate gate;
  final List<ObservabilityLogRecord> _entries = [];
  StreamSubscription<dynamic>? _sub;
  holons.Level minLevel = holons.Level.trace;
  String query = '';

  List<ObservabilityLogRecord> get entries {
    if (!gate.familyEnabled(holons.Family.logs)) return const [];
    final q = query.trim().toLowerCase();
    final filtered = _entries
        .where((entry) {
          if (entry.level.value < minLevel.value) return false;
          if (q.isEmpty) return true;
          return entry.message.toLowerCase().contains(q) ||
              entry.slug.toLowerCase().contains(q);
        })
        .toList(growable: false);
    filtered.sort((a, b) => a.timestamp.compareTo(b.timestamp));
    return List.unmodifiable(filtered);
  }

  void setMinLevel(holons.Level value) {
    minLevel = value;
    notifyListeners();
  }

  void setQuery(String value) {
    query = value;
    notifyListeners();
  }

  @visibleForTesting
  void addRecord(ObservabilityLogRecord record) {
    _entries.add(record);
    notifyListeners();
  }

  @override
  void dispose() {
    gate.removeListener(notifyListeners);
    _sub?.cancel();
    super.dispose();
  }
}

class MetricSnapshot {
  MetricSnapshot({
    required this.capturedAt,
    required this.counters,
    required this.gauges,
    required this.histograms,
  });

  final DateTime capturedAt;
  final List<holons.Counter> counters;
  final List<holons.Gauge> gauges;
  final List<holons.Histogram> histograms;
}

class MetricsController extends ChangeNotifier {
  MetricsController(
    this.obs,
    this.gate, {
    this.interval = const Duration(seconds: 1),
  }) {
    _capture();
    _timer = Timer.periodic(interval, (_) => _capture());
    gate.addListener(notifyListeners);
  }

  final holons.Observability obs;
  final RuntimeGate gate;
  final Duration interval;
  final List<MetricSnapshot> history = [];
  Timer? _timer;

  MetricSnapshot? get latest => history.isEmpty ? null : history.last;

  void refresh() => _capture();

  void _capture() {
    final registry = obs.registry;
    if (registry == null) return;
    history.add(
      MetricSnapshot(
        capturedAt: DateTime.now(),
        counters: registry.listCounters(),
        gauges: registry.listGauges(),
        histograms: registry.listHistograms(),
      ),
    );
    if (history.length > 30) history.removeRange(0, history.length - 30);
    notifyListeners();
  }

  @override
  void dispose() {
    gate.removeListener(notifyListeners);
    _timer?.cancel();
    super.dispose();
  }
}

class EventsController extends ChangeNotifier {
  EventsController(this.obs, this.gate) {
    _events.addAll(
      (obs.eventBus?.drain() ?? const <Object>[]).map(_eventRecordFromObject),
    );
    _sub = obs.eventBus?.watch().listen((event) {
      _events.add(_eventRecordFromObject(event));
      notifyListeners();
    });
    gate.addListener(notifyListeners);
  }

  final holons.Observability obs;
  final RuntimeGate gate;
  final List<ObservabilityLogRecord> _events = [];
  StreamSubscription<dynamic>? _sub;

  List<ObservabilityLogRecord> get events =>
      gate.familyEnabled(holons.Family.events)
      ? List.unmodifiable(_events)
      : const [];

  @visibleForTesting
  void addRecord(ObservabilityLogRecord record) {
    _events.add(record);
    notifyListeners();
  }

  @override
  void dispose() {
    gate.removeListener(notifyListeners);
    _sub?.cancel();
    super.dispose();
  }
}

typedef RelayChannelOpener =
    Future<ClientChannel> Function(ObservabilityMemberRef member);

typedef MemberRelayFactory =
    RelaySession Function({
      required String childSlug,
      required String childUid,
      required ClientChannel channel,
      required holons.Observability observability,
    });

abstract interface class RelaySession {
  Future<void> start();
  Future<void> stop();
  bool get isRunning;
}

class _MemberRelaySession implements RelaySession {
  _MemberRelaySession(this._relay);

  final holons.MemberRelay _relay;

  @override
  bool get isRunning => _relay.isRunning;

  @override
  Future<void> start() => _relay.start();

  @override
  Future<void> stop() => _relay.stop();
}

RelaySession _defaultMemberRelayFactory({
  required String childSlug,
  required String childUid,
  required ClientChannel channel,
  required holons.Observability observability,
}) {
  return _MemberRelaySession(
    holons.MemberRelay(
      childSlug: childSlug,
      childUid: childUid,
      channel: channel,
      observability: observability,
    ),
  );
}

class RelayController extends ChangeNotifier {
  RelayController(
    this.gate,
    this.obs, {
    RelayChannelOpener? channelOpener,
    MemberRelayFactory memberRelayFactory = _defaultMemberRelayFactory,
  }) : _channelOpener = channelOpener,
       _memberRelayFactory = memberRelayFactory {
    gate.addListener(_sync);
  }

  final RuntimeGate gate;
  final holons.Observability obs;
  final RelayChannelOpener? _channelOpener;
  final MemberRelayFactory _memberRelayFactory;
  final Map<String, RelaySession> _relays = {};
  final Set<String> _starting = {};
  String _activeMemberUid = '';
  bool _disposed = false;

  List<ObservabilityMemberRef> get activeMembers => gate.members
      .where((member) => _relays.containsKey(member.uid))
      .toList(growable: false);

  int get runningRelayCount => _relays.length;

  Future<void> activateMember(String uid) async {
    if (_disposed) return;
    _activeMemberUid = uid.trim();
    await _sync();
  }

  Future<void> _sync() async {
    if (_disposed) return;
    final opener = _channelOpener;
    if (opener == null) {
      notifyListeners();
      return;
    }

    final activeUid = _activeMemberUid;
    final stopFutures = <Future<void>>[];
    for (final uid in List<String>.from(_relays.keys)) {
      if (uid != activeUid || !gate.memberEnabled(uid)) {
        stopFutures.add(_stopRelay(uid));
      }
    }

    final member = _memberByUid(activeUid);
    if (member != null &&
        _isRelayTarget(member) &&
        !_relays.containsKey(member.uid) &&
        !_starting.contains(member.uid)) {
      _starting.add(member.uid);
      unawaited(_startRelay(member));
    }

    if (stopFutures.isNotEmpty) {
      await Future.wait(stopFutures);
      notifyListeners();
    }
  }

  Future<void> _startRelay(ObservabilityMemberRef member) async {
    final opener = _channelOpener;
    if (opener == null || !_isRelayTarget(member)) {
      _starting.remove(member.uid);
      return;
    }
    try {
      final channel = await opener(member);
      if (!_isRelayTarget(member)) return;
      final relay = _memberRelayFactory(
        childSlug: member.slug,
        childUid: member.uid,
        channel: channel,
        observability: obs,
      );
      await relay.start();
      if (!_isRelayTarget(member)) {
        await relay.stop();
        return;
      }
      if (relay.isRunning) {
        _relays[member.uid] = relay;
        notifyListeners();
      }
    } on Object catch (error) {
      obs
          .logger('relay-controller')
          .warn(
            'member relay start failed',
            fields: {'slug': member.slug, 'uid': member.uid, 'error': error},
          );
    } finally {
      _starting.remove(member.uid);
    }
  }

  ObservabilityMemberRef? _memberByUid(String uid) {
    if (uid.isEmpty) return null;
    for (final member in gate.members) {
      if (member.uid == uid) {
        return member;
      }
    }
    return null;
  }

  bool _isRelayTarget(ObservabilityMemberRef member) {
    return !_disposed &&
        _activeMemberUid == member.uid &&
        gate.memberEnabled(member.uid);
  }

  Future<void> _stopRelay(String uid) async {
    final relay = _relays.remove(uid);
    if (relay == null) return;
    await relay.stop();
  }

  @override
  void dispose() {
    if (_disposed) return;
    _disposed = true;
    gate.removeListener(_sync);
    final relays = List<RelaySession>.from(_relays.values);
    _relays.clear();
    _starting.clear();
    unawaited(Future.wait<void>([for (final relay in relays) relay.stop()]));
    super.dispose();
  }
}

class PrometheusController extends ChangeNotifier {
  PrometheusController(this.obs, this.gate) {
    gate.addListener(_sync);
    _sync();
  }

  final holons.Observability obs;
  final RuntimeGate gate;
  HttpServer? _server;
  String boundAddress = '';

  Future<void> _sync() async {
    if (gate.familyEnabled(holons.Family.prom)) {
      await start();
    } else {
      await stop();
    }
  }

  Future<void> start() async {
    if (_server != null) return;
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    _server = server;
    boundAddress = 'http://127.0.0.1:${server.port}/metrics';
    await gate.setPromAddress(boundAddress);
    notifyListeners();
    unawaited(_serve(server));
  }

  Future<void> _serve(HttpServer server) async {
    await for (final request in server) {
      if (request.uri.path != '/metrics') {
        request.response.statusCode = HttpStatus.notFound;
        await request.response.close();
        continue;
      }
      request.response.headers.contentType = ContentType(
        'text',
        'plain',
        charset: 'utf-8',
      );
      request.response.write(prometheusText(obs));
      await request.response.close();
    }
  }

  Future<void> stop() async {
    final server = _server;
    _server = null;
    boundAddress = '';
    if (server != null) await server.close(force: true);
    notifyListeners();
  }

  @override
  void dispose() {
    gate.removeListener(_sync);
    unawaited(stop());
    super.dispose();
  }
}

class ExportController {
  ExportController(this.kit);

  final ObservabilityKit kit;

  Future<Directory> exportTo(Directory parent) async {
    final timestamp = DateTime.now().toUtc().toIso8601String().replaceAll(
      ':',
      '',
    );
    final dir = Directory(
      p.join(parent.path, 'observability-${kit.slug}-$timestamp'),
    );
    await dir.create(recursive: true);
    final logs = kit.obs.logRing?.drain() ?? const <Object>[];
    final events = kit.obs.eventBus?.drain() ?? const <Object>[];
    await File(
      p.join(dir.path, 'logs.jsonl'),
    ).writeAsString(logs.map(_logJson).join('\n') + '\n');
    await File(
      p.join(dir.path, 'events.jsonl'),
    ).writeAsString(events.map(_eventJson).join('\n') + '\n');
    await File(
      p.join(dir.path, 'metrics.prom'),
    ).writeAsString(prometheusText(kit.obs));
    await File(p.join(dir.path, 'metadata.json')).writeAsString(
      '${const JsonEncoder.withIndent('  ').convert({
        'slug': kit.slug,
        'instance_uid': kit.obs.cfg.instanceUid,
        'exported_at': DateTime.now().toUtc().toIso8601String(),
        'members': [
          for (final member in kit.gate.members) {'slug': member.slug, 'uid': member.uid, 'address': member.address},
        ],
      })}\n',
    );
    return dir;
  }
}

class ObservabilityKit extends ChangeNotifier {
  ObservabilityKit._({
    required this.slug,
    required this.obs,
    required this.gate,
    required this.logs,
    required this.metrics,
    required this.events,
    required this.relay,
    required this.prometheus,
  });

  final String slug;
  final holons.Observability obs;
  final RuntimeGate gate;
  final LogConsoleController logs;
  final MetricsController metrics;
  final EventsController events;
  final RelayController relay;
  final PrometheusController prometheus;
  late final ExportController export;
  bool _disposed = false;

  static ObservabilityKit standalone({
    required String slug,
    required Iterable<holons.Family> declaredFamilies,
    required SettingsStore settings,
    Iterable<ObservabilityMemberRef> bundledHolons = const [],
    Map<String, String>? environment,
    RelayChannelOpener? relayChannelOpener,
    MemberRelayFactory memberRelayFactory = _defaultMemberRelayFactory,
  }) {
    final families = declaredFamilies.map((family) => family.name).join(',');
    final baseEnv = environment ?? Platform.environment;
    final env = Map<String, String>.from(baseEnv)..['OP_OBS'] = families;
    final launchedUid = (baseEnv['OP_INSTANCE_UID'] ?? '').trim();
    final obs = holons.fromEnv(
      holons.Config(
        slug: slug,
        instanceUid: launchedUid.isEmpty
            ? 'kit-${DateTime.now().microsecondsSinceEpoch}'
            : launchedUid,
      ),
      env,
    );
    final gate = RuntimeGate(settings: settings, members: bundledHolons);
    late final ObservabilityKit kit;
    kit = ObservabilityKit._(
      slug: slug,
      obs: obs,
      gate: gate,
      logs: LogConsoleController(obs, gate),
      metrics: MetricsController(obs, gate),
      events: EventsController(obs, gate),
      relay: RelayController(
        gate,
        obs,
        channelOpener: relayChannelOpener,
        memberRelayFactory: memberRelayFactory,
      ),
      prometheus: PrometheusController(obs, gate),
    );
    kit.export = ExportController(kit);
    return kit;
  }

  @override
  void dispose() {
    if (_disposed) return;
    _disposed = true;
    logs.dispose();
    metrics.dispose();
    events.dispose();
    relay.dispose();
    prometheus.dispose();
    obs.close();
    super.dispose();
  }
}

String prometheusText(holons.Observability obs) {
  final registry = obs.registry;
  if (registry == null) return '';
  final lines = <String>[];
  for (final counter in registry.listCounters()) {
    lines.add('# TYPE ${counter.name} counter');
    lines.add('${counter.name}${_labels(counter.labels)} ${counter.value()}');
  }
  for (final gauge in registry.listGauges()) {
    lines.add('# TYPE ${gauge.name} gauge');
    lines.add('${gauge.name}${_labels(gauge.labels)} ${gauge.value()}');
  }
  for (final histogram in registry.listHistograms()) {
    final snap = histogram.snapshot();
    lines.add('# TYPE ${histogram.name} histogram');
    for (var i = 0; i < snap.bounds.length; i++) {
      lines.add(
        '${histogram.name}_bucket${_labels({...histogram.labels, 'le': snap.bounds[i].toString()})} ${snap.counts[i]}',
      );
    }
    lines.add(
      '${histogram.name}_count${_labels(histogram.labels)} ${snap.total}',
    );
    lines.add('${histogram.name}_sum${_labels(histogram.labels)} ${snap.sum}');
  }
  return '${lines.join('\n')}\n';
}

String _labels(Map<String, String> labels) {
  if (labels.isEmpty) return '';
  final parts = labels.entries
      .map((entry) => '${entry.key}="${entry.value.replaceAll('"', r'\"')}"')
      .join(',');
  return '{$parts}';
}

String _logJson(Object entry) {
  final record = _logRecordFromObject(entry);
  return jsonEncode({
    'ts': record.timestamp.toUtc().toIso8601String(),
    'level': record.level.name,
    'slug': record.slug,
    'instance_uid': record.instanceUid,
    'message': record.message,
    'fields': {
      for (final attribute in record.attributes)
        attribute.key: _jsonValue(attribute.value),
    },
  });
}

String _eventJson(Object event) {
  final record = _eventRecordFromObject(event);
  return jsonEncode({
    'ts': record.timestamp.toUtc().toIso8601String(),
    'event_name': record.eventName,
    'slug': record.slug,
    'instance_uid': record.instanceUid,
    'payload': {
      for (final attribute in record.attributes)
        attribute.key: _jsonValue(attribute.value),
    },
  });
}

Object _jsonValue(ObservabilityAnyValue value) {
  return switch (value.branch) {
    AnyValueBranch.stringValue => value.stringValue,
    AnyValueBranch.boolValue => value.boolValue,
    AnyValueBranch.intValue => value.intValue,
    AnyValueBranch.doubleValue => value.doubleValue,
    AnyValueBranch.notSet => '',
  };
}
