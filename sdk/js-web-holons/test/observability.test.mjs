import test from 'node:test';
import assert from 'node:assert/strict';

import { checkEnv, Family, InvalidTokenError, parseOpObs } from '../src/observability.mjs';

test('parseOpObs drops v2 tokens', () => {
  const all = new Set([Family.LOGS, Family.METRICS, Family.EVENTS, Family.PROM]);
  for (const input of ['all,otel', 'all,sessions']) {
    const got = parseOpObs(input);
    assert.equal(got.size, all.size);
    for (const family of all) assert.ok(got.has(family));
  }
});

test('checkEnv rejects v2 tokens and OP_SESSIONS', () => {
  assert.throws(() => checkEnv({ OP_OBS: 'logs,otel' }), InvalidTokenError);
  assert.throws(() => checkEnv({ OP_OBS: 'logs,sessions' }), InvalidTokenError);
  assert.throws(() => checkEnv({ OP_SESSIONS: 'metrics' }), InvalidTokenError);
});
