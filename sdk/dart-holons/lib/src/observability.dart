/// Dart reference implementation of the cross-SDK observability layer.
///
/// Mirrors sdk/go-holons/pkg/observability (the spec reference). Same
/// activation model (OP_OBS env + zero cost when disabled), same public
/// surface (Logger / Counter / Gauge / Histogram / EventBus / chain
/// helpers), same proto types (holons.v1.HolonObservability). See
/// OBSERVABILITY.md.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

// --- Families ---------------------------------------------------------------

enum Family {
  logs,
  metrics,
  events,
  prom,
  otel, // reserved v2
}

const _v1Tokens = {'logs', 'metrics', 'events', 'prom', 'all'};

class InvalidTokenError extends Error {
  final String token;
  final String reason;
  InvalidTokenError(this.token, this.reason);
  @override
  String toString() => 'OP_OBS: $reason: $token';
}

Set<Family> parseOpObs(String raw) {
  final out = <Family>{};
  if (raw.trim().isEmpty) return out;
  for (final part in raw.split(',')) {
    final tok = part.trim();
    if (tok.isEmpty) continue;
    if (tok == 'otel') continue;
    if (!_v1Tokens.contains(tok)) continue;
    if (tok == 'all') {
      out.addAll({Family.logs, Family.metrics, Family.events, Family.prom});
    } else {
      out.add(Family.values.firstWhere((f) => f.name == tok));
    }
  }
  return out;
}

void checkEnv([Map<String, String>? env]) {
  env ??= Platform.environment;
  final raw = (env['OP_OBS'] ?? '').trim();
  if (raw.isEmpty) return;
  for (final part in raw.split(',')) {
    final tok = part.trim();
    if (tok.isEmpty) continue;
    if (tok == 'otel') {
      throw InvalidTokenError(tok, 'otel export is reserved for v2; not implemented in v1');
    }
    if (!_v1Tokens.contains(tok)) {
      throw InvalidTokenError(tok, 'unknown OP_OBS token');
    }
  }
}

// --- Levels -----------------------------------------------------------------

enum Level {
  unset(0),
  trace(1),
  debug(2),
  info(3),
  warn(4),
  error(5),
  fatal(6);

  final int value;
  const Level(this.value);
}

Level parseLevel(String s) {
  final u = s.trim().toUpperCase();
  return {
    'TRACE': Level.trace,
    'DEBUG': Level.debug,
    'INFO': Level.info,
    'WARN': Level.warn,
    'WARNING': Level.warn,
    'ERROR': Level.error,
    'FATAL': Level.fatal,
  }[u] ?? Level.info;
}

// --- Event types ------------------------------------------------------------

enum EventType {
  unspecified(0, 'UNSPECIFIED'),
  instanceSpawned(1, 'INSTANCE_SPAWNED'),
  instanceReady(2, 'INSTANCE_READY'),
  instanceExited(3, 'INSTANCE_EXITED'),
  instanceCrashed(4, 'INSTANCE_CRASHED'),
  sessionStarted(5, 'SESSION_STARTED'),
  sessionEnded(6, 'SESSION_ENDED'),
  handlerPanic(7, 'HANDLER_PANIC'),
  configReloaded(8, 'CONFIG_RELOADED');

  final int value;
  final String protoName;
  const EventType(this.value, this.protoName);
}

// --- Chain helpers ----------------------------------------------------------

class Hop {
  final String slug;
  final String instanceUid;
  const Hop({required this.slug, required this.instanceUid});

  Map<String, String> toJson() => {'slug': slug, 'instance_uid': instanceUid};
}

List<Hop> appendDirectChild(List<Hop> src, String childSlug, String childUid) {
  return [...src, Hop(slug: childSlug, instanceUid: childUid)];
}

List<Hop> enrichForMultilog(List<Hop> wire, String streamSourceSlug, String streamSourceUid) {
  return appendDirectChild(wire, streamSourceSlug, streamSourceUid);
}

// --- Log entry --------------------------------------------------------------

class LogEntry {
  final DateTime timestamp;
  final Level level;
  final String slug;
  final String instanceUid;
  final String sessionId;
  final String rpcMethod;
  final String message;
  final Map<String, String> fields;
  final String caller;
  final List<Hop> chain;

  LogEntry({
    required this.timestamp,
    required this.level,
    required this.slug,
    required this.instanceUid,
    this.sessionId = '',
    this.rpcMethod = '',
    required this.message,
    this.fields = const {},
    this.caller = '',
    this.chain = const [],
  });
}

// --- Ring buffer + event bus ------------------------------------------------

class LogRing {
  final int capacity;
  final List<LogEntry> _buf = [];
  final _subs = <StreamController<LogEntry>>[];
  LogRing([this.capacity = 1024]);

  void push(LogEntry e) {
    _buf.add(e);
    if (_buf.length > capacity) _buf.removeRange(0, _buf.length - capacity);
    for (final s in List.of(_subs)) {
      if (!s.isClosed) s.add(e);
    }
  }

  List<LogEntry> drain() => List.unmodifiable(_buf);
  List<LogEntry> drainSince(DateTime cutoff) =>
      List.unmodifiable(_buf.where((e) => !e.timestamp.isBefore(cutoff)));

  Stream<LogEntry> watch() {
    final c = StreamController<LogEntry>.broadcast();
    _subs.add(c);
    c.onCancel = () {
      _subs.remove(c);
      c.close();
    };
    return c.stream;
  }

  int get length => _buf.length;
}

class Event {
  final DateTime timestamp;
  final EventType type;
  final String slug;
  final String instanceUid;
  final String sessionId;
  final Map<String, String> payload;
  final List<Hop> chain;

  Event({
    required this.timestamp,
    required this.type,
    required this.slug,
    required this.instanceUid,
    this.sessionId = '',
    this.payload = const {},
    this.chain = const [],
  });
}

class EventBus {
  final int capacity;
  final List<Event> _buf = [];
  final _subs = <StreamController<Event>>[];
  bool _closed = false;
  EventBus([this.capacity = 256]);

  void emit(Event e) {
    if (_closed) return;
    _buf.add(e);
    if (_buf.length > capacity) _buf.removeRange(0, _buf.length - capacity);
    for (final s in List.of(_subs)) {
      if (!s.isClosed) s.add(e);
    }
  }

  List<Event> drain() => List.unmodifiable(_buf);
  List<Event> drainSince(DateTime cutoff) =>
      List.unmodifiable(_buf.where((e) => !e.timestamp.isBefore(cutoff)));

  Stream<Event> watch() {
    final c = StreamController<Event>.broadcast();
    _subs.add(c);
    c.onCancel = () {
      _subs.remove(c);
      c.close();
    };
    return c.stream;
  }

  void close() {
    _closed = true;
    for (final s in _subs) s.close();
    _subs.clear();
  }
}

// --- Metrics ----------------------------------------------------------------

class Counter {
  final String name;
  final String help;
  final Map<String, String> labels;
  int _v = 0;
  Counter({required this.name, this.help = '', Map<String, String>? labels})
      : labels = Map.unmodifiable(labels ?? {});
  void inc([int n = 1]) {
    if (n < 0) return;
    _v += n;
  }
  void add(int n) => inc(n);
  int value() => _v;
}

class Gauge {
  final String name;
  final String help;
  final Map<String, String> labels;
  double _v = 0.0;
  Gauge({required this.name, this.help = '', Map<String, String>? labels})
      : labels = Map.unmodifiable(labels ?? {});
  void set(double v) => _v = v;
  void add(double d) => _v += d;
  double value() => _v;
}

class HistogramSnapshot {
  final List<double> bounds;
  final List<int> counts;
  final int total;
  final double sum;
  HistogramSnapshot({required this.bounds, required this.counts, required this.total, required this.sum});

  double quantile(double q) {
    if (total == 0) return double.nan;
    final target = total * q;
    for (var i = 0; i < counts.length; i++) {
      if (counts[i] >= target) return bounds[i];
    }
    return double.infinity;
  }
}

const defaultBuckets = <double>[
  50e-6, 100e-6, 250e-6, 500e-6,
  1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
  1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
];

class Histogram {
  final String name;
  final String help;
  final Map<String, String> labels;
  final List<double> _bounds;
  final List<int> _counts;
  int _total = 0;
  double _sum = 0.0;

  Histogram({
    required this.name,
    this.help = '',
    Map<String, String>? labels,
    List<double>? bounds,
  })  : labels = Map.unmodifiable(labels ?? {}),
        _bounds = (bounds == null || bounds.isEmpty
            ? List.of(defaultBuckets)
            : List.of(bounds))..sort(),
        _counts = List.filled(
            (bounds == null || bounds.isEmpty ? defaultBuckets.length : bounds.length),
            0);

  void observe(double v) {
    _total += 1;
    _sum += v;
    for (var i = 0; i < _bounds.length; i++) {
      if (v <= _bounds[i]) _counts[i] += 1;
    }
  }

  void observeDuration(Duration d) => observe(d.inMicroseconds / 1e6);

  HistogramSnapshot snapshot() => HistogramSnapshot(
        bounds: List.unmodifiable(_bounds),
        counts: List.unmodifiable(_counts),
        total: _total,
        sum: _sum,
      );
}

String _metricKey(String name, Map<String, String> labels) {
  if (labels.isEmpty) return name;
  final keys = labels.keys.toList()..sort();
  final buf = StringBuffer(name);
  for (final k in keys) {
    buf.write('|$k=${labels[k]}');
  }
  return buf.toString();
}

class Registry {
  final _counters = <String, Counter>{};
  final _gauges = <String, Gauge>{};
  final _histograms = <String, Histogram>{};

  Counter counter(String name, {String help = '', Map<String, String> labels = const {}}) {
    return _counters.putIfAbsent(_metricKey(name, labels),
        () => Counter(name: name, help: help, labels: labels));
  }

  Gauge gauge(String name, {String help = '', Map<String, String> labels = const {}}) {
    return _gauges.putIfAbsent(_metricKey(name, labels),
        () => Gauge(name: name, help: help, labels: labels));
  }

  Histogram histogram(String name,
      {String help = '', Map<String, String> labels = const {}, List<double>? bounds}) {
    return _histograms.putIfAbsent(_metricKey(name, labels),
        () => Histogram(name: name, help: help, labels: labels, bounds: bounds));
  }

  List<Counter> listCounters() {
    final l = _counters.values.toList();
    l.sort((a, b) => a.name.compareTo(b.name));
    return l;
  }

  List<Gauge> listGauges() {
    final l = _gauges.values.toList();
    l.sort((a, b) => a.name.compareTo(b.name));
    return l;
  }

  List<Histogram> listHistograms() {
    final l = _histograms.values.toList();
    l.sort((a, b) => a.name.compareTo(b.name));
    return l;
  }
}

// --- Config + Observability -------------------------------------------------

class Config {
  final String slug;
  final Level defaultLogLevel;
  final String promAddr;
  final List<String> redactedFields;
  final int logsRingSize;
  final int eventsRingSize;
  final String runDir;
  final String instanceUid;
  final String organismUid;
  final String organismSlug;

  const Config({
    this.slug = '',
    this.defaultLogLevel = Level.info,
    this.promAddr = '',
    this.redactedFields = const [],
    this.logsRingSize = 1024,
    this.eventsRingSize = 256,
    this.runDir = '',
    this.instanceUid = '',
    this.organismUid = '',
    this.organismSlug = '',
  });
}

class Logger {
  final Observability _obs;
  final String name;
  Level _level;

  Logger._(this._obs, this.name) : _level = _obs.cfg.defaultLogLevel;

  void setLevel(Level l) => _level = l;
  bool enabled(Level l) => l.value >= _level.value;

  void _log(Level lvl, String message, Map<String, dynamic>? fields) {
    if (!enabled(lvl)) return;
    final redact = Set<String>.from(_obs.cfg.redactedFields);
    final out = <String, String>{};
    if (fields != null) {
      for (final entry in fields.entries) {
        final k = entry.key;
        if (k.isEmpty) continue;
        if (redact.contains(k)) {
          out[k] = '<redacted>';
        } else {
          out[k] = _stringify(entry.value);
        }
      }
    }
    final entry = LogEntry(
      timestamp: DateTime.now(),
      level: lvl,
      slug: _obs.cfg.slug,
      instanceUid: _obs.cfg.instanceUid,
      message: message,
      fields: out,
      caller: _callerFrame(),
    );
    _obs.logRing?.push(entry);
  }

  void trace(String msg, [Map<String, dynamic>? f]) => _log(Level.trace, msg, f);
  void debug(String msg, [Map<String, dynamic>? f]) => _log(Level.debug, msg, f);
  void info(String msg, [Map<String, dynamic>? f]) => _log(Level.info, msg, f);
  void warn(String msg, [Map<String, dynamic>? f]) => _log(Level.warn, msg, f);
  void error(String msg, [Map<String, dynamic>? f]) => _log(Level.error, msg, f);
  void fatal(String msg, [Map<String, dynamic>? f]) => _log(Level.fatal, msg, f);
}

String _stringify(dynamic v) {
  if (v == null) return '';
  if (v is bool) return v ? 'true' : 'false';
  return v.toString();
}

String _callerFrame() {
  try {
    throw Error();
  } catch (_, st) {
    final lines = st.toString().split('\n');
    // Skip: throw Error, _callerFrame, _log, level method => frame 4 (0-indexed).
    if (lines.length > 4) {
      final l = lines[4];
      final m = RegExp(r'\(([^()]+?):(\d+):\d+\)').firstMatch(l);
      if (m != null) {
        final file = m.group(1)!;
        final base = file.split('/').last;
        return '$base:${m.group(2)}';
      }
    }
  }
  return '';
}

class Observability {
  final Config cfg;
  final Set<Family> families;
  final LogRing? logRing;
  final EventBus? eventBus;
  final Registry? registry;
  final Map<String, Logger> _loggers = {};

  Observability._(this.cfg, this.families)
      : logRing = families.contains(Family.logs) ? LogRing(cfg.logsRingSize) : null,
        eventBus = families.contains(Family.events) ? EventBus(cfg.eventsRingSize) : null,
        registry = families.contains(Family.metrics) ? Registry() : null;

  bool enabled(Family f) => families.contains(f);

  bool get isOrganismRoot =>
      cfg.organismUid.isNotEmpty && cfg.organismUid == cfg.instanceUid;

  Logger logger(String name) {
    if (!families.contains(Family.logs)) return _disabledLogger;
    return _loggers.putIfAbsent(name, () => Logger._(this, name));
  }

  Counter? counter(String name, {String help = '', Map<String, String> labels = const {}}) {
    return registry?.counter(name, help: help, labels: labels);
  }

  Gauge? gauge(String name, {String help = '', Map<String, String> labels = const {}}) {
    return registry?.gauge(name, help: help, labels: labels);
  }

  Histogram? histogram(String name,
      {String help = '', Map<String, String> labels = const {}, List<double>? bounds}) {
    return registry?.histogram(name, help: help, labels: labels, bounds: bounds);
  }

  void emit(EventType type, {Map<String, String>? payload}) {
    if (eventBus == null) return;
    final redact = Set<String>.from(cfg.redactedFields);
    final p = <String, String>{};
    if (payload != null) {
      for (final entry in payload.entries) {
        if (redact.contains(entry.key)) {
          p[entry.key] = '<redacted>';
        } else {
          p[entry.key] = entry.value;
        }
      }
    }
    eventBus!.emit(Event(
      timestamp: DateTime.now(),
      type: type,
      slug: cfg.slug,
      instanceUid: cfg.instanceUid,
      payload: p,
    ));
  }

  void close() => eventBus?.close();
}

final Logger _disabledLogger = Logger._(_DisabledObs(), '');

class _DisabledObs extends Observability {
  _DisabledObs() : super._(const Config(defaultLogLevel: Level.fatal), const {});
}

// --- Package-scope singleton -----------------------------------------------

Observability? _current;

Observability configure(Config cfg) {
  final families = parseOpObs(Platform.environment['OP_OBS'] ?? '');
  // If slug is empty, derive from executable.
  final effective = Config(
    slug: cfg.slug.isEmpty ? _basename(Platform.resolvedExecutable) : cfg.slug,
    defaultLogLevel: cfg.defaultLogLevel,
    promAddr: cfg.promAddr,
    redactedFields: cfg.redactedFields,
    logsRingSize: cfg.logsRingSize,
    eventsRingSize: cfg.eventsRingSize,
    runDir: cfg.runDir,
    instanceUid: cfg.instanceUid,
    organismUid: cfg.organismUid,
    organismSlug: cfg.organismSlug,
  );
  final obs = Observability._(effective, families);
  _current = obs;
  return obs;
}

Observability fromEnv([Config? base]) {
  base ??= const Config();
  final env = Platform.environment;
  return configure(Config(
    slug: base.slug.isNotEmpty ? base.slug : '',
    defaultLogLevel: base.defaultLogLevel,
    promAddr: base.promAddr.isNotEmpty ? base.promAddr : (env['OP_PROM_ADDR'] ?? ''),
    redactedFields: base.redactedFields,
    logsRingSize: base.logsRingSize,
    eventsRingSize: base.eventsRingSize,
    runDir: base.runDir.isNotEmpty ? base.runDir : (env['OP_RUN_DIR'] ?? ''),
    instanceUid: base.instanceUid.isNotEmpty ? base.instanceUid : (env['OP_INSTANCE_UID'] ?? ''),
    organismUid: base.organismUid.isNotEmpty ? base.organismUid : (env['OP_ORGANISM_UID'] ?? ''),
    organismSlug: base.organismSlug.isNotEmpty ? base.organismSlug : (env['OP_ORGANISM_SLUG'] ?? ''),
  ));
}

Observability current() {
  return _current ?? _DisabledObs();
}

void reset() {
  _current?.close();
  _current = null;
}

String _basename(String path) {
  final i = path.lastIndexOf(Platform.pathSeparator);
  if (i < 0) return path;
  return path.substring(i + 1);
}

// --- Disk writers -----------------------------------------------------------

void enableDiskWriters(String runDir) {
  final obs = _current;
  if (obs == null || runDir.isEmpty) return;
  Directory(runDir).createSync(recursive: true);

  if (obs.enabled(Family.logs) && obs.logRing != null) {
    final f = File('$runDir${Platform.pathSeparator}stdout.log');
    obs.logRing!.watch().listen((e) {
      final rec = <String, dynamic>{
        'kind': 'log',
        'ts': e.timestamp.toUtc().toIso8601String(),
        'level': e.level.name.toUpperCase(),
        'slug': e.slug,
        'instance_uid': e.instanceUid,
        'message': e.message,
      };
      if (e.sessionId.isNotEmpty) rec['session_id'] = e.sessionId;
      if (e.rpcMethod.isNotEmpty) rec['rpc_method'] = e.rpcMethod;
      if (e.fields.isNotEmpty) rec['fields'] = e.fields;
      if (e.caller.isNotEmpty) rec['caller'] = e.caller;
      if (e.chain.isNotEmpty) rec['chain'] = e.chain.map((h) => h.toJson()).toList();
      try {
        f.writeAsStringSync('${jsonEncode(rec)}\n', mode: FileMode.append, flush: false);
      } catch (_) {}
    });
  }

  if (obs.enabled(Family.events) && obs.eventBus != null) {
    final f = File('$runDir${Platform.pathSeparator}events.jsonl');
    obs.eventBus!.watch().listen((e) {
      final rec = <String, dynamic>{
        'kind': 'event',
        'ts': e.timestamp.toUtc().toIso8601String(),
        'type': e.type.protoName,
        'slug': e.slug,
        'instance_uid': e.instanceUid,
      };
      if (e.sessionId.isNotEmpty) rec['session_id'] = e.sessionId;
      if (e.payload.isNotEmpty) rec['payload'] = e.payload;
      if (e.chain.isNotEmpty) rec['chain'] = e.chain.map((h) => h.toJson()).toList();
      try {
        f.writeAsStringSync('${jsonEncode(rec)}\n', mode: FileMode.append, flush: false);
      } catch (_) {}
    });
  }
}

class MetaJson {
  final String slug;
  final String uid;
  final int pid;
  final DateTime startedAt;
  final String mode;
  final String transport;
  final String address;
  final String metricsAddr;
  final String logPath;
  final int logBytesRotated;
  final String organismUid;
  final String organismSlug;
  final bool isDefault;

  const MetaJson({
    required this.slug,
    required this.uid,
    required this.pid,
    required this.startedAt,
    this.mode = 'persistent',
    this.transport = '',
    this.address = '',
    this.metricsAddr = '',
    this.logPath = '',
    this.logBytesRotated = 0,
    this.organismUid = '',
    this.organismSlug = '',
    this.isDefault = false,
  });

  Map<String, dynamic> toJson() {
    final m = <String, dynamic>{
      'slug': slug,
      'uid': uid,
      'pid': pid,
      'started_at': startedAt.toUtc().toIso8601String(),
      'mode': mode,
      'transport': transport,
      'address': address,
    };
    if (metricsAddr.isNotEmpty) m['metrics_addr'] = metricsAddr;
    if (logPath.isNotEmpty) m['log_path'] = logPath;
    if (logBytesRotated > 0) m['log_bytes_rotated'] = logBytesRotated;
    if (organismUid.isNotEmpty) m['organism_uid'] = organismUid;
    if (organismSlug.isNotEmpty) m['organism_slug'] = organismSlug;
    if (isDefault) m['default'] = true;
    return m;
  }
}

void writeMetaJson(String runDir, MetaJson m) {
  Directory(runDir).createSync(recursive: true);
  final p = '$runDir${Platform.pathSeparator}meta.json';
  final tmp = '$p.tmp';
  final enc = JsonEncoder.withIndent('  ').convert(m.toJson());
  File(tmp).writeAsStringSync(enc);
  File(tmp).renameSync(p);
}

// Guarantee the `dart:math` import is used to avoid unused-import warnings
// when future randomness helpers land.
final _rng = Random();
// ignore: unused_element
double _noise() => _rng.nextDouble();
