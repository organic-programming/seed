import 'package:test/test.dart';
import 'package:holons/src/observability.dart' as obs;

void main() {
  setUp(() {
    obs.reset();
  });

  test('parseOpObs basic', () {
    expect(obs.parseOpObs(''), isEmpty);
    expect(obs.parseOpObs('logs'), equals({obs.Family.logs}));
    expect(obs.parseOpObs('logs,metrics'),
        equals({obs.Family.logs, obs.Family.metrics}));
    expect(
      obs.parseOpObs('all'),
      equals({obs.Family.logs, obs.Family.metrics, obs.Family.events, obs.Family.prom}),
    );
    expect(obs.parseOpObs('unknown'), isEmpty);
    expect(
      obs.parseOpObs('all,otel'),
      equals({obs.Family.logs, obs.Family.metrics, obs.Family.events, obs.Family.prom}),
    );
    expect(
      obs.parseOpObs('all,sessions'),
      equals({obs.Family.logs, obs.Family.metrics, obs.Family.events, obs.Family.prom}),
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
      slug: 'g', instanceUid: 'x', organismUid: 'x',
    ));
    expect(o1.isOrganismRoot, isTrue);

    obs.reset();
    final o2 = obs.configure(const obs.Config(
      slug: 'g', instanceUid: 'x', organismUid: 'y',
    ));
    expect(o2.isOrganismRoot, isFalse);
  });
}
