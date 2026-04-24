'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const obs = require('../src/observability');

test.beforeEach(() => {
  obs.reset();
  delete process.env.OP_OBS;
  delete process.env.OP_SESSIONS;
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
