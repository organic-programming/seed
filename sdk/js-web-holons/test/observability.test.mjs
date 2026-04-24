import test from 'node:test';
import assert from 'node:assert/strict';
import WebSocket from 'ws';

import { HolonClient } from '../src/index.mjs';
import { HolonServer } from '../src/server.mjs';
import {
  checkEnv,
  configure,
  EventType,
  Family,
  InvalidTokenError,
  OBSERVABILITY_METHODS,
  parseOpObs,
  registerObservabilityService,
  reset,
} from '../src/observability.mjs';

test('parseOpObs rejects v2 and unknown tokens', () => {
  assert.throws(() => parseOpObs('all,otel'), InvalidTokenError);
  assert.throws(() => parseOpObs('all,sessions'), InvalidTokenError);
  assert.throws(() => parseOpObs('logs,unknown'), InvalidTokenError);
  const got = parseOpObs('logs,metrics,events');
  assert.deepEqual(got, new Set([Family.LOGS, Family.METRICS, Family.EVENTS]));
});

test('checkEnv rejects v2 tokens and OP_SESSIONS', () => {
  assert.throws(() => checkEnv({ OP_OBS: 'logs,otel' }), InvalidTokenError);
  assert.throws(() => checkEnv({ OP_OBS: 'logs,sessions' }), InvalidTokenError);
  assert.throws(() => checkEnv({ OP_SESSIONS: 'metrics' }), InvalidTokenError);
});

test('HolonObservability handlers serve ring contents over the WebSocket channel', async () => {
  const previousEnv = globalThis.__HOLON_ENV__;
  globalThis.__HOLON_ENV__ = {
    OP_OBS: 'logs,metrics,events',
    OP_INSTANCE_UID: 'web-uid-1',
  };

  const server = new HolonServer('ws://127.0.0.1:0/api/v1/rpc', { maxConnections: 1 });
  let client = null;
  try {
    const obs = configure({ slug: 'gabriel-greeting-web', instanceUid: 'web-uid-1' });
    obs.logger('test').info('INSTANCE_READY', { component: 'web' });
    obs.counter('web_requests_total', 'request count', { method: 'SayHello' }).inc();
    obs.gauge('web_live_gauge', 'live gauge').set(2.5);
    obs.histogram('web_latency_seconds', 'latency').observe(0.025);
    obs.emit(EventType.INSTANCE_READY, { listener: 'websocket' });

    const address = await server.start();
    client = new HolonClient(address, {
      WebSocket,
      reconnect: false,
      heartbeat: false,
    });
    registerObservabilityService(client, obs);
    await client.connect();
    const peer = await server.waitForClient({ timeout: 1000 });

    const logs = await server.invoke(peer, OBSERVABILITY_METHODS.Logs, {
      follow: false,
      min_level: 'TRACE',
    });
    assert.equal(logs.entries.length, 1);
    assert.equal(logs.entries[0].message, 'INSTANCE_READY');
    assert.equal(logs.entries[0].slug, 'gabriel-greeting-web');

    const metrics = await server.invoke(peer, OBSERVABILITY_METHODS.Metrics, {
      name_prefixes: ['web_'],
    });
    assert.equal(metrics.slug, 'gabriel-greeting-web');
    assert.ok(metrics.samples.some((sample) => sample.name === 'web_requests_total' && sample.counter === 1));
    assert.ok(metrics.samples.some((sample) => sample.name === 'web_live_gauge' && sample.gauge === 2.5));
    assert.ok(metrics.samples.some((sample) => sample.name === 'web_latency_seconds' && sample.histogram.count === 1));

    const events = await server.invoke(peer, OBSERVABILITY_METHODS.Events, {
      follow: false,
      types: ['INSTANCE_READY'],
    });
    assert.equal(events.events.length, 1);
    assert.equal(events.events[0].type, 'INSTANCE_READY');
    assert.equal(events.events[0].payload.listener, 'websocket');
  } finally {
    if (client) client.close();
    await server.close();
    reset();
    if (previousEnv === undefined) {
      delete globalThis.__HOLON_ENV__;
    } else {
      globalThis.__HOLON_ENV__ = previousEnv;
    }
  }
});
