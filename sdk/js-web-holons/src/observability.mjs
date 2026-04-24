/**
 * Browser / WASM reference implementation of the cross-SDK observability layer.
 *
 * Smaller scope than Node (js-holons): no filesystem access,
 * no Prometheus HTTP server — js-web holons are dial-only sandboxed
 * peers per the spec §Transports without outbound dial. The root
 * (Hub or Flutter organism) reads their HolonObservability.Logs /
 * Events / Metrics over the existing WebSocket full-duplex channel.
 *
 * Same activation model (OP_OBS env through import.meta.env or a
 * runtime bootstrap), same public surface, same on-wire shape.
 */

export const Family = Object.freeze({
  LOGS: 'logs', METRICS: 'metrics', EVENTS: 'events', PROM: 'prom', OTEL: 'otel',
});

const V1_TOKENS = new Set(['logs', 'metrics', 'events', 'prom', 'all']);

export class InvalidTokenError extends Error {
  constructor(token, reason) {
    super(`OP_OBS: ${reason}: ${token}`);
    this.name = 'InvalidTokenError';
    this.token = token;
  }
}

export function parseOpObs(raw) {
  const out = new Set();
  if (!raw || !raw.trim()) return out;
  for (const p of raw.split(',')) {
    const tok = p.trim();
    if (!tok) continue;
    if (tok === 'otel' || tok === 'sessions') continue;
    if (!V1_TOKENS.has(tok)) continue;
    if (tok === 'all') {
      out.add(Family.LOGS); out.add(Family.METRICS);
      out.add(Family.EVENTS); out.add(Family.PROM);
    } else {
      out.add(tok);
    }
  }
  return out;
}

export function checkEnv(env = {}) {
  if ((env.OP_SESSIONS || '').trim()) {
    throw new InvalidTokenError(env.OP_SESSIONS.trim(), 'sessions are reserved for v2; not implemented in v1');
  }
  const raw = (env.OP_OBS || '').trim();
  if (!raw) return;
  for (const p of raw.split(',')) {
    const tok = p.trim();
    if (!tok) continue;
    if (tok === 'otel') throw new InvalidTokenError(tok, 'otel export is reserved for v2; not implemented in v1');
    if (tok === 'sessions') throw new InvalidTokenError(tok, 'sessions are reserved for v2; not implemented in v1');
    if (!V1_TOKENS.has(tok)) throw new InvalidTokenError(tok, 'unknown OP_OBS token');
  }
}

export const Level = Object.freeze({
  UNSET: 0, TRACE: 1, DEBUG: 2, INFO: 3, WARN: 4, ERROR: 5, FATAL: 6,
});
const LEVEL_NAMES = { 1: 'TRACE', 2: 'DEBUG', 3: 'INFO', 4: 'WARN', 5: 'ERROR', 6: 'FATAL' };

export function parseLevel(s) {
  if (!s) return Level.INFO;
  const u = String(s).trim().toUpperCase();
  return ({ TRACE: Level.TRACE, DEBUG: Level.DEBUG, INFO: Level.INFO,
    WARN: Level.WARN, WARNING: Level.WARN, ERROR: Level.ERROR,
    FATAL: Level.FATAL })[u] || Level.INFO;
}

export const EventType = Object.freeze({
  UNSPECIFIED: 0,
  INSTANCE_SPAWNED: 1, INSTANCE_READY: 2, INSTANCE_EXITED: 3, INSTANCE_CRASHED: 4,
  SESSION_STARTED: 5, SESSION_ENDED: 6, HANDLER_PANIC: 7, CONFIG_RELOADED: 8,
});

export function appendDirectChild(src, childSlug, childUid) {
  return [...(src || []), { slug: childSlug, instance_uid: childUid }];
}
export function enrichForMultilog(wire, srcSlug, srcUid) {
  return appendDirectChild(wire, srcSlug, srcUid);
}

export class LogRing {
  constructor(capacity = 1024) {
    this._capacity = Math.max(1, capacity);
    this._buf = [];
    this._subs = [];
  }
  push(e) {
    this._buf.push(e);
    if (this._buf.length > this._capacity) this._buf.splice(0, this._buf.length - this._capacity);
    for (const fn of [...this._subs]) try { fn(e); } catch (_) {}
  }
  drain() { return [...this._buf]; }
  drainSince(cutoff) { return this._buf.filter((e) => e.timestamp >= cutoff); }
  subscribe(fn) {
    this._subs.push(fn);
    return () => { const i = this._subs.indexOf(fn); if (i >= 0) this._subs.splice(i, 1); };
  }
}

export class EventBus {
  constructor(capacity = 256) {
    this._capacity = Math.max(1, capacity);
    this._buf = [];
    this._subs = [];
    this._closed = false;
  }
  emit(e) {
    if (this._closed) return;
    this._buf.push(e);
    if (this._buf.length > this._capacity) this._buf.splice(0, this._buf.length - this._capacity);
    for (const fn of [...this._subs]) try { fn(e); } catch (_) {}
  }
  drain() { return [...this._buf]; }
  drainSince(cutoff) { return this._buf.filter((e) => e.timestamp >= cutoff); }
  subscribe(fn) {
    this._subs.push(fn);
    return () => { const i = this._subs.indexOf(fn); if (i >= 0) this._subs.splice(i, 1); };
  }
  close() { this._closed = true; this._subs.length = 0; }
}

export class Counter {
  constructor(name, help = '', labels = {}) {
    this.name = name; this.help = help; this.labels = { ...labels };
    this._v = 0;
  }
  inc(n = 1) { if (n >= 0) this._v += n; }
  add(n) { this.inc(n); }
  value() { return this._v; }
}

export class Gauge {
  constructor(name, help = '', labels = {}) {
    this.name = name; this.help = help; this.labels = { ...labels };
    this._v = 0;
  }
  set(v) { this._v = Number(v); }
  add(d) { this._v += Number(d); }
  value() { return this._v; }
}

export const DEFAULT_BUCKETS = [
  50e-6, 100e-6, 250e-6, 500e-6,
  1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
  1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
];

export class Histogram {
  constructor(name, help = '', labels = {}, bounds = null) {
    this.name = name; this.help = help; this.labels = { ...labels };
    this._bounds = [...(bounds && bounds.length ? bounds : DEFAULT_BUCKETS)].sort((a, b) => a - b);
    this._counts = new Array(this._bounds.length).fill(0);
    this._total = 0;
    this._sum = 0;
  }
  observe(v) {
    this._total += 1;
    this._sum += v;
    for (let i = 0; i < this._bounds.length; i++) if (v <= this._bounds[i]) this._counts[i] += 1;
  }
  snapshot() { return { bounds: [...this._bounds], counts: [...this._counts], total: this._total, sum: this._sum }; }
  static quantile(snap, q) {
    if (snap.total === 0) return NaN;
    const target = snap.total * q;
    for (let i = 0; i < snap.counts.length; i++) if (snap.counts[i] >= target) return snap.bounds[i];
    return Infinity;
  }
}

function metricKey(name, labels) {
  const keys = Object.keys(labels || {}).sort();
  return keys.length ? name + '|' + keys.map((k) => `${k}=${labels[k]}`).join(',') : name;
}

export class Registry {
  constructor() {
    this._c = new Map(); this._g = new Map(); this._h = new Map();
  }
  counter(name, help = '', labels = {}) {
    const k = metricKey(name, labels);
    let c = this._c.get(k);
    if (!c) { c = new Counter(name, help, labels); this._c.set(k, c); }
    return c;
  }
  gauge(name, help = '', labels = {}) {
    const k = metricKey(name, labels);
    let g = this._g.get(k);
    if (!g) { g = new Gauge(name, help, labels); this._g.set(k, g); }
    return g;
  }
  histogram(name, help = '', labels = {}, bounds = null) {
    const k = metricKey(name, labels);
    let h = this._h.get(k);
    if (!h) { h = new Histogram(name, help, labels, bounds); this._h.set(k, h); }
    return h;
  }
}

export class Logger {
  constructor(obs, name) {
    this._obs = obs;
    this._name = name;
    this._level = obs ? obs.cfg.defaultLogLevel : Level.FATAL;
  }
  setLevel(l) { this._level = l; }
  enabled(l) { return this._obs && l >= this._level; }
  _log(level, message, fields) {
    if (!this.enabled(level)) return;
    const redact = this._obs.cfg.redactedFields || new Set();
    const out = {};
    if (fields) {
      for (const k of Object.keys(fields)) {
        const has = redact.has ? redact.has(k) : (redact.includes && redact.includes(k));
        out[k] = has ? '<redacted>' : String(fields[k] ?? '');
      }
    }
    const entry = {
      timestamp: Date.now() / 1000,
      level, slug: this._obs.cfg.slug, instance_uid: this._obs.cfg.instanceUid,
      session_id: '', rpc_method: '', message, fields: out, caller: '', chain: [],
    };
    if (this._obs.logRing) this._obs.logRing.push(entry);
  }
  trace(m, f) { this._log(Level.TRACE, m, f); }
  debug(m, f) { this._log(Level.DEBUG, m, f); }
  info(m, f)  { this._log(Level.INFO,  m, f); }
  warn(m, f)  { this._log(Level.WARN,  m, f); }
  error(m, f) { this._log(Level.ERROR, m, f); }
  fatal(m, f) { this._log(Level.FATAL, m, f); }
}

const _disabledLogger = Object.freeze({
  _obs: null, _name: '', _level: Level.FATAL,
  enabled() { return false; }, setLevel() {},
  trace() {}, debug() {}, info() {}, warn() {}, error() {}, fatal() {},
});

export class Observability {
  constructor(cfg, families) {
    this.cfg = cfg;
    this.families = families;
    this.logRing = families.has(Family.LOGS) ? new LogRing(cfg.logsRingSize || 1024) : null;
    this.registry = families.has(Family.METRICS) ? new Registry() : null;
    this.eventBus = families.has(Family.EVENTS) ? new EventBus(cfg.eventsRingSize || 256) : null;
    this._loggers = new Map();
  }
  enabled(f) { return this.families.has(f); }
  isOrganismRoot() { return !!this.cfg.organismUid && this.cfg.organismUid === this.cfg.instanceUid; }
  logger(name) {
    if (!this.families.has(Family.LOGS)) return _disabledLogger;
    let l = this._loggers.get(name);
    if (!l) { l = new Logger(this, name); this._loggers.set(name, l); }
    return l;
  }
  counter(n, h = '', labels = {}) { return this.registry ? this.registry.counter(n, h, labels) : null; }
  gauge(n, h = '', labels = {}) { return this.registry ? this.registry.gauge(n, h, labels) : null; }
  histogram(n, h = '', labels = {}, bounds = null) { return this.registry ? this.registry.histogram(n, h, labels, bounds) : null; }
  emit(type, payload) {
    if (!this.eventBus) return;
    const redact = this.cfg.redactedFields || new Set();
    const p = {};
    if (payload) {
      for (const k of Object.keys(payload)) {
        const has = redact.has ? redact.has(k) : (redact.includes && redact.includes(k));
        p[k] = has ? '<redacted>' : String(payload[k] ?? '');
      }
    }
    this.eventBus.emit({
      timestamp: Date.now() / 1000,
      type, slug: this.cfg.slug, instance_uid: this.cfg.instanceUid,
      session_id: '', payload: p, chain: [],
    });
  }
  close() { if (this.eventBus) this.eventBus.close(); }
}

let _current = null;

export function configure(cfg = {}) {
  const env = (typeof import.meta !== 'undefined' && import.meta.env) || (typeof globalThis !== 'undefined' && globalThis.__HOLON_ENV__) || {};
  const families = parseOpObs(env.OP_OBS || '');
  const normalized = {
    slug: cfg.slug || '',
    instanceUid: cfg.instanceUid || env.OP_INSTANCE_UID || '',
    organismUid: cfg.organismUid || env.OP_ORGANISM_UID || '',
    organismSlug: cfg.organismSlug || env.OP_ORGANISM_SLUG || '',
    promAddr: cfg.promAddr || env.OP_PROM_ADDR || '',
    runDir: cfg.runDir || env.OP_RUN_DIR || '',
    defaultLogLevel: cfg.defaultLogLevel || Level.INFO,
    redactedFields: cfg.redactedFields ? new Set(cfg.redactedFields) : new Set(),
    logsRingSize: cfg.logsRingSize,
    eventsRingSize: cfg.eventsRingSize,
  };
  const obs = new Observability(normalized, families);
  _current = obs;
  return obs;
}

export function fromEnv(base = {}) { return configure(base); }

export function current() {
  if (_current) return _current;
  return new Observability({
    slug: '', instanceUid: '', organismUid: '', organismSlug: '',
    defaultLogLevel: Level.FATAL, redactedFields: new Set(),
  }, new Set());
}

export function reset() { if (_current) _current.close(); _current = null; }
