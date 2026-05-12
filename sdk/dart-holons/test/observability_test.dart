import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:holons/gen/holons/v1/manifest.pb.dart' as manifestpb;
import 'package:holons/gen/holons/v1/observability.pbgrpc.dart' as obsgrpc;
import 'package:holons/holons.dart'
    show ServeOptions, startWithOptions, useStaticResponse;
import 'package:test/test.dart';
import 'package:holons/src/observability.dart' as obs;

void main() {
  setUp(() {
    obs.reset();
    useStaticResponse(null);
  });

  test('parseOpObs basic', () {
    expect(obs.parseOpObs(''), isEmpty);
    expect(obs.parseOpObs('logs'), equals({obs.Family.logs}));
    expect(obs.parseOpObs('logs,metrics'),
        equals({obs.Family.logs, obs.Family.metrics}));
    expect(
      obs.parseOpObs('all'),
      equals({
        obs.Family.logs,
        obs.Family.metrics,
        obs.Family.events,
        obs.Family.prom
      }),
    );
    expect(
        () => obs.parseOpObs('unknown'), throwsA(isA<obs.InvalidTokenError>()));
    expect(() => obs.parseOpObs('all,otel'),
        throwsA(isA<obs.InvalidTokenError>()));
    expect(() => obs.parseOpObs('all,sessions'),
        throwsA(isA<obs.InvalidTokenError>()));
  });

  test('checkEnv rejects otel and unknown', () {
    expect(() => obs.checkEnv({'OP_OBS': 'logs,otel'}),
        throwsA(isA<obs.InvalidTokenError>()));
    expect(() => obs.checkEnv({'OP_OBS': 'logs,sessions'}),
        throwsA(isA<obs.InvalidTokenError>()));
    expect(() => obs.checkEnv({'OP_SESSIONS': 'metrics'}),
        throwsA(isA<obs.InvalidTokenError>()));
    expect(() => obs.checkEnv({'OP_OBS': 'bogus'}),
        throwsA(isA<obs.InvalidTokenError>()));
    expect(() => obs.checkEnv({'OP_OBS': 'logs,metrics,events,prom,all'}),
        returnsNormally);
  });

  test('disabled is no-op', () {
    final o = obs.configure(const obs.Config(slug: 't'));
    expect(o.enabled(obs.Family.logs), isFalse);
    o.logger('x').info('drop', {'k': 'v'});
    expect(o.counter('t_total'), isNull);
  });

  test('counter and histogram (metrics family on)', () {
    // parseOpObs reads Platform.environment; we simulate by forcing a
    // registry via Registry directly, the path most tests exercise.
    final reg = obs.Registry();
    final c = reg.counter('t_total');
    for (var i = 0; i < 1000; i++) c.inc();
    expect(c.value(), equals(1000));

    final h = reg.histogram('lat_s', bounds: [1e-3, 1e-2, 1e-1, 1.0]);
    for (var i = 0; i < 900; i++) h.observe(0.5e-3);
    for (var i = 0; i < 100; i++) h.observe(0.5);
    final snap = h.snapshot();
    expect(snap.quantile(0.5), equals(1e-3));
    expect(snap.quantile(0.99), equals(1.0));
  });

  test('LogRing retention + ordering', () {
    final ring = obs.LogRing(3);
    for (var i = 0; i < 5; i++) {
      ring.push(obs.LogEntry(
        timestamp: DateTime.now(),
        level: obs.Level.info,
        slug: 'g',
        instanceUid: '',
        message: String.fromCharCode('a'.codeUnitAt(0) + i),
      ));
    }
    final entries = ring.drain();
    expect(entries.length, equals(3));
    expect(entries.first.message, equals('c'));
    expect(entries.last.message, equals('e'));
  });

  test('EventBus fan-out', () async {
    final bus = obs.EventBus(16);
    final received = <obs.Event>[];
    final sub = bus.watch().listen(received.add);
    bus.emit(obs.Event(
      timestamp: DateTime.now(),
      type: obs.EventType.instanceReady,
      slug: 'g',
      instanceUid: 'uid',
    ));
    await Future.delayed(Duration(milliseconds: 10));
    expect(received, hasLength(1));
    expect(received.first.type, equals(obs.EventType.instanceReady));
    await sub.cancel();
  });

  test('Chain append + multilog enrichment', () {
    final c1 = obs.appendDirectChild([], 'gabriel-greeting-rust', '1c2d');
    expect(c1, hasLength(1));
    expect(c1.first.slug, equals('gabriel-greeting-rust'));

    final c2 = obs.enrichForMultilog(c1, 'gabriel-greeting-go', 'ea34');
    expect(c2, hasLength(2));
    expect(c2.last.slug, equals('gabriel-greeting-go'));
    expect(c1, hasLength(1)); // original unchanged
  });

  test('MemberRelay forwards logs/events with child chain enrichment',
      () async {
    final fake = await _startFakeObservabilityService();
    addTearDown(fake.close);
    final local = obs.configure(
      const obs.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs,events'},
    );
    final relay = obs.MemberRelay(
      childSlug: 'child-x',
      childUid: 'uid-123',
      channel: fake.channel,
      observability: local,
    );

    await relay.start();
    await _waitFor(() => fake.service.logsOpened == 1);
    await _waitFor(() => fake.service.eventsOpened == 1);
    expect(fake.service.lastLogsFollow, isTrue);
    expect(fake.service.lastEventsFollow, isTrue);

    fake.service.emitLog('one', obs.Level.info, {'k': 'v1'});
    fake.service.emitLog('two', obs.Level.warn, {'k': 'v2'});
    fake.service.emitEvent(obs.EventType.instanceReady);
    fake.service.emitEvent(obs.EventType.instanceExited);

    await _waitFor(() => local.logRing!.drain().length == 2);
    await _waitFor(() => local.eventBus!.drain().length == 2);
    await relay.stop();

    final logs = local.logRing!.drain();
    expect(logs.map((entry) => entry.message), equals(['one', 'two']));
    expect(logs.map((entry) => entry.fields['k']), equals(['v1', 'v2']));
    expect(logs.map((entry) => entry.level),
        equals([obs.Level.info, obs.Level.warn]));
    for (final entry in logs) {
      _expectChildHopAppended(entry.chain);
    }

    final events = local.eventBus!.drain();
    expect(events.map((event) => event.type),
        equals([obs.EventType.instanceReady, obs.EventType.instanceExited]));
    for (final event in events) {
      _expectChildHopAppended(event.chain);
    }
  });

  test('MemberRelay family gating short-circuits', () async {
    final fake = await _startFakeObservabilityService();
    addTearDown(fake.close);
    final local = obs.configure(
      const obs.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': ''},
    );
    final relay = obs.MemberRelay(
      childSlug: 'child-x',
      childUid: 'uid-123',
      channel: fake.channel,
      observability: local,
    );

    await relay.start();

    expect(relay.isRunning, isFalse);
    expect(fake.service.logsOpened, equals(0));
    expect(fake.service.eventsOpened, equals(0));
    expect(local.logRing, isNull);
    expect(local.eventBus, isNull);
  });

  test('MemberRelay stop halts forwarding', () async {
    final fake = await _startFakeObservabilityService();
    addTearDown(fake.close);
    final local = obs.configure(
      const obs.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs'},
    );
    final relay = obs.MemberRelay(
      childSlug: 'child-x',
      childUid: 'uid-123',
      channel: fake.channel,
      observability: local,
    );

    await relay.start();
    await _waitFor(() => fake.service.logsOpened == 1);
    fake.service.emitLog('before-stop', obs.Level.info, {'k': 'v1'});
    await _waitFor(() => local.logRing!.drain().length == 1);

    await relay.stop();
    fake.service.emitLog('after-stop', obs.Level.info, {'k': 'v2'});
    await Future<void>.delayed(const Duration(milliseconds: 50));

    expect(local.logRing!.drain().map((entry) => entry.message),
        equals(['before-stop']));
    expect(relay.isRunning, isFalse);
  });

  test('MemberRelay logs warning and stops on stream error', () async {
    final fake = await _startFakeObservabilityService();
    addTearDown(fake.close);
    final local = obs.configure(
      const obs.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs'},
    );
    var closed = 0;
    final relay = obs.MemberRelay(
      childSlug: 'child-x',
      childUid: 'uid-123',
      channel: fake.channel,
      observability: local,
      onClosed: () {
        closed += 1;
      },
    );

    await relay.start();
    await _waitFor(() => fake.service.logsOpened == 1);
    fake.service.failLogs(GrpcError.unavailable('stream failed'));

    await _waitFor(() => relay.isRunning == false);
    await _waitFor(() => local.logRing!.drain().any((entry) =>
        entry.level == obs.Level.warn &&
        entry.loggerName == 'member-relay' &&
        entry.fields['child_slug'] == 'child-x' &&
        entry.fields['child_uid'] == 'uid-123' &&
        (entry.fields['error'] ?? '').contains('stream failed')));
    expect(closed, equals(1));
  });

  test('MemberRelay stop does not invoke onClosed callback', () async {
    final fake = await _startFakeObservabilityService();
    addTearDown(fake.close);
    final local = obs.configure(
      const obs.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs'},
    );
    var closed = 0;
    final relay = obs.MemberRelay(
      childSlug: 'child-x',
      childUid: 'uid-123',
      channel: fake.channel,
      observability: local,
      onClosed: () {
        closed += 1;
      },
    );

    await relay.start();
    await _waitFor(() => fake.service.logsOpened == 1);
    await relay.stop();
    await Future<void>.delayed(const Duration(milliseconds: 50));

    expect(relay.isRunning, isFalse);
    expect(closed, equals(0));
  });

  test('MemberRelay stop is idempotent', () async {
    final fake = await _startFakeObservabilityService();
    addTearDown(fake.close);
    final local = obs.configure(
      const obs.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs'},
    );
    final relay = obs.MemberRelay(
      childSlug: 'child-x',
      childUid: 'uid-123',
      channel: fake.channel,
      observability: local,
    );

    await relay.start();
    await _waitFor(() => fake.service.logsOpened == 1);
    await relay.stop();
    await relay.stop();

    expect(relay.isRunning, isFalse);
    expect(local.logRing!.drain(), isEmpty);
  });

  test('MemberRelay start remains no-op when families stay disabled', () async {
    final fake = await _startFakeObservabilityService();
    addTearDown(fake.close);
    final local = obs.configure(
      const obs.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': ''},
    );
    final relay = obs.MemberRelay(
      childSlug: 'child-x',
      childUid: 'uid-123',
      channel: fake.channel,
      observability: local,
    );

    await relay.start();
    await relay.stop();
    await relay.start();

    expect(relay.isRunning, isFalse);
    expect(fake.service.logsOpened, equals(0));
    expect(fake.service.eventsOpened, equals(0));
  });

  test('isOrganismRoot', () {
    final o1 = obs.configure(const obs.Config(
      slug: 'g',
      instanceUid: 'x',
      organismUid: 'x',
    ));
    expect(o1.isOrganismRoot, isTrue);

    obs.reset();
    final o2 = obs.configure(const obs.Config(
      slug: 'g',
      instanceUid: 'x',
      organismUid: 'y',
    ));
    expect(o2.isOrganismRoot, isFalse);
  });

  test('run dir derives from registry root', () {
    final root = Directory.systemTemp.createTempSync('dart-obs-root-');
    addTearDown(() => root.deleteSync(recursive: true));

    final o = obs.configure(
      obs.Config(slug: 'gabriel', instanceUid: 'uid-1', runDir: root.path),
      env: const {'OP_OBS': 'logs'},
    );

    expect(
        o.cfg.runDir,
        equals(
            '${root.path}${Platform.pathSeparator}gabriel${Platform.pathSeparator}uid-1'));
  });

  test('disk writers and meta.json use instance run dir', () async {
    final root = Directory.systemTemp.createTempSync('dart-obs-disk-');
    addTearDown(() => root.deleteSync(recursive: true));

    final o = obs.configure(
      obs.Config(slug: 'gabriel', instanceUid: 'uid-1', runDir: root.path),
      env: const {'OP_OBS': 'logs,events'},
    );
    obs.enableDiskWriters(o.cfg.runDir);
    o.logger('test').info('ready', {'port': 123});
    o.emit(obs.EventType.instanceReady,
        payload: {'listener': 'tcp://127.0.0.1:123'});
    await Future<void>.delayed(const Duration(milliseconds: 20));

    obs.writeMetaJson(
      o.cfg.runDir,
      obs.MetaJson(
        slug: 'gabriel',
        uid: 'uid-1',
        pid: 42,
        startedAt: DateTime.fromMillisecondsSinceEpoch(1000, isUtc: true),
        transport: 'tcp',
        address: 'tcp://127.0.0.1:123',
        logPath: '${o.cfg.runDir}${Platform.pathSeparator}stdout.log',
      ),
    );

    expect(
        File('${o.cfg.runDir}${Platform.pathSeparator}stdout.log')
            .readAsStringSync(),
        contains('ready'));
    expect(
        File('${o.cfg.runDir}${Platform.pathSeparator}events.jsonl')
            .readAsStringSync(),
        contains('INSTANCE_READY'));
    final meta = File('${o.cfg.runDir}${Platform.pathSeparator}meta.json')
        .readAsStringSync();
    expect(meta, contains('"slug": "gabriel"'));
    expect(meta, contains('"uid": "uid-1"'));
  });

  test('serve auto-registers HolonObservability when OP_OBS is set', () async {
    final root = Directory.systemTemp.createTempSync('dart-obs-serve-');
    addTearDown(() => root.deleteSync(recursive: true));
    useStaticResponse(_sampleDescribeResponse());

    final env = {
      'OP_OBS': 'logs,metrics,events',
      'OP_RUN_DIR': root.path,
      'OP_INSTANCE_UID': 'uid-1',
    };
    final running = await startWithOptions(
      'tcp://127.0.0.1:0',
      const [],
      options: ServeOptions(
        environment: env,
        logger: (_) {},
      ),
    );
    addTearDown(running.stop);

    final current = obs.current();
    current.logger('test').info('served');
    current.counter('requests_total', help: 'requests')!.inc();
    current.emit(obs.EventType.configReloaded, payload: {'source': 'test'});
    await Future<void>.delayed(const Duration(milliseconds: 20));

    final port = int.parse(running.publicUri.split(':').last);
    final channel = ClientChannel(
      '127.0.0.1',
      port: port,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
    addTearDown(channel.shutdown);

    final client = obsgrpc.HolonObservabilityClient(channel);
    final logs = await client.logs(obsgrpc.LogsRequest(follow: false)).toList();
    expect(logs.map((entry) => entry.message), contains('served'));

    final metrics = await client.metrics(obsgrpc.MetricsRequest());
    expect(metrics.samples.map((sample) => sample.name),
        contains('requests_total'));

    final events =
        await client.events(obsgrpc.EventsRequest(follow: false)).toList();
    expect(events.map((event) => event.type),
        contains(obsgrpc.EventType.INSTANCE_READY));
    expect(events.map((event) => event.type),
        contains(obsgrpc.EventType.CONFIG_RELOADED));

    final meta = File(
        '${root.path}${Platform.pathSeparator}${current.cfg.slug}${Platform.pathSeparator}uid-1${Platform.pathSeparator}meta.json');
    expect(meta.existsSync(), isTrue);
    expect(meta.readAsStringSync(), contains(running.publicUri));
  });

  test('serve reuses already configured observability runtime', () async {
    final root = Directory.systemTemp.createTempSync('dart-obs-serve-current-');
    addTearDown(() => root.deleteSync(recursive: true));
    useStaticResponse(_sampleDescribeResponse());

    final env = {
      'OP_OBS': 'logs,events',
      'OP_RUN_DIR': root.path,
      'OP_INSTANCE_UID': 'uid-1',
    };
    final configured = obs.fromEnv(
      obs.Config(slug: 'gabriel-greeting-app-flutter'),
      env,
    );
    final running = await startWithOptions(
      'tcp://127.0.0.1:0',
      const [],
      options: ServeOptions(
        environment: env,
        logger: (_) {},
      ),
    );
    addTearDown(running.stop);

    expect(obs.current(), same(configured));
    expect(obs.current().cfg.slug, equals('gabriel-greeting-app-flutter'));

    final meta = File(
        '${root.path}${Platform.pathSeparator}gabriel-greeting-app-flutter${Platform.pathSeparator}uid-1${Platform.pathSeparator}meta.json');
    expect(meta.existsSync(), isTrue);
    final payload = meta.readAsStringSync();
    expect(payload, contains('"slug": "gabriel-greeting-app-flutter"'));
    expect(payload, contains(running.publicUri));
  });
}

class _FakeObservabilityHarness {
  _FakeObservabilityHarness(this.service, this.server, this.channel);

  final _FakeObservabilityService service;
  final Server server;
  final ClientChannel channel;

  Future<void> close() async {
    await channel.shutdown();
    await service.close();
    await server.shutdown();
  }
}

class _FakeObservabilityService extends obsgrpc.HolonObservabilityServiceBase {
  final _logs = StreamController<obsgrpc.LogEntry>.broadcast();
  final _events = StreamController<obsgrpc.EventInfo>.broadcast();
  var logsOpened = 0;
  var eventsOpened = 0;
  bool? lastLogsFollow;
  bool? lastEventsFollow;

  @override
  Stream<obsgrpc.LogEntry> logs(ServiceCall call, obsgrpc.LogsRequest request) {
    logsOpened += 1;
    lastLogsFollow = request.follow;
    return _logs.stream;
  }

  @override
  Future<obsgrpc.MetricsSnapshot> metrics(
      ServiceCall call, obsgrpc.MetricsRequest request) async {
    return obsgrpc.MetricsSnapshot();
  }

  @override
  Stream<obsgrpc.EventInfo> events(
      ServiceCall call, obsgrpc.EventsRequest request) {
    eventsOpened += 1;
    lastEventsFollow = request.follow;
    return _events.stream;
  }

  void emitLog(String message, obs.Level level, Map<String, String> fields) {
    _logs.add(obs.toProtoLogEntry(obs.LogEntry(
      timestamp: DateTime.now().toUtc(),
      level: level,
      slug: 'origin',
      instanceUid: 'origin-uid',
      message: message,
      fields: fields,
      chain: const [obs.Hop(slug: 'origin', instanceUid: 'origin-uid')],
    )));
  }

  void emitEvent(obs.EventType type) {
    _events.add(obs.toProtoEvent(obs.Event(
      timestamp: DateTime.now().toUtc(),
      type: type,
      slug: 'origin',
      instanceUid: 'origin-uid',
      chain: const [obs.Hop(slug: 'origin', instanceUid: 'origin-uid')],
    )));
  }

  void failLogs(Object error) {
    _logs.addError(error);
  }

  Future<void> close() async {
    await _logs.close();
    await _events.close();
  }
}

Future<_FakeObservabilityHarness> _startFakeObservabilityService() async {
  final service = _FakeObservabilityService();
  final server = Server.create(services: [service]);
  await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
  final channel = ClientChannel(
    '127.0.0.1',
    port: server.port!,
    options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
  );
  return _FakeObservabilityHarness(service, server, channel);
}

Future<void> _waitFor(
  bool Function() condition, {
  Duration timeout = const Duration(seconds: 2),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (condition()) return;
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
  fail('condition was not met within $timeout');
}

void _expectChildHopAppended(List<obs.Hop> chain) {
  expect(chain, hasLength(2));
  expect(chain.first.slug, equals('origin'));
  expect(chain.first.instanceUid, equals('origin-uid'));
  expect(chain.last.slug, equals('child-x'));
  expect(chain.last.instanceUid, equals('uid-123'));
}

DescribeResponse _sampleDescribeResponse() {
  return DescribeResponse()
    ..manifest = (manifestpb.HolonManifest()
      ..identity = (manifestpb.HolonManifest_Identity()
        ..schema = 'holon/v1'
        ..uuid = 'dart-observability-0000'
        ..givenName = 'Dart'
        ..familyName = 'Observability'
        ..motto = 'Static describe fixture.'
        ..composer = 'dart-observability-test'
        ..status = 'draft'
        ..born = '2026-03-23')
      ..lang = 'dart')
    ..services.add(
      ServiceDoc()
        ..name = 'fixture.v1.Empty'
        ..description = 'Static fixture service.'
        ..methods.add(
          MethodDoc()
            ..name = 'Ping'
            ..description = 'No-op fixture method.',
        ),
    );
}
