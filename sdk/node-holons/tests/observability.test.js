'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const EventEmitter = require('node:events');
const fs = require('node:fs');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const grpc = require('@grpc/grpc-js');

const obs = require('../src/observability');
const describe = require('../src/describe');
const serve = require('../src/serve');
const transport = require('../src/transport');
const observabilityWire = require('../src/gen/holons/v1/observability');
const { sampleStaticDescribeResponse } = require('./helpers/static_describe');

test.beforeEach(() => {
  obs.reset();
  delete process.env.OP_OBS;
  delete process.env.OP_RUN_DIR;
  delete process.env.OP_INSTANCE_UID;
  describe.useStaticResponse(null);
});

test('parseOpObs basic', () => {
  const cases = [
    ['', new Set()],
    ['logs', new Set([obs.Family.LOGS])],
    ['logs,metrics', new Set([obs.Family.LOGS, obs.Family.METRICS])],
    ['all', new Set([obs.Family.LOGS, obs.Family.METRICS, obs.Family.EVENTS, obs.Family.PROM])],
  ];
  for (const [input, want] of cases) {
    const got = obs.parseOpObs(input);
    assert.equal(got.size, want.size, input);
    for (const f of want) assert.ok(got.has(f), `${input}: missing ${f}`);
  }
  for (const input of ['all,otel', 'all,sessions', 'unknown']) {
    assert.throws(() => obs.parseOpObs(input), obs.InvalidTokenError);
  }
});

test('checkEnv rejects unknown OP_OBS tokens', () => {
  process.env.OP_OBS = 'logs,otel';
  assert.throws(() => obs.checkEnv(), obs.InvalidTokenError);

  process.env.OP_OBS = 'bogus';
  assert.throws(() => obs.checkEnv(), obs.InvalidTokenError);

  process.env.OP_OBS = 'logs,sessions';
  assert.throws(() => obs.checkEnv(), obs.InvalidTokenError);

  process.env.OP_OBS = 'logs,metrics,events,prom,all';
  obs.checkEnv();
});

test('disabled is no-op', () => {
  const o = obs.configure({ slug: 't' });
  assert.equal(o.enabled(obs.Family.LOGS), false);
  o.logger('x').info('drop', { k: 'v' });
  assert.equal(o.counter('t_total'), null);
});

test('logs ring emits LogRecord with typed AnyValue attributes', () => {
  process.env.OP_OBS = 'logs';
  const o = obs.configure({ slug: 'g', instanceUid: 'uid', redactedFields: ['password'] });
  const l = o.logger('r');
  l.info('hello', {
    who: 'bob',
    ok: true,
    count: 3,
    ratio: 1.5,
    object: { value: 1 },
    password: 'secret',
  });
  const entries = o.logRing.drain();
  assert.equal(entries.length, 1);
  const record = entries[0];
  assert.equal(obs.bodyString(record), 'hello');
  assert.equal(record.severity_number, obs.Level.INFO);
  assert.equal(record.severity_text, 'INFO');
  assert.equal(obs.stringAttribute(record, obs.Attr.HOLONS_SLUG), 'g');
  assert.equal(obs.stringAttribute(record, obs.Attr.SERVICE_NAME), 'g');
  assert.equal(obs.stringAttribute(record, obs.Attr.HOLONS_INSTANCE_UID), 'uid');
  assert.equal(obs.stringAttribute(record, obs.Attr.SERVICE_INSTANCE_ID), 'uid');
  assert.equal(obs.stringAttribute(record, 'who'), 'bob');
  assert.deepEqual(attr(record, 'ok').value, { bool_value: true });
  assert.deepEqual(attr(record, 'count').value, { int_value: 3 });
  assert.deepEqual(attr(record, 'ratio').value, { double_value: 1.5 });
  assert.deepEqual(attr(record, 'object').value, { string_value: '[object Object]' });
  assert.deepEqual(attr(record, 'password').value, { string_value: '<redacted>' });
});

test('logs include current session context attributes when available', () => {
  process.env.OP_OBS = 'logs';
  const o = obs.configure({ slug: 'g', instanceUid: 'uid' });
  obs.withSessionContext({ sessionId: 'session-1', rpcMethod: '/greeting.v1.GreetingService/SayHello' }, () => {
    o.logger('r').info('inside session');
  });
  const record = o.logRing.drain()[0];
  assert.equal(obs.stringAttribute(record, obs.Attr.HOLONS_SESSION_ID), 'session-1');
  assert.equal(obs.stringAttribute(record, obs.Attr.RPC_METHOD), '/greeting.v1.GreetingService/SayHello');
});

test('counter increments and histogram percentile', () => {
  process.env.OP_OBS = 'metrics';
  const o = obs.configure({ slug: 'g' });
  const c = o.counter('t_total');
  for (let i = 0; i < 1000; i++) c.inc();
  assert.equal(c.value(), 1000);

  const h = o.histogram('lat_s', '', {}, [1e-3, 1e-2, 1e-1, 1.0]);
  for (let i = 0; i < 900; i++) h.observe(0.5e-3);
  for (let i = 0; i < 100; i++) h.observe(0.5);
  const snap = h.snapshot();
  assert.equal(obs.Histogram.quantile(snap, 0.5), 1e-3);
  assert.equal(obs.Histogram.quantile(snap, 0.99), 1.0);
});

test('event bus fan-out', () => {
  process.env.OP_OBS = 'events';
  const o = obs.configure({ slug: 'g', instanceUid: 'uid' });
  const received = [];
  o.eventBus.subscribe((e) => received.push(e));
  o.emit(obs.EventName.INSTANCE_READY, { listener: 'stdio://' });
  assert.equal(received.length, 1);
  assert.equal(received[0].event_name, obs.EventName.INSTANCE_READY);
  assert.equal(obs.stringAttribute(received[0], 'listener'), 'stdio://');
});

test('chain append and enrichment', () => {
  const c1 = obs.appendDirectChild([], 'gabriel-greeting-rust', '1c2d');
  assert.equal(c1.length, 1);
  assert.equal(c1[0], 'gabriel-greeting-rust');
  const c2 = obs.enrichForMultilog(c1, 'gabriel-greeting-go', 'ea34');
  assert.equal(c2.length, 2);
  assert.equal(c2[1], 'gabriel-greeting-go');
  assert.equal(c1.length, 1); // original unchanged
});

test('isOrganismRoot', () => {
  const o1 = obs.configure({ slug: 'g', instanceUid: 'x', organismUid: 'x' });
  assert.equal(o1.isOrganismRoot(), true);
  obs.reset();
  const o2 = obs.configure({ slug: 'g', instanceUid: 'x', organismUid: 'y' });
  assert.equal(o2.isOrganismRoot(), false);
});

test('current never returns null', () => {
  const c = obs.current();
  assert.ok(c);
  c.logger('x').info('safe'); // must not throw
});

test('run dir derives from registry root', () => {
  process.env.OP_OBS = 'logs';
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'js-obs-root-'));
  const o = obs.configure({ slug: 'gabriel', instanceUid: 'uid-1', runDir: root });
  assert.equal(o.cfg.runDir, path.join(root, 'gabriel', 'uid-1'));
});

test('disk writers and meta.json use instance run dir', () => {
  process.env.OP_OBS = 'logs,events';
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'js-obs-disk-'));
  const o = obs.configure({ slug: 'gabriel', instanceUid: 'uid-1', runDir: root });
  obs.enableDiskWriters(o.cfg.runDir);
  o.logger('test').info('ready', { port: 123 });
  o.emit(obs.EventName.INSTANCE_READY, { listener: 'tcp://127.0.0.1:123' });
  obs.writeMetaJson(o.cfg.runDir, {
    slug: 'gabriel',
    uid: 'uid-1',
    pid: 42,
    started_at: new Date(1000).toISOString(),
    mode: 'persistent',
    transport: 'tcp',
    address: 'tcp://127.0.0.1:123',
    log_path: path.join(o.cfg.runDir, 'stdout.log'),
  });

  assert.match(fs.readFileSync(path.join(root, 'gabriel', 'uid-1', 'stdout.log'), 'utf8'), /ready/);
  assert.match(fs.readFileSync(path.join(root, 'gabriel', 'uid-1', 'events.jsonl'), 'utf8'), /instance\.ready/);
  const meta = obs.readMetaJson(o.cfg.runDir);
  assert.equal(meta.slug, 'gabriel');
  assert.equal(meta.uid, 'uid-1');
  assert.equal(meta.address, 'tcp://127.0.0.1:123');
});

test('HolonObservability handlers replay logs, metrics, and events', async () => {
  process.env.OP_OBS = 'logs,metrics,events';
  const o = obs.configure({ slug: 'gabriel', instanceUid: 'uid-1' });
  o.logger('test').info('hello');
  o.counter('requests_total', 'requests').inc();
  o.emit(obs.EventName.INSTANCE_READY, { listener: 'stdio://' });

  const handlers = obs.makeHolonObservabilityHandlers(o);
  const logs = new FakeStreamCall({ follow: false });
  handlers.Logs(logs);
  assert.deepEqual(logs.writes.map((entry) => obs.bodyString(entry)), ['hello']);
  assert.equal(logs.ended, true);

  const metrics = new FakeStreamCall({});
  handlers.Metrics(metrics);
  const requests = metrics.writes.find((metric) => metric.name === 'requests_total');
  assert.ok(requests);
  assert.equal(requests.sum.is_monotonic, true);
  assert.equal(requests.sum.aggregation_temporality, 'AGGREGATION_TEMPORALITY_CUMULATIVE');
  assert.equal(requests.sum.data_points[0].as_int, 1);
  assert.equal(obs.stringAttribute(requests.sum.data_points[0].attributes, obs.Attr.HOLONS_SLUG), 'gabriel');
  assert.equal(obs.stringAttribute(requests.sum.data_points[0].attributes, obs.Attr.HOLONS_INSTANCE_UID), 'uid-1');

  const events = new FakeStreamCall({ follow: false });
  handlers.Events(events);
  assert.deepEqual(events.writes.map((event) => event.event_name), [obs.EventName.INSTANCE_READY]);
});

test('metric oneofs encode counters, gauges, and histograms', () => {
  process.env.OP_OBS = 'metrics';
  const o = obs.configure({ slug: 'gabriel', instanceUid: 'uid-1' });
  o.counter('requests_total', 'requests').inc(2);
  o.gauge('temperature_celsius', 'temperature').set(21.5);
  const histogram = o.histogram('latency_seconds', 'latency', {}, [0.1, 1.0]);
  histogram.observe(0.05);
  histogram.observe(2.0);

  const metrics = obs.toProtoMetrics(o.registry.snapshot(), o.cfg, o.startUnixNano);
  const counter = metrics.find((metric) => metric.name === 'requests_total');
  const gauge = metrics.find((metric) => metric.name === 'temperature_celsius');
  const hist = metrics.find((metric) => metric.name === 'latency_seconds');
  assert.equal(counter.sum.data_points[0].as_int, 2);
  assert.equal(counter.sum.is_monotonic, true);
  assert.equal(gauge.gauge.data_points[0].as_double, 21.5);
  assert.deepEqual(hist.histogram.data_points[0].bucket_counts, [1, 0, 1]);
  assert.deepEqual(hist.histogram.data_points[0].explicit_bounds, [0.1, 1.0]);
  assert.equal(hist.histogram.data_points[0].count, 2);
  assert.equal(hist.histogram.data_points[0].sum, 2.05);
  assert.equal(hist.histogram.data_points[0].min, 0.05);
  assert.equal(hist.histogram.data_points[0].max, 2.0);
});

test('Logs(follow=true) replays ring before live entries', () => {
  process.env.OP_OBS = 'logs';
  const o = obs.configure({ slug: 'gabriel', instanceUid: 'uid-1' });
  o.logger('test').info('before');

  const handlers = obs.makeHolonObservabilityHandlers(o);
  const logs = new FakeStreamCall({ follow: true });
  handlers.Logs(logs);
  assert.deepEqual(logs.writes.map((entry) => obs.bodyString(entry)), ['before']);

  o.logger('test').info('after');
  assert.deepEqual(logs.writes.map((entry) => obs.bodyString(entry)), ['before', 'after']);
  logs.emit('close');
});

test('Events(follow=true) replays ring before live entries', () => {
  process.env.OP_OBS = 'events';
  const o = obs.configure({ slug: 'gabriel', instanceUid: 'uid-1' });
  o.emit(obs.EventName.INSTANCE_READY, { listener: 'stdio://' });

  const handlers = obs.makeHolonObservabilityHandlers(o);
  const events = new FakeStreamCall({ follow: true });
  handlers.Events(events);
  assert.deepEqual(events.writes.map((event) => event.event_name), [obs.EventName.INSTANCE_READY]);

  o.emit(obs.EventName.CONFIG_RELOADED, { source: 'test' });
  assert.deepEqual(events.writes.map((event) => event.event_name), [obs.EventName.INSTANCE_READY, obs.EventName.CONFIG_RELOADED]);
  events.emit('close');
});

test('private log and event emissions remain local but not follow-streamed', () => {
  process.env.OP_OBS = 'logs,events';
  const o = obs.configure({ slug: 'gabriel', instanceUid: 'uid-1' });
  o.logger('test').private().info('secret');
  o.logger('test').info('public');
  o.emitPrivate(obs.EventName.CONFIG_RELOADED, { source: 'secret' });
  o.emit(obs.EventName.INSTANCE_READY, { listener: 'stdio://' });

  const handlers = obs.makeHolonObservabilityHandlers(o);
  const logs = new FakeStreamCall({ follow: true });
  handlers.Logs(logs);
  assert.deepEqual(o.logRing.drain().map((entry) => obs.bodyString(entry)), ['secret', 'public']);
  assert.deepEqual(logs.writes.map((entry) => obs.bodyString(entry)), ['public']);
  logs.emit('close');

  const events = new FakeStreamCall({ follow: true });
  handlers.Events(events);
  assert.equal(o.eventBus.drain().length, 2);
  assert.deepEqual(events.writes.map((event) => event.event_name), [obs.EventName.INSTANCE_READY]);
  events.emit('close');
});

test('Prometheus exposition includes injected runtime labels', async () => {
  process.env.OP_OBS = 'all';
  const o = obs.configure({ slug: 'node-prom', instanceUid: 'uid-prom' });
  o.counter('cascade_ticks_total', 'Ticks received by this cascade node.', { responder_uid: 'uid-prom' }).inc();

  const text = obs.toPrometheusText(o);
  assert.match(text, /# HELP cascade_ticks_total Ticks received by this cascade node\./);
  assert.ok(text.includes('cascade_ticks_total{instance_uid="uid-prom",responder_uid="uid-prom",slug="node-prom"} 1'));

  const prom = new obs.PromServer('127.0.0.1:0');
  try {
    const addr = await prom.start();
    const body = await httpGet(addr);
    assert.match(body, /cascade_ticks_total/);
  } finally {
    await prom.close();
  }
});

test('MemberRelay forwards logs and events with child chain hop', () => {
  process.env.OP_OBS = 'logs,events';
  const parent = obs.configure({ slug: 'parent-node', instanceUid: 'parent-uid' });
  const logStream = new EventEmitter();
  const eventStream = new EventEmitter();
  logStream.cancel = () => {};
  eventStream.cancel = () => {};
  const relay = new obs.MemberRelay({
    childSlug: 'child-node',
    childUid: 'child-uid',
    client: {
      Logs() { return logStream; },
      Events() { return eventStream; },
    },
    observability: parent,
    retryDelayMs: 10,
  });
  relay.start();
  logStream.emit('data', {
    time_unix_nano: '1000000000',
    observed_time_unix_nano: '1000000000',
    severity_number: obs.Level.INFO,
    severity_text: 'INFO',
    body: { string_value: 'relay-log' },
    attributes: [
      obs.keyValue(obs.Attr.HOLONS_SLUG, 'child-node'),
      obs.keyValue(obs.Attr.HOLONS_INSTANCE_UID, 'child-uid'),
    ],
    chain: [],
  });
  eventStream.emit('data', {
    time_unix_nano: '1000000000',
    observed_time_unix_nano: '1000000000',
    severity_number: obs.Level.INFO,
    severity_text: 'INFO',
    body: { string_value: obs.EventName.INSTANCE_READY },
    event_name: obs.EventName.INSTANCE_READY,
    attributes: [
      obs.keyValue(obs.Attr.HOLONS_SLUG, 'child-node'),
      obs.keyValue(obs.Attr.HOLONS_INSTANCE_UID, 'child-uid'),
    ],
    chain: [],
  });
  relay.stop();

  assert.deepEqual(parent.logRing.drain()[0].chain, ['child-node']);
  assert.deepEqual(parent.eventBus.drain()[0].chain, ['child-node']);
});

test('serve auto-registers HolonObservability when OP_OBS is set', async (t) => {
  if (!await canListenOnLoopback()) {
    t.skip('socket bind not permitted in this environment');
    return;
  }

  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'js-obs-serve-'));
  process.env.OP_OBS = 'logs,metrics,events';
  process.env.OP_RUN_DIR = root;
  process.env.OP_INSTANCE_UID = 'uid-1';
  describe.useStaticResponse(sampleStaticDescribeResponse());

  const server = await serve.runWithOptions('tcp://127.0.0.1:0', () => {}, {
    reflect: false,
    logger: quietLogger(),
  });
  t.after(async () => {
    await server.stopHolon();
    describe.useStaticResponse(null);
  });

  const current = obs.current();
  current.logger('test').info('served');
  current.counter('requests_total', 'requests').inc();
  current.emit(obs.EventName.CONFIG_RELOADED, { source: 'test' });

  const parsed = transport.parseURI(server.__holonsRuntime.publicURI);
  const client = new ObservabilityClient(
    `${parsed.host === '0.0.0.0' ? '127.0.0.1' : parsed.host}:${parsed.port}`,
    grpc.credentials.createInsecure(),
  );
  t.after(() => client.close());

  const logs = await collectStream(client.Logs.bind(client), { follow: false });
  assert.ok(logs.some((entry) => obs.bodyString(entry) === 'served'));

  const metrics = await collectStream(client.Metrics.bind(client), {});
  assert.ok(metrics.some((metric) => metric.name === 'requests_total' && metric.sum?.data_points?.[0]?.as_int === '1'));

  const events = await collectStream(client.Events.bind(client), { follow: false });
  assert.ok(events.some((event) => event.event_name === obs.EventName.INSTANCE_READY));
  assert.ok(events.some((event) => event.event_name === obs.EventName.CONFIG_RELOADED));

  const metaPath = path.join(root, current.cfg.slug, 'uid-1', 'meta.json');
  const meta = JSON.parse(fs.readFileSync(metaPath, 'utf8'));
  assert.equal(meta.uid, 'uid-1');
  assert.equal(meta.address, server.__holonsRuntime.publicURI);
});

class FakeStreamCall extends EventEmitter {
  constructor(request) {
    super();
    this.request = request;
    this.writes = [];
    this.ended = false;
  }
  write(value) {
    this.writes.push(value);
  }
  end() {
    this.ended = true;
    this.emit('close');
  }
}

const ObservabilityClient = grpc.makeGenericClientConstructor(
  observabilityWire.HOLON_OBSERVABILITY_SERVICE_DEF,
  'HolonObservability',
  {},
);

function attr(record, key) {
  const found = (record.attributes || []).find((item) => item.key === key);
  assert.ok(found, `missing attribute ${key}`);
  return found;
}

function quietLogger() {
  return {
    error() {},
    warn() {},
  };
}

function canListenOnLoopback() {
  return new Promise((resolve) => {
    const probe = net.createServer();
    probe.once('error', (err) => {
      const code = err && err.code;
      resolve(code !== 'EPERM' && code !== 'EACCES');
    });
    probe.listen(0, '127.0.0.1', () => {
      probe.close(() => resolve(true));
    });
  });
}

function collectStream(method, request) {
  return new Promise((resolve, reject) => {
    const out = [];
    const stream = method(request);
    stream.on('data', (entry) => out.push(entry));
    stream.on('error', reject);
    stream.on('end', () => resolve(out));
  });
}

function httpGet(addr) {
  return new Promise((resolve, reject) => {
    const req = require('node:http').get(addr, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => { body += chunk; });
      res.on('end', () => resolve(body));
    });
    req.on('error', reject);
  });
}
