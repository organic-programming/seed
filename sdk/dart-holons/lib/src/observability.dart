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
  debug(5),
  info(9),
  warn(13),
  error(17),
  fatal(21);

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

// --- OTLP attribute and event names -----------------------------------------

const eventInstanceSpawned = 'instance.spawned';
const eventInstanceReady = 'instance.ready';
const eventInstanceExited = 'instance.exited';
const eventInstanceCrashed = 'instance.crashed';
const eventSessionStarted = 'session.started';
const eventSessionEnded = 'session.ended';
const eventHandlerPanic = 'handler.panic';
const eventConfigReloaded = 'config.reloaded';

const attrHolonsSlug = 'holons.slug';
const attrHolonsInstanceUid = 'holons.instance_uid';
const attrHolonsSessionId = 'holons.session_id';
const attrHolonsTransport = 'holons.transport';
const attrServiceName = 'service.name';
const attrServiceInstanceId = 'service.instance.id';
const attrRpcMethod = 'rpc.method';
const attrLoggerName = 'logger.name';
const attrCodeCaller = 'code.caller';

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

// --- Log record -------------------------------------------------------------

class LogRecord {
  final obs_pb.LogRecord record;
  final bool private;

  LogRecord({
    obs_pb.LogRecord? record,
    this.private = false,
  }) : record = record == null
            ? obs_pb.LogRecord()
            : obs_pb.LogRecord.fromBuffer(record.writeToBuffer());

  DateTime get timestamp {
    if (record.timeUnixNano == Int64.ZERO) {
      return DateTime.fromMicrosecondsSinceEpoch(0, isUtc: true);
    }
    return DateTime.fromMicrosecondsSinceEpoch(
      (record.timeUnixNano ~/ Int64(1000)).toInt(),
      isUtc: true,
    );
  }

  Level get level => Level.values.firstWhere(
        (candidate) => candidate.value == record.severityNumber.value,
        orElse: () => Level.unset,
      );

  String get message => anyValueString(record.body);
  String get eventName => record.eventName;
  List<Hop> get chain => List.unmodifiable(record.chain.map(_hopFromString));
  String attr(String key) => stringAttribute(record.attributes, key);
  String get slug => attr(attrHolonsSlug);
  String get instanceUid => attr(attrHolonsInstanceUid);
  String get sessionId => attr(attrHolonsSessionId);
  String get rpcMethod => attr(attrRpcMethod);
  String get loggerName => attr(attrLoggerName);
  String get caller => attr(attrCodeCaller);
  Map<String, String> get fields => userAttributesMap(record.attributes);
}

// --- Ring buffer + event bus ------------------------------------------------

class LogRing {
  final int capacity;
  final List<LogRecord> _buf = [];
  final _subs = <StreamController<LogRecord>>[];
  LogRing([this.capacity = 1024]);

  void push(LogRecord e) {
    _buf.add(e);
    if (_buf.length > capacity) _buf.removeRange(0, _buf.length - capacity);
    for (final s in List.of(_subs)) {
      if (!s.isClosed) s.add(e);
    }
  }

  List<LogRecord> drain() => List.unmodifiable(_buf);
  List<LogRecord> drainSince(DateTime cutoff) =>
      List.unmodifiable(_buf.where((e) => !e.timestamp.isBefore(cutoff)));

  Stream<LogRecord> watch() {
    final c = StreamController<LogRecord>.broadcast();
    _subs.add(c);
    c.onCancel = () {
      _subs.remove(c);
      c.close();
    };
    return c.stream;
  }

  _ReplayAndWatch<LogRecord> replayAndWatch([DateTime? cutoff]) {
    final replay = cutoff == null
        ? List<LogRecord>.unmodifiable(_buf)
        : List<LogRecord>.unmodifiable(
            _buf.where((e) => !e.timestamp.isBefore(cutoff)),
          );
    final c = StreamController<LogRecord>();
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

class EventBus {
  final int capacity;
  final List<LogRecord> _buf = [];
  final _subs = <StreamController<LogRecord>>[];
  bool _closed = false;
  EventBus([this.capacity = 256]);

  void emit(LogRecord e) {
    if (_closed) return;
    _buf.add(e);
    if (_buf.length > capacity) _buf.removeRange(0, _buf.length - capacity);
    for (final s in List.of(_subs)) {
      if (!s.isClosed) s.add(e);
    }
  }

  List<LogRecord> drain() => List.unmodifiable(_buf);
  List<LogRecord> drainSince(DateTime cutoff) =>
      List.unmodifiable(_buf.where((e) => !e.timestamp.isBefore(cutoff)));

  Stream<LogRecord> watch() {
    final c = StreamController<LogRecord>.broadcast();
    _subs.add(c);
    c.onCancel = () {
      _subs.remove(c);
      c.close();
    };
    return c.stream;
  }

  _ReplayAndWatch<LogRecord> replayAndWatch([DateTime? cutoff]) {
    if (_closed) {
      return _ReplayAndWatch(
        const <LogRecord>[],
        const Stream<LogRecord>.empty(),
        () async {},
      );
    }
    final replay = cutoff == null
        ? List<LogRecord>.unmodifiable(_buf)
        : List<LogRecord>.unmodifiable(
            _buf.where((e) => !e.timestamp.isBefore(cutoff)),
          );
    final c = StreamController<LogRecord>();
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
  final double min;
  final double max;
  HistogramSnapshot(
      {required this.bounds,
      required this.counts,
      required this.total,
      required this.sum,
      required this.min,
      required this.max});

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
  double _min = 0.0;
  double _max = 0.0;

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
    if (_total == 0) {
      _min = v;
      _max = v;
    } else {
      if (v < _min) _min = v;
      if (v > _max) _max = v;
    }
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
        min: _min,
        max: _max,
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
    final attributes = resourceAttributes(_obs.cfg);
    if (name.isNotEmpty) {
      attributes.add(keyValue(attrLoggerName, name));
    }
    if (fields != null) {
      for (final entry in fields.entries) {
        final k = entry.key;
        if (k.isEmpty) continue;
        if (redact.contains(k)) {
          attributes.add(keyValue(k, '<redacted>'));
        } else {
          attributes.add(keyValue(k, entry.value));
        }
      }
    }
    final caller = _callerFrame();
    if (caller.isNotEmpty) {
      attributes.add(keyValue(attrCodeCaller, caller));
    }
    final now = _nowUnixNano();
    _obs.logRing?.push(LogRecord(
      record: obs_pb.LogRecord(
        timeUnixNano: now,
        observedTimeUnixNano: now,
        severityNumber: _levelToSeverity(lvl),
        severityText: lvl.name.toUpperCase(),
        body: anyValue(message),
        attributes: attributes,
      ),
      private: private,
    ));
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
  final DateTime startedAt;
  final Map<String, Logger> _loggers = {};

  Observability._(this.cfg, this.families)
      : logRing =
            families.contains(Family.logs) ? LogRing(cfg.logsRingSize) : null,
        eventBus = families.contains(Family.events)
            ? EventBus(cfg.eventsRingSize)
            : null,
        registry = families.contains(Family.metrics) ? Registry() : null,
        startedAt = DateTime.now();

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

  void emit(String eventName,
      {Map<String, dynamic>? payload, bool private = false}) {
    if (eventBus == null) return;
    final redact = Set<String>.from(cfg.redactedFields);
    final attributes = resourceAttributes(cfg);
    if (payload != null) {
      for (final entry in payload.entries) {
        if (entry.key.isEmpty) continue;
        if (redact.contains(entry.key)) {
          attributes.add(keyValue(entry.key, '<redacted>'));
        } else {
          attributes.add(keyValue(entry.key, entry.value));
        }
      }
    }
    final now = _nowUnixNano();
    eventBus!.emit(LogRecord(
      record: obs_pb.LogRecord(
        timeUnixNano: now,
        observedTimeUnixNano: now,
        severityNumber: obs_pb.SeverityNumber.SEVERITY_NUMBER_INFO,
        severityText: 'INFO',
        body: anyValue(eventName),
        attributes: attributes,
        eventName: eventName,
      ),
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

  StreamSubscription<obs_pb.LogRecord>? _logsSub;
  StreamSubscription<obs_pb.LogRecord>? _eventsSub;
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

  void _relayLog(obs_pb.LogRecord proto) {
    final obs = current();
    if (!obs.enabled(Family.logs) || obs.logRing == null) {
      return;
    }
    final record = fromProtoLogRecord(proto);
    _refreshChildIdentity(record.slug, record.instanceUid, record.chain);
    obs.logRing!.push(_withAppendedChain(record, childSlug, childUid));
  }

  void _relayEvent(obs_pb.LogRecord proto) {
    final obs = current();
    if (!obs.enabled(Family.events) || obs.eventBus == null) {
      return;
    }
    final record = fromProtoLogRecord(proto);
    _refreshChildIdentity(record.slug, record.instanceUid, record.chain);
    obs.eventBus!.emit(_withAppendedChain(record, childSlug, childUid));
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

Observability configure(Config cfg, {Map<String, String>? env}) {
  env ??= Platform.environment;
  checkEnv(env);
  final families = parseOpObs(env['OP_OBS'] ?? '');
  final slug = cfg.slug;
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

Int64 _nowUnixNano() =>
    Int64(DateTime.now().microsecondsSinceEpoch) * Int64(1000);

obs_pb.SeverityNumber _levelToSeverity(Level level) {
  return obs_pb.SeverityNumber.valueOf(level.value) ??
      obs_pb.SeverityNumber.SEVERITY_NUMBER_UNSPECIFIED;
}

obs_pb.AnyValue anyValue(dynamic value) {
  if (value is int) {
    return obs_pb.AnyValue(intValue: Int64(value));
  }
  if (value is double) {
    return obs_pb.AnyValue(doubleValue: value);
  }
  if (value is bool) {
    return obs_pb.AnyValue(boolValue: value);
  }
  if (value is String) {
    return obs_pb.AnyValue(stringValue: value);
  }
  return obs_pb.AnyValue(stringValue: value?.toString() ?? '');
}

String anyValueString(obs_pb.AnyValue value) {
  switch (value.whichValue()) {
    case obs_pb.AnyValue_Value.stringValue:
      return value.stringValue;
    case obs_pb.AnyValue_Value.boolValue:
      return value.boolValue ? 'true' : 'false';
    case obs_pb.AnyValue_Value.intValue:
      return value.intValue.toString();
    case obs_pb.AnyValue_Value.doubleValue:
      return value.doubleValue.toString();
    case obs_pb.AnyValue_Value.notSet:
      return '';
  }
}

obs_pb.KeyValue keyValue(String key, dynamic value) =>
    obs_pb.KeyValue(key: key, value: anyValue(value));

List<obs_pb.KeyValue> resourceAttributes(Config cfg, {String sessionId = ''}) {
  return [
    keyValue(attrHolonsSlug, cfg.slug),
    keyValue(attrServiceName, cfg.slug),
    keyValue(attrHolonsInstanceUid, cfg.instanceUid),
    keyValue(attrServiceInstanceId, cfg.instanceUid),
    keyValue(attrHolonsSessionId, sessionId),
  ];
}

String stringAttribute(Iterable<obs_pb.KeyValue> attributes, String key) {
  for (final attr in attributes) {
    if (attr.key == key) {
      return anyValueString(attr.value);
    }
  }
  return '';
}

Map<String, String> userAttributesMap(Iterable<obs_pb.KeyValue> attributes) {
  final out = <String, String>{};
  for (final attr in attributes) {
    if (!_isSystemAttribute(attr.key)) {
      out[attr.key] = anyValueString(attr.value);
    }
  }
  return Map.unmodifiable(out);
}

bool _isSystemAttribute(String key) {
  return key == attrHolonsSlug ||
      key == attrServiceName ||
      key == attrHolonsInstanceUid ||
      key == attrServiceInstanceId ||
      key == attrHolonsSessionId ||
      key == attrHolonsTransport ||
      key == attrRpcMethod ||
      key == attrLoggerName ||
      key == attrCodeCaller;
}

obs_pb.LogRecord toProtoLogRecord(LogRecord record) {
  return obs_pb.LogRecord.fromBuffer(record.record.writeToBuffer());
}

LogRecord fromProtoLogRecord(obs_pb.LogRecord record) {
  return LogRecord(record: record);
}

String _hopToString(Hop hop) => '${hop.slug}/${hop.instanceUid}';

Hop _hopFromString(String value) {
  final index = value.lastIndexOf('/');
  if (index < 0) {
    return Hop(slug: value, instanceUid: '');
  }
  return Hop(
    slug: value.substring(0, index),
    instanceUid: value.substring(index + 1),
  );
}

LogRecord _withAppendedChain(
    LogRecord record, String childSlug, String childUid) {
  final clone = toProtoLogRecord(record);
  clone.chain.add(_hopToString(Hop(slug: childSlug, instanceUid: childUid)));
  return LogRecord(record: clone, private: record.private);
}

List<obs_pb.KeyValue> _metricAttributes(
  Config cfg,
  Map<String, String> labels,
) {
  final attrs = resourceAttributes(cfg);
  final keys = labels.keys.toList()..sort();
  for (final key in keys) {
    attrs.add(keyValue(key, labels[key] ?? ''));
  }
  return attrs;
}

List<Int64> _histogramBucketCounts(HistogramSnapshot snapshot) {
  final counts = <Int64>[];
  var previous = 0;
  for (final cumulative in snapshot.counts) {
    final delta = max(0, cumulative - previous);
    counts.add(Int64(delta));
    previous = cumulative;
  }
  counts.add(Int64(max(0, snapshot.total - previous)));
  return counts;
}

List<obs_pb.Metric> toProtoMetrics(Observability obs) {
  final registry = obs.registry;
  if (registry == null) return const [];
  final start = Int64(obs.startedAt.microsecondsSinceEpoch) * Int64(1000);
  final now = _nowUnixNano();
  return [
    for (final counter in registry.listCounters())
      obs_pb.Metric(
        name: counter.name,
        description: counter.help,
        sum: obs_pb.Sum(
          aggregationTemporality:
              obs_pb.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE,
          isMonotonic: true,
          dataPoints: [
            obs_pb.NumberDataPoint(
              startTimeUnixNano: start,
              timeUnixNano: now,
              asInt: Int64(counter.value()),
              attributes: _metricAttributes(obs.cfg, counter.labels),
            ),
          ],
        ),
      ),
    for (final gauge in registry.listGauges())
      obs_pb.Metric(
        name: gauge.name,
        description: gauge.help,
        gauge: obs_pb.Gauge(
          dataPoints: [
            obs_pb.NumberDataPoint(
              startTimeUnixNano: start,
              timeUnixNano: now,
              asDouble: gauge.value(),
              attributes: _metricAttributes(obs.cfg, gauge.labels),
            ),
          ],
        ),
      ),
    for (final histogram in registry.listHistograms())
      obs_pb.Metric(
        name: histogram.name,
        description: histogram.help,
        histogram: obs_pb.Histogram(
          aggregationTemporality:
              obs_pb.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE,
          dataPoints: [
            obs_pb.HistogramDataPoint(
              startTimeUnixNano: start,
              timeUnixNano: now,
              count: Int64(histogram.snapshot().total),
              sum: histogram.snapshot().sum,
              bucketCounts: _histogramBucketCounts(histogram.snapshot()),
              explicitBounds: histogram.snapshot().bounds,
              attributes: _metricAttributes(obs.cfg, histogram.labels),
              min: histogram.snapshot().min,
              max: histogram.snapshot().max,
            ),
          ],
        ),
      ),
  ];
}

class HolonObservabilityService extends obs_grpc.HolonObservabilityServiceBase {
  final Observability obs;
  HolonObservabilityService([Observability? obs]) : obs = obs ?? current();

  @override
  Stream<obs_pb.LogRecord> logs(
      ServiceCall call, obs_pb.LogsRequest request) async* {
    if (!obs.enabled(Family.logs) || obs.logRing == null) {
      throw GrpcError.failedPrecondition('logs family is not enabled (OP_OBS)');
    }
    final minLevel =
        request.hasMinSeverityNumber() && request.minSeverityNumber.value != 0
            ? request.minSeverityNumber.value
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
          yield toProtoLogRecord(entry);
        }
      }
      if (!request.follow) return;
      await for (final entry in followed!.live) {
        if (!entry.private &&
            _matchLog(
                entry, minLevel, request.sessionIds, request.rpcMethods)) {
          yield toProtoLogRecord(entry);
        }
      }
    } finally {
      await followed?.stop();
    }
  }

  @override
  Stream<obs_pb.Metric> metrics(
      ServiceCall call, obs_pb.MetricsRequest request) async* {
    if (!obs.enabled(Family.metrics) || obs.registry == null) {
      throw GrpcError.failedPrecondition(
          'metrics family is not enabled (OP_OBS)');
    }
    var metrics = toProtoMetrics(obs);
    if (request.namePrefixes.isNotEmpty) {
      metrics = metrics
          .where((metric) => request.namePrefixes
              .any((prefix) => metric.name.startsWith(prefix)))
          .toList();
    }
    for (final metric in metrics) {
      yield metric;
    }
  }

  @override
  Stream<obs_pb.LogRecord> events(
      ServiceCall call, obs_pb.EventsRequest request) async* {
    if (!obs.enabled(Family.events) || obs.eventBus == null) {
      throw GrpcError.failedPrecondition(
          'events family is not enabled (OP_OBS)');
    }
    final wanted = request.eventNames.toSet();
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
          yield toProtoLogRecord(event);
      }
      if (!request.follow) return;
      await for (final event in followed!.live) {
        if (!event.private && _matchEvent(event, wanted)) {
          yield toProtoLogRecord(event);
        }
      }
    } finally {
      await followed?.stop();
    }
  }
}

Service registerService([Observability? obs]) => HolonObservabilityService(obs);

bool _matchLog(LogRecord entry, int minLevel, List<String> sessionIds,
    List<String> rpcMethods) {
  if (entry.level.value < minLevel) return false;
  if (sessionIds.isNotEmpty && !sessionIds.contains(entry.sessionId))
    return false;
  if (rpcMethods.isNotEmpty && !rpcMethods.contains(entry.rpcMethod))
    return false;
  return true;
}

bool _matchEvent(LogRecord event, Set<String> wanted) {
  if (wanted.isEmpty) return true;
  return wanted.contains(event.eventName);
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
        'event_name': e.eventName,
        'slug': e.slug,
        'instance_uid': e.instanceUid,
      };
      if (e.sessionId.isNotEmpty) rec['session_id'] = e.sessionId;
      if (e.fields.isNotEmpty) rec['payload'] = e.fields;
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
