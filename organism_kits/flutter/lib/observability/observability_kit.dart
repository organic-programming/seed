import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:holons/holons.dart' as holons;
import 'package:path/path.dart' as p;

import '../src/settings_store.dart';

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
    _entries.addAll(obs.logRing?.drain() ?? const []);
    _sub = obs.logRing?.watch().listen((entry) {
      _entries.add(entry);
      notifyListeners();
    });
    gate.addListener(notifyListeners);
  }

  final holons.Observability obs;
  final RuntimeGate gate;
  final List<holons.LogEntry> _entries = [];
  StreamSubscription<holons.LogEntry>? _sub;
  holons.Level minLevel = holons.Level.trace;
  String query = '';

  List<holons.LogEntry> get entries {
    if (!gate.familyEnabled(holons.Family.logs)) return const [];
    final q = query.trim().toLowerCase();
    return List.unmodifiable(
      _entries.where((entry) {
        if (entry.level.value < minLevel.value) return false;
        if (q.isEmpty) return true;
        return entry.message.toLowerCase().contains(q) ||
            entry.slug.toLowerCase().contains(q);
      }),
    );
  }

  void setMinLevel(holons.Level value) {
    minLevel = value;
    notifyListeners();
  }

  void setQuery(String value) {
    query = value;
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
    _events.addAll(obs.eventBus?.drain() ?? const []);
    _sub = obs.eventBus?.watch().listen((event) {
      _events.add(event);
      notifyListeners();
    });
    gate.addListener(notifyListeners);
  }

  final holons.Observability obs;
  final RuntimeGate gate;
  final List<holons.Event> _events = [];
  StreamSubscription<holons.Event>? _sub;

  List<holons.Event> get events => gate.familyEnabled(holons.Family.events)
      ? List.unmodifiable(_events)
      : const [];

  @override
  void dispose() {
    gate.removeListener(notifyListeners);
    _sub?.cancel();
    super.dispose();
  }
}

class RelayController extends ChangeNotifier {
  RelayController(this.gate) {
    gate.addListener(notifyListeners);
  }

  final RuntimeGate gate;

  List<ObservabilityMemberRef> get activeMembers => gate.members
      .where((member) => gate.memberEnabled(member.uid))
      .toList(growable: false);

  @override
  void dispose() {
    gate.removeListener(notifyListeners);
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
    final logs = kit.obs.logRing?.drain() ?? const <holons.LogEntry>[];
    final events = kit.obs.eventBus?.drain() ?? const <holons.Event>[];
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
  }) {
    final families = declaredFamilies.map((family) => family.name).join(',');
    final env = Map<String, String>.from(Platform.environment)
      ..['OP_OBS'] = families;
    final obs = holons.fromEnv(
      holons.Config(
        slug: slug,
        instanceUid: 'kit-${DateTime.now().microsecondsSinceEpoch}',
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
      relay: RelayController(gate),
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

String _logJson(holons.LogEntry entry) => jsonEncode({
  'ts': entry.timestamp.toUtc().toIso8601String(),
  'level': entry.level.name,
  'slug': entry.slug,
  'instance_uid': entry.instanceUid,
  'message': entry.message,
  'fields': entry.fields,
});

String _eventJson(holons.Event event) => jsonEncode({
  'ts': event.timestamp.toUtc().toIso8601String(),
  'type': event.type.name,
  'slug': event.slug,
  'instance_uid': event.instanceUid,
  'payload': event.payload,
});
