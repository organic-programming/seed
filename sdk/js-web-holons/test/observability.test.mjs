import test from 'node:test';
import assert from 'node:assert/strict';
import WebSocket from 'ws';

import { HolonClient, HolonHTTPClient } from '../src/index.mjs';
import { HolonServer } from '../src/server.mjs';
import { startHTTPHolonRPCServer } from './support/http_harness.mjs';
import {
  checkEnv,
  configure,
  EventType,
  Family,
  InvalidTokenError,
  makeHolonObservabilityHandlers,
  OBSERVABILITY_METHODS,
  parseOpObs,
  Private,
  registerObservabilityService,
  reset,
  WithTransitiveObservability,
} from '../src/observability.mjs';

test('parseOpObs rejects v2 and unknown tokens', () => {
  assert.throws(() => parseOpObs('all,otel'), InvalidTokenError);
  assert.throws(() => parseOpObs('all,sessions'), InvalidTokenError);
  assert.throws(() => parseOpObs('logs,unknown'), InvalidTokenError);
  const got = parseOpObs('logs,metrics,events');
  assert.deepEqual(got, new Set([Family.LOGS, Family.METRICS, Family.EVENTS]));
});

test('Logs follow replays ring on subscribe then delivers live entries', async () => {
  const previousEnv = globalThis.__HOLON_ENV__;
  globalThis.__HOLON_ENV__ = { OP_OBS: 'logs', OP_INSTANCE_UID: 'web-logs-1' };

  try {
    const obs = configure({ slug: 'web-logger', instanceUid: 'web-logs-1' });
    const handler = makeHolonObservabilityHandlers(obs)[OBSERVABILITY_METHODS.Logs];

    obs.logger('test').info('private-before', { scope: 'local' }, Private());
    obs.logger('test').info('snapshot-before', { phase: 'snapshot' });
    assert.equal(obs.logRing.drain().length, 2);

    const stream = await handler({ follow: true, min_level: 'TRACE' });
    const iterator = stream[Symbol.asyncIterator]();
    const first = await iterator.next();
    assert.equal(first.done, false);
    assert.equal(first.value.message, 'snapshot-before');

    obs.logger('test').info('live-after', { phase: 'live' });
    const second = await iterator.next();
    assert.equal(second.done, false);
    assert.equal(second.value.message, 'live-after');

    await iterator.return?.();
  } finally {
    reset();
    restoreEnv(previousEnv);
  }
});

test('Events follow replays ring on subscribe then delivers live entries', async () => {
  const previousEnv = globalThis.__HOLON_ENV__;
  globalThis.__HOLON_ENV__ = { OP_OBS: 'events', OP_INSTANCE_UID: 'web-events-1' };

  try {
    const obs = configure({ slug: 'web-events', instanceUid: 'web-events-1' });
    const handler = makeHolonObservabilityHandlers(obs)[OBSERVABILITY_METHODS.Events];

    obs.emit(EventType.INSTANCE_READY, { listener: 'private' }, Private());
    obs.emit(EventType.INSTANCE_READY, { listener: 'snapshot' });
    assert.equal(obs.eventBus.drain().length, 2);

    const stream = await handler({ follow: true, types: ['INSTANCE_READY'] });
    const iterator = stream[Symbol.asyncIterator]();
    const first = await iterator.next();
    assert.equal(first.done, false);
    assert.equal(first.value.payload.listener, 'snapshot');

    obs.emit(EventType.INSTANCE_READY, { listener: 'live' });
    const second = await iterator.next();
    assert.equal(second.done, false);
    assert.equal(second.value.payload.listener, 'live');

    await iterator.return?.();
  } finally {
    reset();
    restoreEnv(previousEnv);
  }
});

test('HolonHTTPClient opt-in transitive observability relays SSE logs and events', async () => {
  const previousEnv = globalThis.__HOLON_ENV__;
  globalThis.__HOLON_ENV__ = { OP_OBS: 'logs,events', OP_INSTANCE_UID: 'web-root-1' };

  const server = await startHTTPHolonRPCServer({
    streamHandlers: {
      [OBSERVABILITY_METHODS.Logs]: async function* (params) {
        assert.equal(params.follow, true);
        yield {
          ts: { seconds: '1', nanos: 0 },
          level: 'INFO',
          slug: 'remote-web',
          instance_uid: 'remote-uid',
          session_id: '',
          rpc_method: '',
          message: 'remote log',
          fields: {},
          caller: '',
          chain: [],
        };
      },
      [OBSERVABILITY_METHODS.Events]: async function* (params) {
        assert.equal(params.follow, true);
        yield {
          ts: { seconds: '2', nanos: 0 },
          type: 'INSTANCE_READY',
          slug: 'remote-web',
          instance_uid: 'remote-uid',
          session_id: '',
          payload: { listener: 'https://example.invalid/rpc' },
          chain: [],
        };
      },
    },
  });

  let client = null;
  try {
    const obs = configure({ slug: 'web-root', instanceUid: 'web-root-1' });
    client = new HolonHTTPClient(server.baseUrl, {
      ...WithTransitiveObservability(true),
      transitiveObservabilityRetryDelayMs: 10,
    });

    await waitFor(() => obs.logRing.drain().some((entry) => entry.message === 'remote log'));
    await waitFor(() => obs.eventBus.drain().some((event) => event.payload.listener === 'https://example.invalid/rpc'));

    const log = obs.logRing.drain().find((entry) => entry.message === 'remote log');
    assert.deepEqual(log.chain, [{ slug: 'remote-web', instance_uid: 'remote-uid' }]);

    const event = obs.eventBus.drain().find((entry) => entry.payload.listener === 'https://example.invalid/rpc');
    assert.deepEqual(event.chain, [{ slug: 'remote-web', instance_uid: 'remote-uid' }]);
  } finally {
    if (client) client.close();
    await server.close();
    reset();
    restoreEnv(previousEnv);
  }
});

function restoreEnv(previousEnv) {
  if (previousEnv === undefined) {
    delete globalThis.__HOLON_ENV__;
  } else {
    globalThis.__HOLON_ENV__ = previousEnv;
  }
}

async function waitFor(predicate, timeoutMs = 1000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.fail('timed out waiting for condition');
}

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
