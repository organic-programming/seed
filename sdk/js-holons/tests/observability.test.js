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
  delete process.env.OP_SESSIONS;
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
    ['all,otel', new Set([obs.Family.LOGS, obs.Family.METRICS, obs.Family.EVENTS, obs.Family.PROM])],
    ['all,sessions', new Set([obs.Family.LOGS, obs.Family.METRICS, obs.Family.EVENTS, obs.Family.PROM])],
    ['unknown', new Set()],
  ];
  for (const [input, want] of cases) {
    const got = obs.parseOpObs(input);
    assert.equal(got.size, want.size, input);
    for (const f of want) assert.ok(got.has(f), `${input}: missing ${f}`);
  }
});

test('checkEnv rejects otel and unknown', () => {
  process.env.OP_OBS = 'logs,otel';
  assert.throws(() => obs.checkEnv(), obs.InvalidTokenError);

  process.env.OP_OBS = 'bogus';
  assert.throws(() => obs.checkEnv(), obs.InvalidTokenError);

  process.env.OP_OBS = 'logs,sessions';
  assert.throws(() => obs.checkEnv(), obs.InvalidTokenError);

  process.env.OP_OBS = '';
  process.env.OP_SESSIONS = 'metrics';
  assert.throws(() => obs.checkEnv(), obs.InvalidTokenError);
  delete process.env.OP_SESSIONS;

  process.env.OP_OBS = 'logs,metrics,events,prom,all';
  obs.checkEnv();
});

test('disabled is no-op', () => {
  const o = obs.configure({ slug: 't' });
  assert.equal(o.enabled(obs.Family.LOGS), false);
  o.logger('x').info('drop', { k: 'v' });
  assert.equal(o.counter('t_total'), null);
});

test('logs ring + fields + redact', () => {
  process.env.OP_OBS = 'logs';
  const o = obs.configure({ slug: 'g', instanceUid: 'uid', redactedFields: ['password'] });
  const l = o.logger('r');
  l.info('hello', { who: 'bob', password: 'secret' });
  const entries = o.logRing.drain();
  assert.equal(entries.length, 1);
  assert.equal(entries[0].message, 'hello');
  assert.equal(entries[0].fields.who, 'bob');
  assert.equal(entries[0].fields.password, '<redacted>');
  assert.equal(entries[0].slug, 'g');
  assert.equal(entries[0].instance_uid, 'uid');
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
  o.emit(obs.EventType.INSTANCE_READY, { listener: 'stdio://' });
  assert.equal(received.length, 1);
  assert.equal(received[0].type, obs.EventType.INSTANCE_READY);
});

test('chain append and enrichment', () => {
  const c1 = obs.appendDirectChild([], 'gabriel-greeting-rust', '1c2d');
  assert.equal(c1.length, 1);
  assert.equal(c1[0].slug, 'gabriel-greeting-rust');
  const c2 = obs.enrichForMultilog(c1, 'gabriel-greeting-go', 'ea34');
  assert.equal(c2.length, 2);
  assert.equal(c2[1].slug, 'gabriel-greeting-go');
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
  o.emit(obs.EventType.INSTANCE_READY, { listener: 'tcp://127.0.0.1:123' });
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
  assert.match(fs.readFileSync(path.join(root, 'gabriel', 'uid-1', 'events.jsonl'), 'utf8'), /INSTANCE_READY/);
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
  o.emit(obs.EventType.INSTANCE_READY, { listener: 'stdio://' });

  const handlers = obs.makeHolonObservabilityHandlers(o);
  const logs = new FakeStreamCall({ follow: false });
  handlers.Logs(logs);
  assert.deepEqual(logs.writes.map((entry) => entry.message), ['hello']);
  assert.equal(logs.ended, true);

  const metrics = await new Promise((resolve, reject) => {
    handlers.Metrics({ request: {} }, (err, out) => (err ? reject(err) : resolve(out)));
  });
  assert.equal(metrics.slug, 'gabriel');
  assert.ok(metrics.samples.some((sample) => sample.name === 'requests_total' && sample.counter === 1));

  const events = new FakeStreamCall({ follow: false });
  handlers.Events(events);
  assert.deepEqual(events.writes.map((event) => event.type), ['INSTANCE_READY']);
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
  current.emit(obs.EventType.CONFIG_RELOADED, { source: 'test' });

  const parsed = transport.parseURI(server.__holonsRuntime.publicURI);
  const client = new ObservabilityClient(
    `${parsed.host === '0.0.0.0' ? '127.0.0.1' : parsed.host}:${parsed.port}`,
    grpc.credentials.createInsecure(),
  );
  t.after(() => client.close());

  const logs = await collectStream(client.Logs.bind(client), { follow: false });
  assert.ok(logs.some((entry) => entry.message === 'served'));

  const metrics = await unary(client.Metrics.bind(client), {});
  assert.ok(metrics.samples.some((sample) => sample.name === 'requests_total'));

  const events = await collectStream(client.Events.bind(client), { follow: false });
  assert.ok(events.some((event) => event.type === 'INSTANCE_READY'));
  assert.ok(events.some((event) => event.type === 'CONFIG_RELOADED'));

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

function unary(method, request) {
  return new Promise((resolve, reject) => {
    method(request, (err, out) => (err ? reject(err) : resolve(out || {})));
  });
}
