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
    expect(obs.parseOpObs('unknown'), isEmpty);
    expect(
      obs.parseOpObs('all,otel'),
      equals({
        obs.Family.logs,
        obs.Family.metrics,
        obs.Family.events,
        obs.Family.prom
      }),
    );
    expect(
      obs.parseOpObs('all,sessions'),
      equals({
        obs.Family.logs,
        obs.Family.metrics,
        obs.Family.events,
        obs.Family.prom
      }),
    );
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
