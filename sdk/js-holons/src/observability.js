/**
 * @fileoverview Node.js reference implementation of the cross-SDK
 * observability layer. Mirrors sdk/go-holons/pkg/observability and the
 * Python port in sdk/python-holons/holons/observability.py.
 *
 * Activation follows the OP_OBS env discipline from OBSERVABILITY.md:
 * logs/metrics/events/prom are off by default; the parent launcher
 * turns families on; code tunes granularity within enabled families.
 * Zero cost when disabled — loggers and metrics return shared stubs.
 */

'use strict';

const fs = require('fs');
const path = require('path');
const os = require('os');

// --- Families ----------------------------------------------------------------

const Family = Object.freeze({
  LOGS: 'logs',
  METRICS: 'metrics',
  EVENTS: 'events',
  PROM: 'prom',
  OTEL: 'otel', // reserved v2
});

const V1_TOKENS = new Set(['logs', 'metrics', 'events', 'prom', 'all']);

class InvalidTokenError extends Error {
  constructor(token, reason) {
    super(`OP_OBS: ${reason}: ${token}`);
    this.name = 'InvalidTokenError';
    this.token = token;
    this.reason = reason;
  }
}

function parseOpObs(raw) {
  const out = new Set();
  if (!raw || !raw.trim()) return out;
  for (const tokRaw of raw.split(',')) {
    const tok = tokRaw.trim();
    if (!tok) continue;
    if (tok === 'otel' || tok === 'sessions') continue; // swallowed here, rejected by checkEnv
    if (!V1_TOKENS.has(tok)) continue;
    if (tok === 'all') {
      out.add(Family.LOGS);
      out.add(Family.METRICS);
      out.add(Family.EVENTS);
      out.add(Family.PROM);
    } else {
      out.add(tok);
    }
  }
  return out;
}

function checkEnv(env = process.env) {
  if ((env.OP_SESSIONS || '').trim()) {
    throw new InvalidTokenError(env.OP_SESSIONS.trim(), 'sessions are reserved for v2; not implemented in v1');
  }
  const raw = (env.OP_OBS || '').trim();
  if (!raw) return;
  for (const tokRaw of raw.split(',')) {
    const tok = tokRaw.trim();
    if (!tok) continue;
    if (tok === 'otel') {
      throw new InvalidTokenError('otel', 'otel export is reserved for v2; not implemented in v1');
    }
    if (tok === 'sessions') {
      throw new InvalidTokenError('sessions', 'sessions are reserved for v2; not implemented in v1');
    }
    if (!V1_TOKENS.has(tok)) {
      throw new InvalidTokenError(tok, 'unknown OP_OBS token');
    }
  }
}

// --- Levels ------------------------------------------------------------------

const Level = Object.freeze({
  UNSET: 0, TRACE: 1, DEBUG: 2, INFO: 3, WARN: 4, ERROR: 5, FATAL: 6,
});
const LEVEL_NAMES = { 1: 'TRACE', 2: 'DEBUG', 3: 'INFO', 4: 'WARN', 5: 'ERROR', 6: 'FATAL' };
function levelName(l) { return LEVEL_NAMES[l] || 'UNSPECIFIED'; }

function parseLevel(s) {
  if (!s) return Level.INFO;
  const u = String(s).trim().toUpperCase();
  return { TRACE: Level.TRACE, DEBUG: Level.DEBUG, INFO: Level.INFO,
    WARN: Level.WARN, WARNING: Level.WARN,
    ERROR: Level.ERROR, FATAL: Level.FATAL }[u] || Level.INFO;
}

// --- Event types -------------------------------------------------------------

const EventType = Object.freeze({
  UNSPECIFIED: 0,
  INSTANCE_SPAWNED: 1, INSTANCE_READY: 2, INSTANCE_EXITED: 3, INSTANCE_CRASHED: 4,
  SESSION_STARTED: 5, SESSION_ENDED: 6,
  HANDLER_PANIC: 7, CONFIG_RELOADED: 8,
});
const EVENT_TYPE_NAMES = {
  1: 'INSTANCE_SPAWNED', 2: 'INSTANCE_READY', 3: 'INSTANCE_EXITED', 4: 'INSTANCE_CRASHED',
  5: 'SESSION_STARTED', 6: 'SESSION_ENDED', 7: 'HANDLER_PANIC', 8: 'CONFIG_RELOADED',
};
function eventTypeName(t) { return EVENT_TYPE_NAMES[t] || 'UNSPECIFIED'; }

// --- Chain helpers -----------------------------------------------------------

function appendDirectChild(src, childSlug, childUid) {
  return [...(src || []), { slug: childSlug, instance_uid: childUid }];
}

function enrichForMultilog(wire, streamSourceSlug, streamSourceUid) {
  return appendDirectChild(wire, streamSourceSlug, streamSourceUid);
}

// --- Ring buffer for log entries --------------------------------------------

class LogRing {
  constructor(capacity = 1024) {
    this._capacity = Math.max(1, capacity);
    this._buf = [];
    this._subs = [];
  }
  push(entry) {
    this._buf.push(entry);
    if (this._buf.length > this._capacity) {
      this._buf.splice(0, this._buf.length - this._capacity);
    }
    for (const fn of [...this._subs]) {
      try { fn(entry); } catch (_) { /* ignore */ }
    }
  }
  drain() { return [...this._buf]; }
  drainSince(cutoff) { return this._buf.filter((e) => e.timestamp >= cutoff); }
  subscribe(fn) {
    this._subs.push(fn);
    return () => {
      const i = this._subs.indexOf(fn);
      if (i >= 0) this._subs.splice(i, 1);
    };
  }
  get length() { return this._buf.length; }
  get capacity() { return this._capacity; }
}

// --- Event bus ---------------------------------------------------------------

class EventBus {
  constructor(capacity = 256) {
    this._capacity = Math.max(1, capacity);
    this._buf = [];
    this._subs = [];
    this._closed = false;
  }
  emit(event) {
    if (this._closed) return;
    this._buf.push(event);
    if (this._buf.length > this._capacity) {
      this._buf.splice(0, this._buf.length - this._capacity);
    }
    for (const fn of [...this._subs]) {
      try { fn(event); } catch (_) { /* ignore */ }
    }
  }
  drain() { return [...this._buf]; }
  drainSince(cutoff) { return this._buf.filter((e) => e.timestamp >= cutoff); }
  subscribe(fn) {
    this._subs.push(fn);
    return () => {
      const i = this._subs.indexOf(fn);
      if (i >= 0) this._subs.splice(i, 1);
    };
  }
  close() {
    this._closed = true;
    this._subs.length = 0;
  }
}

// --- Metrics -----------------------------------------------------------------

class Counter {
  constructor(name, help = '', labels = {}) {
    this.name = name;
    this.help = help;
    this.labels = { ...labels };
    this._v = 0;
  }
  inc(n = 1) { if (n >= 0) this._v += n; }
  add(n) { this.inc(n); }
  value() { return this._v; }
}

class Gauge {
  constructor(name, help = '', labels = {}) {
    this.name = name;
    this.help = help;
    this.labels = { ...labels };
    this._v = 0;
  }
  set(v) { this._v = Number(v); }
  add(d) { this._v += Number(d); }
  value() { return this._v; }
}

const DEFAULT_BUCKETS = [
  50e-6, 100e-6, 250e-6, 500e-6,
  1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
  1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
];

class Histogram {
  constructor(name, help = '', labels = {}, bounds = null) {
    this.name = name;
    this.help = help;
    this.labels = { ...labels };
    this._bounds = [...(bounds && bounds.length ? bounds : DEFAULT_BUCKETS)].sort((a, b) => a - b);
    this._counts = new Array(this._bounds.length).fill(0);
    this._total = 0;
    this._sum = 0;
  }
  observe(v) {
    this._total += 1;
    this._sum += v;
    for (let i = 0; i < this._bounds.length; i++) {
      if (v <= this._bounds[i]) this._counts[i] += 1;
    }
  }
  observeDuration(seconds) { this.observe(seconds); }
  snapshot() {
    return { bounds: [...this._bounds], counts: [...this._counts], total: this._total, sum: this._sum };
  }
  static quantile(snap, q) {
    if (snap.total === 0) return NaN;
    const target = snap.total * q;
    for (let i = 0; i < snap.counts.length; i++) {
      if (snap.counts[i] >= target) return snap.bounds[i];
    }
    return Infinity;
  }
}

function metricKey(name, labels) {
  const keys = Object.keys(labels || {}).sort();
  if (!keys.length) return name;
  return name + '|' + keys.map((k) => `${k}=${labels[k]}`).join(',');
}

class Registry {
  constructor() {
    this._counters = new Map();
    this._gauges = new Map();
    this._histograms = new Map();
  }
  counter(name, help = '', labels = {}) {
    const k = metricKey(name, labels);
    let c = this._counters.get(k);
    if (!c) { c = new Counter(name, help, labels); this._counters.set(k, c); }
    return c;
  }
  gauge(name, help = '', labels = {}) {
    const k = metricKey(name, labels);
    let g = this._gauges.get(k);
    if (!g) { g = new Gauge(name, help, labels); this._gauges.set(k, g); }
    return g;
  }
  histogram(name, help = '', labels = {}, bounds = null) {
    const k = metricKey(name, labels);
    let h = this._histograms.get(k);
    if (!h) { h = new Histogram(name, help, labels, bounds); this._histograms.set(k, h); }
    return h;
  }
  snapshot() {
    const counters = [...this._counters.values()]
      .map((c) => ({ name: c.name, help: c.help, labels: { ...c.labels }, value: c.value() }))
      .sort((a, b) => a.name.localeCompare(b.name));
    const gauges = [...this._gauges.values()]
      .map((g) => ({ name: g.name, help: g.help, labels: { ...g.labels }, value: g.value() }))
      .sort((a, b) => a.name.localeCompare(b.name));
    const histograms = [...this._histograms.values()]
      .map((h) => ({ name: h.name, help: h.help, labels: { ...h.labels }, snap: h.snapshot() }))
      .sort((a, b) => a.name.localeCompare(b.name));
    return { capturedAt: new Date(), counters, gauges, histograms };
  }
}

// --- Logger ------------------------------------------------------------------

function stringify(v) {
  if (v === null || v === undefined) return '';
  if (typeof v === 'boolean') return v ? 'true' : 'false';
  if (v instanceof Error) return v.message;
  if (typeof v === 'object') try { return JSON.stringify(v); } catch (_) { return String(v); }
  return String(v);
}

class Logger {
  constructor(obs, name) {
    this._obs = obs;
    this._name = name;
    this._level = obs ? obs.cfg.defaultLogLevel : Level.FATAL;
  }
  get name() { return this._name; }
  setLevel(l) { this._level = l; }
  enabled(l) { return this._obs && l >= this._level; }
  _log(level, message, fields) {
    if (!this.enabled(level)) return;
    const redact = this._obs.cfg.redactedFields || new Set();
    const outFields = {};
    if (fields) {
      for (const k of Object.keys(fields)) {
        if (redact.has ? redact.has(k) : redact.includes && redact.includes(k)) {
          outFields[k] = '<redacted>';
        } else {
          outFields[k] = stringify(fields[k]);
        }
      }
    }
    const caller = callerFrame();
    const entry = {
      timestamp: Date.now() / 1000,
      level,
      slug: this._obs.cfg.slug || '',
      instance_uid: this._obs.cfg.instanceUid || '',
      session_id: '', // P2 fills via async context when wired
      rpc_method: '',
      message,
      fields: outFields,
      caller,
      chain: [],
    };
    if (this._obs.logRing) this._obs.logRing.push(entry);
  }
  trace(msg, fields) { this._log(Level.TRACE, msg, fields); }
  debug(msg, fields) { this._log(Level.DEBUG, msg, fields); }
  info(msg, fields)  { this._log(Level.INFO,  msg, fields); }
  warn(msg, fields)  { this._log(Level.WARN,  msg, fields); }
  error(msg, fields) { this._log(Level.ERROR, msg, fields); }
  fatal(msg, fields) { this._log(Level.FATAL, msg, fields); }
}

function callerFrame() {
  // Best-effort file:line extraction from a fresh Error stack.
  const err = new Error();
  const stack = (err.stack || '').split('\n');
  // Skip this function + Logger._log + level method + caller.
  for (let i = 3; i < stack.length; i++) {
    const m = stack[i].match(/\(([^()]+):(\d+):\d+\)$/) || stack[i].match(/at ([^\s]+):(\d+):\d+/);
    if (m) return `${path.basename(m[1])}:${m[2]}`;
  }
  return '';
}

const _disabledLogger = Object.freeze({
  _obs: null,
  _name: '',
  _level: Level.FATAL,
  enabled() { return false; },
  setLevel() { /* noop */ },
  trace() {}, debug() {}, info() {}, warn() {}, error() {}, fatal() {},
});

// --- Observability root ------------------------------------------------------

class Observability {
  constructor(cfg, families) {
    this.cfg = cfg;
    this.families = families;
    this.logRing = families.has(Family.LOGS) ? new LogRing(cfg.logsRingSize || 1024) : null;
    this.registry = families.has(Family.METRICS) ? new Registry() : null;
    this.eventBus = families.has(Family.EVENTS) ? new EventBus(cfg.eventsRingSize || 256) : null;
    this._loggers = new Map();
  }
  enabled(family) { return this.families.has(family); }
  isOrganismRoot() {
    return !!this.cfg.organismUid && this.cfg.organismUid === this.cfg.instanceUid;
  }
  logger(name) {
    if (!this.families.has(Family.LOGS)) return _disabledLogger;
    let l = this._loggers.get(name);
    if (!l) { l = new Logger(this, name); this._loggers.set(name, l); }
    return l;
  }
  counter(name, help = '', labels = {}) { return this.registry ? this.registry.counter(name, help, labels) : null; }
  gauge(name, help = '', labels = {}) { return this.registry ? this.registry.gauge(name, help, labels) : null; }
  histogram(name, help = '', labels = {}, bounds = null) { return this.registry ? this.registry.histogram(name, help, labels, bounds) : null; }
  emit(type, payload) {
    if (!this.eventBus) return;
    const redact = this.cfg.redactedFields || new Set();
    const p = {};
    if (payload) {
      for (const k of Object.keys(payload)) {
        if (redact.has ? redact.has(k) : (redact.includes && redact.includes(k))) {
          p[k] = '<redacted>';
        } else {
          p[k] = stringify(payload[k]);
        }
      }
    }
    this.eventBus.emit({
      timestamp: Date.now() / 1000,
      type,
      slug: this.cfg.slug || '',
      instance_uid: this.cfg.instanceUid || '',
      session_id: '',
      payload: p,
      chain: [],
    });
  }
  close() {
    if (this.eventBus) this.eventBus.close();
  }
}

let _current = null;

function configure(cfg = {}) {
  const families = parseOpObs(process.env.OP_OBS || '');
  const normalized = {
    slug: cfg.slug || (process.argv[1] ? path.basename(process.argv[1]) : ''),
    instanceUid: cfg.instanceUid || '',
    organismUid: cfg.organismUid || '',
    organismSlug: cfg.organismSlug || '',
    promAddr: cfg.promAddr || '',
    runDir: cfg.runDir || '',
    defaultLogLevel: cfg.defaultLogLevel || Level.INFO,
    redactedFields: cfg.redactedFields ? new Set(cfg.redactedFields) : new Set(),
    logsRingSize: cfg.logsRingSize,
    eventsRingSize: cfg.eventsRingSize,
  };
  const obs = new Observability(normalized, families);
  _current = obs;
  return obs;
}

function fromEnv(base = {}) {
  const cfg = { ...base };
  cfg.instanceUid = cfg.instanceUid || process.env.OP_INSTANCE_UID || '';
  cfg.organismUid = cfg.organismUid || process.env.OP_ORGANISM_UID || '';
  cfg.organismSlug = cfg.organismSlug || process.env.OP_ORGANISM_SLUG || '';
  cfg.promAddr = cfg.promAddr || process.env.OP_PROM_ADDR || '';
  cfg.runDir = cfg.runDir || process.env.OP_RUN_DIR || '';
  return configure(cfg);
}

function current() {
  if (_current) return _current;
  return new Observability({
    slug: '', instanceUid: '', organismUid: '', organismSlug: '',
    defaultLogLevel: Level.FATAL, redactedFields: new Set(),
  }, new Set());
}

function reset() {
  if (_current) _current.close();
  _current = null;
}

// --- Disk writers -----------------------------------------------------------

function enableDiskWriters(runDir) {
  const obs = _current;
  if (!obs || !runDir) return;
  fs.mkdirSync(runDir, { recursive: true });
  if (obs.enabled(Family.LOGS) && obs.logRing) {
    const fp = path.join(runDir, 'stdout.log');
    obs.logRing.subscribe((e) => {
      const rec = {
        kind: 'log',
        ts: new Date(e.timestamp * 1000).toISOString(),
        level: levelName(e.level),
        slug: e.slug,
        instance_uid: e.instance_uid,
        message: e.message,
      };
      if (e.session_id) rec.session_id = e.session_id;
      if (e.rpc_method) rec.rpc_method = e.rpc_method;
      if (Object.keys(e.fields || {}).length) rec.fields = e.fields;
      if (e.caller) rec.caller = e.caller;
      if (e.chain && e.chain.length) rec.chain = e.chain;
      try { fs.appendFileSync(fp, JSON.stringify(rec) + '\n'); } catch (_) {}
    });
  }
  if (obs.enabled(Family.EVENTS) && obs.eventBus) {
    const fp = path.join(runDir, 'events.jsonl');
    obs.eventBus.subscribe((e) => {
      const rec = {
        kind: 'event',
        ts: new Date(e.timestamp * 1000).toISOString(),
        type: eventTypeName(e.type),
        slug: e.slug,
        instance_uid: e.instance_uid,
      };
      if (e.session_id) rec.session_id = e.session_id;
      if (Object.keys(e.payload || {}).length) rec.payload = e.payload;
      if (e.chain && e.chain.length) rec.chain = e.chain;
      try { fs.appendFileSync(fp, JSON.stringify(rec) + '\n'); } catch (_) {}
    });
  }
}

function writeMetaJson(runDir, meta) {
  if (!runDir) throw new Error('writeMetaJson: empty runDir');
  fs.mkdirSync(runDir, { recursive: true });
  const p = path.join(runDir, 'meta.json');
  const tmp = p + '.tmp';
  fs.writeFileSync(tmp, JSON.stringify(meta, null, 2));
  fs.renameSync(tmp, p);
}

function readMetaJson(runDir) {
  const p = path.join(runDir, 'meta.json');
  return JSON.parse(fs.readFileSync(p, 'utf8'));
}

module.exports = {
  Family, Level, EventType, InvalidTokenError,
  Counter, Gauge, Histogram, Registry, LogRing, EventBus,
  Logger, Observability,
  parseOpObs, checkEnv, parseLevel, levelName, eventTypeName,
  appendDirectChild, enrichForMultilog,
  configure, fromEnv, current, reset,
  enableDiskWriters, writeMetaJson, readMetaJson,
  DEFAULT_BUCKETS,
};
