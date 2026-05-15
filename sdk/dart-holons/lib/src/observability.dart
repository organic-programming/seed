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

import 'package:fixnum/fixnum.dart';
import 'package:grpc/grpc.dart';
import 'package:grpc/service_api.dart' as grpc_api;
import 'package:holons/gen/holons/v1/observability.pb.dart' as obs_pb;
import 'package:holons/gen/holons/v1/observability.pbgrpc.dart' as obs_grpc;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as pb_ts;

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
    if (tok == 'otel') {
      throw InvalidTokenError(
          tok, 'otel export is reserved for v2; not implemented in v1');
    }
    if (tok == 'sessions') {
      throw InvalidTokenError(
          tok, 'sessions are reserved for v2; not implemented in v1');
    }
    if (!_v1Tokens.contains(tok)) {
      throw InvalidTokenError(tok, 'unknown OP_OBS token');
    }
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
  final sessions = (env['OP_SESSIONS'] ?? '').trim();
  if (sessions.isNotEmpty) {
    throw InvalidTokenError(
        sessions, 'sessions are reserved for v2; not implemented in v1');
  }
  final raw = (env['OP_OBS'] ?? '').trim();
  if (raw.isEmpty) return;
  for (final part in raw.split(',')) {
    final tok = part.trim();
    if (tok.isEmpty) continue;
    if (tok == 'otel') {
      throw InvalidTokenError(
          tok, 'otel export is reserved for v2; not implemented in v1');
    }
    if (tok == 'sessions') {
      throw InvalidTokenError(
          tok, 'sessions are reserved for v2; not implemented in v1');
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
      }[u] ??
      Level.info;
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

List<Hop> enrichForMultilog(
    List<Hop> wire, String streamSourceSlug, String streamSourceUid) {
  return appendDirectChild(wire, streamSourceSlug, streamSourceUid);
}

// --- Log entry --------------------------------------------------------------

class LogEntry {
  final DateTime timestamp;
  final Level level;
  final String loggerName;
  final String slug;
  final String instanceUid;
  final String sessionId;
  final String rpcMethod;
  final String message;
  final Map<String, String> fields;
  final String caller;
  final List<Hop> chain;
  final bool private;

  LogEntry({
    required this.timestamp,
    required this.level,
    this.loggerName = '',
    required this.slug,
    required this.instanceUid,
    this.sessionId = '',
    this.rpcMethod = '',
    required this.message,
    this.fields = const {},
    this.caller = '',
    this.chain = const [],
    this.private = false,
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

  _ReplayAndWatch<LogEntry> replayAndWatch([DateTime? cutoff]) {
    final replay = cutoff == null
        ? List<LogEntry>.unmodifiable(_buf)
        : List<LogEntry>.unmodifiable(
            _buf.where((e) => !e.timestamp.isBefore(cutoff)),
          );
    final c = StreamController<LogEntry>();
    // Dart executes this synchronous block without interleaving awaits:
    // snapshot first, then register, so later pushes are buffered live without
    // repeating entries that were already included in the replay.
    _subs.add(c);
    c.onCancel = () {
      _subs.remove(c);
    };
    return _ReplayAndWatch(replay, c.stream, () async {
      _subs.remove(c);
      if (!c.isClosed) {
        await c.close();
      }
    });
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
  final bool private;

  Event({
    required this.timestamp,
    required this.type,
    required this.slug,
    required this.instanceUid,
    this.sessionId = '',
    this.payload = const {},
    this.chain = const [],
    this.private = false,
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

  _ReplayAndWatch<Event> replayAndWatch([DateTime? cutoff]) {
    if (_closed) {
      return _ReplayAndWatch(
        const <Event>[],
        const Stream<Event>.empty(),
        () async {},
      );
    }
    final replay = cutoff == null
        ? List<Event>.unmodifiable(_buf)
        : List<Event>.unmodifiable(
            _buf.where((e) => !e.timestamp.isBefore(cutoff)),
          );
    final c = StreamController<Event>();
    // Dart executes this synchronous block without interleaving awaits:
    // snapshot first, then register, so later emits are buffered live without
    // repeating events that were already included in the replay.
    _subs.add(c);
    c.onCancel = () {
      _subs.remove(c);
    };
    return _ReplayAndWatch(replay, c.stream, () async {
      _subs.remove(c);
      if (!c.isClosed) {
        await c.close();
      }
    });
  }

  void close() {
    _closed = true;
    for (final s in _subs) s.close();
    _subs.clear();
  }
}

class _ReplayAndWatch<T> {
  final List<T> replay;
  final Stream<T> live;
  final Future<void> Function() stop;
  const _ReplayAndWatch(this.replay, this.live, this.stop);
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
  HistogramSnapshot(
      {required this.bounds,
      required this.counts,
      required this.total,
      required this.sum});

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
  50e-6,
  100e-6,
  250e-6,
  500e-6,
  1e-3,
  2.5e-3,
  5e-3,
  10e-3,
  25e-3,
  50e-3,
  100e-3,
  250e-3,
  500e-3,
  1.0,
  2.5,
  5.0,
  10.0,
  30.0,
  60.0,
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
            : List.of(bounds))
          ..sort(),
        _counts = List.filled(
            (bounds == null || bounds.isEmpty
                ? defaultBuckets.length
                : bounds.length),
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

  Counter counter(String name,
      {String help = '', Map<String, String> labels = const {}}) {
    return _counters.putIfAbsent(_metricKey(name, labels),
        () => Counter(name: name, help: help, labels: labels));
  }

  Gauge gauge(String name,
      {String help = '', Map<String, String> labels = const {}}) {
    return _gauges.putIfAbsent(_metricKey(name, labels),
        () => Gauge(name: name, help: help, labels: labels));
  }

  Histogram histogram(String name,
      {String help = '',
      Map<String, String> labels = const {},
      List<double>? bounds}) {
    return _histograms.putIfAbsent(
        _metricKey(name, labels),
        () =>
            Histogram(name: name, help: help, labels: labels, bounds: bounds));
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

class PromServer {
  PromServer(this.addr);

  final String addr;
  HttpServer? _server;

  Future<String> start() async {
    final current = _server;
    if (current != null) {
      return _metricsUrl(current);
    }
    final parsed = _parsePromAddr(addr);
    final server = await HttpServer.bind(parsed.$1, parsed.$2);
    _server = server;
    unawaited(_serve(server));
    return _metricsUrl(server);
  }

  Future<void> close() async {
    final server = _server;
    _server = null;
    if (server != null) {
      await server.close(force: true);
    }
  }

  Future<void> _serve(HttpServer server) async {
    await for (final request in server) {
      if (request.uri.path != '/metrics') {
        request.response.statusCode = HttpStatus.notFound;
        await request.response.close();
        continue;
      }
      final obs = current();
      request.response.headers.contentType = ContentType(
        'text',
        'plain',
        charset: 'utf-8',
        parameters: const {'version': '0.0.4'},
      );
      if (!obs.enabled(Family.metrics)) {
        request.response.statusCode = HttpStatus.serviceUnavailable;
        request.response.write('# metrics family disabled (OP_OBS)\n');
      } else if (!obs.enabled(Family.prom)) {
        request.response.statusCode = HttpStatus.serviceUnavailable;
        request.response.write('# prom family disabled (OP_OBS)\n');
      } else {
        request.response.write(toPrometheusText(obs));
      }
      await request.response.close();
    }
  }
}

String toPrometheusText(Observability obs) {
  if (!obs.enabled(Family.metrics) || obs.registry == null) {
    return '# metrics family disabled (OP_OBS)\n';
  }
  final groups = <String, _PromGroup>{};
  _PromGroup ensure(String name, String help, String type) {
    return groups.putIfAbsent(name, () => _PromGroup(name, help, type))
      ..setHelp(help);
  }

  final injected = <String, String>{'slug': obs.cfg.slug};
  if (obs.cfg.instanceUid.isNotEmpty) {
    injected['instance_uid'] = obs.cfg.instanceUid;
  }
  for (final counter in obs.registry!.listCounters()) {
    ensure(counter.name, counter.help, 'counter').counters.add(counter);
  }
  for (final gauge in obs.registry!.listGauges()) {
    ensure(gauge.name, gauge.help, 'gauge').gauges.add(gauge);
  }
  for (final histogram in obs.registry!.listHistograms()) {
    ensure(histogram.name, histogram.help, 'histogram')
        .histograms
        .add(histogram);
  }

  final names = groups.keys.toList()..sort();
  final out = StringBuffer();
  for (final name in names) {
    final group = groups[name]!;
    out.writeln('# HELP ${group.name} ${_promEscapeHelp(group.help)}');
    out.writeln('# TYPE ${group.name} ${group.type}');
    for (final counter in group.counters) {
      out.writeln(
          '${counter.name}${_promLabels(_mergeLabels(counter.labels, injected))} ${counter.value()}');
    }
    for (final gauge in group.gauges) {
      out.writeln(
          '${gauge.name}${_promLabels(_mergeLabels(gauge.labels, injected))} ${_formatFloat(gauge.value())}');
    }
    for (final histogram in group.histograms) {
      final snapshot = histogram.snapshot();
      final labels = _mergeLabels(histogram.labels, injected);
      for (var i = 0; i < snapshot.bounds.length; i++) {
        labels['le'] = _formatFloat(snapshot.bounds[i]);
        out.writeln(
            '${histogram.name}_bucket${_promLabels(labels)} ${snapshot.counts[i]}');
      }
      labels['le'] = '+Inf';
      out.writeln(
          '${histogram.name}_bucket${_promLabels(labels)} ${snapshot.total}');
      labels.remove('le');
      out.writeln(
          '${histogram.name}_sum${_promLabels(labels)} ${_formatFloat(snapshot.sum)}');
      out.writeln(
          '${histogram.name}_count${_promLabels(labels)} ${snapshot.total}');
    }
  }
  return out.toString();
}

class _PromGroup {
  _PromGroup(this.name, String help, this.type) : help = help;

  final String name;
  String help;
  final String type;
  final counters = <Counter>[];
  final gauges = <Gauge>[];
  final histograms = <Histogram>[];

  void setHelp(String next) {
    if (help.isEmpty) {
      help = next;
    }
  }
}

(String, int) _parsePromAddr(String raw) {
  final trimmed = raw.trim().isEmpty ? ':0' : raw.trim();
  if (trimmed.startsWith(':')) {
    return ('0.0.0.0', int.parse(trimmed.substring(1)));
  }
  final index = trimmed.lastIndexOf(':');
  if (index <= 0 || index == trimmed.length - 1) {
    throw ArgumentError('invalid Prometheus address "$raw"');
  }
  return (trimmed.substring(0, index), int.parse(trimmed.substring(index + 1)));
}

String _metricsUrl(HttpServer server) {
  final host = _advertisedPromHost(server.address.address);
  return 'http://$host:${server.port}/metrics';
}

String _advertisedPromHost(String host) {
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

Map<String, String> _mergeLabels(
    Map<String, String> base, Map<String, String> extra) {
  final out = <String, String>{};
  extra.forEach((key, value) {
    if (value.isNotEmpty) {
      out[key] = value;
    }
  });
  out.addAll(base);
  return out;
}

String _promLabels(Map<String, String> labels) {
  if (labels.isEmpty) return '';
  final keys = labels.keys.toList()..sort();
  return '{${keys.map((key) => '$key="${_promEscapeValue(labels[key] ?? '')}"').join(',')}}';
}

String _promEscapeValue(String value) {
  return value
      .replaceAll('\\', r'\\')
      .replaceAll('\n', r'\n')
      .replaceAll('"', r'\"');
}

String _promEscapeHelp(String value) {
  return value.replaceAll('\\', r'\\').replaceAll('\n', r'\n');
}

String _formatFloat(double value) {
  if (value.isInfinite) {
    return value.isNegative ? '-Inf' : '+Inf';
  }
  if (value.isNaN) return 'NaN';
  return value.toString();
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

  void _log(Level lvl, String message, Map<String, dynamic>? fields,
      {bool private = false}) {
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
      loggerName: name,
      slug: _obs.cfg.slug,
      instanceUid: _obs.cfg.instanceUid,
      message: message,
      fields: out,
      caller: _callerFrame(),
      chain: _selfChain(_obs.cfg),
      private: private,
    );
    _obs.logRing?.push(entry);
  }

  void trace(String msg,
          {Map<String, dynamic>? fields, bool private = false}) =>
      _log(Level.trace, msg, fields, private: private);
  void debug(String msg,
          {Map<String, dynamic>? fields, bool private = false}) =>
      _log(Level.debug, msg, fields, private: private);
  void info(String msg, {Map<String, dynamic>? fields, bool private = false}) =>
      _log(Level.info, msg, fields, private: private);
  void warn(String msg, {Map<String, dynamic>? fields, bool private = false}) =>
      _log(Level.warn, msg, fields, private: private);
  void error(String msg,
          {Map<String, dynamic>? fields, bool private = false}) =>
      _log(Level.error, msg, fields, private: private);
  void fatal(String msg,
          {Map<String, dynamic>? fields, bool private = false}) =>
      _log(Level.fatal, msg, fields, private: private);
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
      : logRing =
            families.contains(Family.logs) ? LogRing(cfg.logsRingSize) : null,
        eventBus = families.contains(Family.events)
            ? EventBus(cfg.eventsRingSize)
            : null,
        registry = families.contains(Family.metrics) ? Registry() : null;

  bool enabled(Family f) => families.contains(f);

  bool get isOrganismRoot =>
      cfg.organismUid.isNotEmpty && cfg.organismUid == cfg.instanceUid;

  Logger logger(String name) {
    if (!families.contains(Family.logs)) return _disabledLogger;
    return _loggers.putIfAbsent(name, () => Logger._(this, name));
  }

  Counter? counter(String name,
      {String help = '', Map<String, String> labels = const {}}) {
    return registry?.counter(name, help: help, labels: labels);
  }

  Gauge? gauge(String name,
      {String help = '', Map<String, String> labels = const {}}) {
    return registry?.gauge(name, help: help, labels: labels);
  }

  Histogram? histogram(String name,
      {String help = '',
      Map<String, String> labels = const {},
      List<double>? bounds}) {
    return registry?.histogram(name,
        help: help, labels: labels, bounds: bounds);
  }

  void emit(EventType type,
      {Map<String, String>? payload, bool private = false}) {
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
      chain: _selfChain(cfg),
      private: private,
    ));
  }

  void close() => eventBus?.close();
}

class MemberRelay {
  MemberRelay({
    required this.childSlug,
    required this.childUid,
    required this.channel,
    required this.observability,
    this.forceStreams = false,
  });

  String childSlug;
  String childUid;
  final grpc_api.ClientChannel channel;
  final Observability observability;
  final bool forceStreams;

  StreamSubscription<obs_pb.LogEntry>? _logsSub;
  StreamSubscription<obs_pb.EventInfo>? _eventsSub;
  Timer? _retryTimer;
  bool _isRunning = false;
  bool _stopping = false;
  bool _failed = false;
  bool _starting = false;

  bool get isRunning => _isRunning;

  Future<void> start() async {
    if (_isRunning || _starting) return;
    _stopping = false;
    await _openStreams();
  }

  Future<void> _openStreams() async {
    if (_stopping || _starting) return;
    var wantLogs = observability.enabled(Family.logs);
    var wantEvents = observability.enabled(Family.events);
    if (!wantLogs && !wantEvents) {
      if (!forceStreams) return;
      wantLogs = true;
      wantEvents = true;
    }

    _starting = true;
    _failed = false;
    _retryTimer?.cancel();
    _retryTimer = null;
    final client = obs_grpc.HolonObservabilityClient(channel);

    try {
      if (wantLogs) {
        final stream = client.logs(obs_pb.LogsRequest(follow: true));
        _logsSub = stream.listen(
          _relayLog,
          onError: (Object error) => _handleStreamFailure(error),
          onDone: () => _handleStreamDone('logs'),
          cancelOnError: true,
        );
      }
      if (wantEvents) {
        final stream = client.events(obs_pb.EventsRequest(follow: true));
        _eventsSub = stream.listen(
          _relayEvent,
          onError: (Object error) => _handleStreamFailure(error),
          onDone: () => _handleStreamDone('events'),
          cancelOnError: true,
        );
      }
      _isRunning = _logsSub != null || _eventsSub != null;
    } catch (error) {
      _isRunning = false;
      await _cancelSubscriptions();
      _warn(error);
      _scheduleRetry();
    } finally {
      _starting = false;
    }
  }

  Future<void> stop() async {
    _stopping = true;
    _retryTimer?.cancel();
    _retryTimer = null;
    await _cancelSubscriptions();
    _isRunning = false;
    _starting = false;
    _stopping = false;
  }

  void _relayLog(obs_pb.LogEntry proto) {
    final obs = current();
    if (!obs.enabled(Family.logs) || obs.logRing == null) {
      return;
    }
    final entry = _logEntryFromProto(proto);
    _refreshChildIdentity(entry.slug, entry.instanceUid, entry.chain);
    obs.logRing!.push(LogEntry(
      timestamp: entry.timestamp,
      level: entry.level,
      loggerName: entry.loggerName,
      slug: entry.slug,
      instanceUid: entry.instanceUid,
      sessionId: entry.sessionId,
      rpcMethod: entry.rpcMethod,
      message: entry.message,
      fields: entry.fields,
      caller: entry.caller,
      chain: appendDirectChild(entry.chain, childSlug, childUid),
      private: entry.private,
    ));
  }

  void _relayEvent(obs_pb.EventInfo proto) {
    final obs = current();
    if (!obs.enabled(Family.events) || obs.eventBus == null) {
      return;
    }
    final event = _eventFromProto(proto);
    _refreshChildIdentity(event.slug, event.instanceUid, event.chain);
    obs.eventBus!.emit(Event(
      timestamp: event.timestamp,
      type: event.type,
      slug: event.slug,
      instanceUid: event.instanceUid,
      sessionId: event.sessionId,
      payload: event.payload,
      chain: appendDirectChild(event.chain, childSlug, childUid),
      private: event.private,
    ));
  }

  void _handleStreamDone(String streamName) {
    if (_stopping || _failed) return;
    _handleStreamFailure('$streamName stream closed');
  }

  void _handleStreamFailure(Object error) {
    if (_stopping || _failed) return;
    _failed = true;
    _isRunning = false;
    _warn(error);
    unawaited(_cancelSubscriptions().whenComplete(() {
      _scheduleRetry();
    }));
  }

  void _scheduleRetry() {
    if (_stopping || _retryTimer != null) return;
    _retryTimer = Timer(_memberRelayRetryDelay, () {
      _retryTimer = null;
      if (!_stopping) {
        unawaited(_openStreams());
      }
    });
  }

  void _refreshChildIdentity(String slug, String uid, List<Hop> chain) {
    if (chain.isNotEmpty) return;
    final nextSlug = slug.trim();
    final nextUid = uid.trim();
    if (nextSlug.isNotEmpty) {
      childSlug = nextSlug;
    }
    if (nextUid.isNotEmpty) {
      childUid = nextUid;
    }
  }

  void _warn(Object error) {
    observability
        .logger('member-relay')
        .warn('member relay stream error', fields: {
      'child_slug': childSlug,
      'child_uid': childUid,
      'error': error,
    });
  }

  Future<void> _cancelSubscriptions() async {
    final subs = [
      if (_logsSub != null) _logsSub!,
      if (_eventsSub != null) _eventsSub!,
    ];
    _logsSub = null;
    _eventsSub = null;
    try {
      await Future.wait<void>([
        for (final sub in subs) sub.cancel(),
      ]).timeout(_memberRelayStopTimeout);
    } on TimeoutException {
      // Process shutdown must not wait forever for a transport to surface
      // cancellation on a follow stream.
    }
  }
}

const _memberRelayRetryDelay = Duration(milliseconds: 100);
const _memberRelayStopTimeout = Duration(seconds: 2);

final Logger _disabledLogger = Logger._(_DisabledObs(), '');

class _DisabledObs extends Observability {
  _DisabledObs()
      : super._(const Config(defaultLogLevel: Level.fatal), const {});
}

// --- Package-scope singleton -----------------------------------------------

Observability? _current;

List<Hop> _selfChain(Config _) => const [];

Observability configure(Config cfg, {Map<String, String>? env}) {
  env ??= Platform.environment;
  checkEnv(env);
  final families = parseOpObs(env['OP_OBS'] ?? '');
  // If slug is empty, derive from executable.
  final slug =
      cfg.slug.isEmpty ? _basename(Platform.resolvedExecutable) : cfg.slug;
  final uid = cfg.instanceUid.isEmpty ? _newInstanceUid() : cfg.instanceUid;
  final effective = Config(
    slug: slug,
    defaultLogLevel: cfg.defaultLogLevel,
    promAddr: cfg.promAddr,
    redactedFields: cfg.redactedFields,
    logsRingSize: cfg.logsRingSize,
    eventsRingSize: cfg.eventsRingSize,
    runDir: cfg.runDir.isEmpty ? '' : deriveRunDir(cfg.runDir, slug, uid),
    instanceUid: uid,
    organismUid: cfg.organismUid,
    organismSlug: cfg.organismSlug,
  );
  final obs = Observability._(effective, families);
  _current = obs;
  return obs;
}

Observability fromEnv([Config? base, Map<String, String>? env]) {
  base ??= const Config();
  env ??= Platform.environment;
  return configure(
      Config(
        slug: base.slug.isNotEmpty ? base.slug : '',
        defaultLogLevel: base.defaultLogLevel,
        promAddr: base.promAddr.isNotEmpty
            ? base.promAddr
            : (env['OP_PROM_ADDR'] ?? ''),
        redactedFields: base.redactedFields,
        logsRingSize: base.logsRingSize,
        eventsRingSize: base.eventsRingSize,
        runDir:
            base.runDir.isNotEmpty ? base.runDir : (env['OP_RUN_DIR'] ?? ''),
        instanceUid: base.instanceUid.isNotEmpty
            ? base.instanceUid
            : (env['OP_INSTANCE_UID'] ?? ''),
        organismUid: base.organismUid.isNotEmpty
            ? base.organismUid
            : (env['OP_ORGANISM_UID'] ?? ''),
        organismSlug: base.organismSlug.isNotEmpty
            ? base.organismSlug
            : (env['OP_ORGANISM_SLUG'] ?? ''),
      ),
      env: env);
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

String deriveRunDir(String root, String slug, String uid) {
  if (root.isEmpty || slug.isEmpty || uid.isEmpty) return root;
  return [root, slug, uid].join(Platform.pathSeparator);
}

String _newInstanceUid() {
  final r = Random.secure();
  String hex(int value, int width) =>
      value.toRadixString(16).padLeft(width, '0');
  return [
    hex(DateTime.now().microsecondsSinceEpoch, 12),
    hex(r.nextInt(1 << 32), 8),
    hex(r.nextInt(1 << 32), 8),
  ].join('-');
}

// --- Proto conversion + gRPC service ---------------------------------------

pb_ts.Timestamp _timestamp(DateTime dt) {
  final utc = dt.toUtc();
  final micros = utc.microsecondsSinceEpoch;
  return pb_ts.Timestamp(
    seconds: Int64(micros ~/ 1000000),
    nanos: (micros % 1000000) * 1000,
  );
}

obs_pb.ChainHop _hopToProto(Hop hop) {
  return obs_pb.ChainHop(slug: hop.slug, instanceUid: hop.instanceUid);
}

obs_pb.LogLevel _levelToProto(Level level) {
  return obs_pb.LogLevel.valueOf(level.value) ??
      obs_pb.LogLevel.LOG_LEVEL_UNSPECIFIED;
}

obs_pb.EventType _eventTypeToProto(EventType type) {
  return obs_pb.EventType.valueOf(type.value) ??
      obs_pb.EventType.EVENT_TYPE_UNSPECIFIED;
}

obs_pb.LogEntry toProtoLogEntry(LogEntry entry) {
  return obs_pb.LogEntry(
    ts: _timestamp(entry.timestamp),
    level: _levelToProto(entry.level),
    slug: entry.slug,
    instanceUid: entry.instanceUid,
    sessionId: entry.sessionId,
    rpcMethod: entry.rpcMethod,
    message: entry.message,
    fields: entry.fields.entries,
    caller: entry.caller,
    chain: entry.chain.map(_hopToProto),
  );
}

DateTime _dateTimeFromProto(pb_ts.Timestamp ts) {
  final micros = ts.seconds.toInt() * 1000000 + ts.nanos ~/ 1000;
  return DateTime.fromMicrosecondsSinceEpoch(micros, isUtc: true);
}

Hop _hopFromProto(obs_pb.ChainHop hop) {
  return Hop(slug: hop.slug, instanceUid: hop.instanceUid);
}

Level _levelFromProto(obs_pb.LogLevel level) {
  return Level.values.firstWhere(
    (candidate) => candidate.value == level.value,
    orElse: () => Level.unset,
  );
}

EventType _eventTypeFromProto(obs_pb.EventType type) {
  return EventType.values.firstWhere(
    (candidate) => candidate.value == type.value,
    orElse: () => EventType.unspecified,
  );
}

LogEntry _logEntryFromProto(obs_pb.LogEntry entry) {
  return LogEntry(
    timestamp: entry.hasTs()
        ? _dateTimeFromProto(entry.ts)
        : DateTime.fromMicrosecondsSinceEpoch(0, isUtc: true),
    level: _levelFromProto(entry.level),
    slug: entry.slug,
    instanceUid: entry.instanceUid,
    sessionId: entry.sessionId,
    rpcMethod: entry.rpcMethod,
    message: entry.message,
    fields: Map.unmodifiable(entry.fields),
    caller: entry.caller,
    chain: List.unmodifiable(entry.chain.map(_hopFromProto)),
  );
}

obs_pb.HistogramSample _histogramToProto(HistogramSnapshot snapshot) {
  return obs_pb.HistogramSample(
    buckets: [
      for (var i = 0; i < snapshot.bounds.length; i++)
        obs_pb.Bucket(
            upperBound: snapshot.bounds[i], count: Int64(snapshot.counts[i])),
    ],
    count: Int64(snapshot.total),
    sum: snapshot.sum,
  );
}

List<obs_pb.MetricSample> toProtoMetricSamples(Registry registry) {
  return [
    for (final counter in registry.listCounters())
      obs_pb.MetricSample(
        name: counter.name,
        help: counter.help,
        labels: counter.labels.entries,
        counter: Int64(counter.value()),
      ),
    for (final gauge in registry.listGauges())
      obs_pb.MetricSample(
        name: gauge.name,
        help: gauge.help,
        labels: gauge.labels.entries,
        gauge: gauge.value(),
      ),
    for (final histogram in registry.listHistograms())
      obs_pb.MetricSample(
        name: histogram.name,
        help: histogram.help,
        labels: histogram.labels.entries,
        histogram: _histogramToProto(histogram.snapshot()),
      ),
  ];
}

obs_pb.EventInfo toProtoEvent(Event event) {
  return obs_pb.EventInfo(
    ts: _timestamp(event.timestamp),
    type: _eventTypeToProto(event.type),
    slug: event.slug,
    instanceUid: event.instanceUid,
    sessionId: event.sessionId,
    payload: event.payload.entries,
    chain: event.chain.map(_hopToProto),
  );
}

Event _eventFromProto(obs_pb.EventInfo event) {
  return Event(
    timestamp: event.hasTs()
        ? _dateTimeFromProto(event.ts)
        : DateTime.fromMicrosecondsSinceEpoch(0, isUtc: true),
    type: _eventTypeFromProto(event.type),
    slug: event.slug,
    instanceUid: event.instanceUid,
    sessionId: event.sessionId,
    payload: Map.unmodifiable(event.payload),
    chain: List.unmodifiable(event.chain.map(_hopFromProto)),
  );
}

class HolonObservabilityService extends obs_grpc.HolonObservabilityServiceBase {
  final Observability obs;
  HolonObservabilityService([Observability? obs]) : obs = obs ?? current();

  @override
  Stream<obs_pb.LogEntry> logs(
      ServiceCall call, obs_pb.LogsRequest request) async* {
    if (!obs.enabled(Family.logs) || obs.logRing == null) {
      throw GrpcError.failedPrecondition('logs family is not enabled (OP_OBS)');
    }
    final minLevel = request.hasMinLevel() && request.minLevel.value != 0
        ? request.minLevel.value
        : Level.info.value;
    final cutoff = request.hasSince()
        ? DateTime.now().subtract(Duration(
            seconds: request.since.seconds.toInt(),
            microseconds: request.since.nanos ~/ 1000,
          ))
        : null;

    final followed =
        request.follow ? obs.logRing!.replayAndWatch(cutoff) : null;

    try {
      final entries = followed?.replay ??
          (cutoff == null
              ? obs.logRing!.drain()
              : obs.logRing!.drainSince(cutoff));
      for (final entry in entries) {
        if (!entry.private &&
            _matchLog(
                entry, minLevel, request.sessionIds, request.rpcMethods)) {
          yield toProtoLogEntry(entry);
        }
      }
      if (!request.follow) return;
      await for (final entry in followed!.live) {
        if (!entry.private &&
            _matchLog(
                entry, minLevel, request.sessionIds, request.rpcMethods)) {
          yield toProtoLogEntry(entry);
        }
      }
    } finally {
      await followed?.stop();
    }
  }

  @override
  Future<obs_pb.MetricsSnapshot> metrics(
      ServiceCall call, obs_pb.MetricsRequest request) async {
    if (!obs.enabled(Family.metrics) || obs.registry == null) {
      throw GrpcError.failedPrecondition(
          'metrics family is not enabled (OP_OBS)');
    }
    var samples = toProtoMetricSamples(obs.registry!);
    if (request.namePrefixes.isNotEmpty) {
      samples = samples
          .where((sample) => request.namePrefixes
              .any((prefix) => sample.name.startsWith(prefix)))
          .toList();
    }
    return obs_pb.MetricsSnapshot(
      capturedAt: _timestamp(DateTime.now()),
      slug: obs.cfg.slug,
      instanceUid: obs.cfg.instanceUid,
      samples: samples,
    );
  }

  @override
  Stream<obs_pb.EventInfo> events(
      ServiceCall call, obs_pb.EventsRequest request) async* {
    if (!obs.enabled(Family.events) || obs.eventBus == null) {
      throw GrpcError.failedPrecondition(
          'events family is not enabled (OP_OBS)');
    }
    final wanted = request.types.map((t) => t.value).toSet();
    final cutoff = request.hasSince()
        ? DateTime.now().subtract(Duration(
            seconds: request.since.seconds.toInt(),
            microseconds: request.since.nanos ~/ 1000,
          ))
        : null;

    final followed =
        request.follow ? obs.eventBus!.replayAndWatch(cutoff) : null;

    try {
      final events = followed?.replay ??
          (cutoff == null
              ? obs.eventBus!.drain()
              : obs.eventBus!.drainSince(cutoff));
      for (final event in events) {
        if (!event.private && _matchEvent(event, wanted))
          yield toProtoEvent(event);
      }
      if (!request.follow) return;
      await for (final event in followed!.live) {
        if (!event.private && _matchEvent(event, wanted)) {
          yield toProtoEvent(event);
        }
      }
    } finally {
      await followed?.stop();
    }
  }
}

Service registerService([Observability? obs]) => HolonObservabilityService(obs);

bool _matchLog(LogEntry entry, int minLevel, List<String> sessionIds,
    List<String> rpcMethods) {
  if (entry.level.value < minLevel) return false;
  if (sessionIds.isNotEmpty && !sessionIds.contains(entry.sessionId))
    return false;
  if (rpcMethods.isNotEmpty && !rpcMethods.contains(entry.rpcMethod))
    return false;
  return true;
}

bool _matchEvent(Event event, Set<int> wanted) {
  if (wanted.isEmpty) return true;
  return wanted.contains(event.type.value);
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
      if (e.chain.isNotEmpty)
        rec['chain'] = e.chain.map((h) => h.toJson()).toList();
      try {
        f.writeAsStringSync('${jsonEncode(rec)}\n',
            mode: FileMode.append, flush: false);
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
      if (e.chain.isNotEmpty)
        rec['chain'] = e.chain.map((h) => h.toJson()).toList();
      try {
        f.writeAsStringSync('${jsonEncode(rec)}\n',
            mode: FileMode.append, flush: false);
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
