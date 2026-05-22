/**
 * @fileoverview Node.js implementation of the cross-SDK observability layer.
 * The wire shape mirrors holons/v1/observability.proto: logs and events are
 * LogRecord values, metrics are Metric values.
 */

'use strict';

const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { AsyncLocalStorage } = require('node:async_hooks');
const grpc = require('@grpc/grpc-js');

const observabilityWire = require('./gen/holons/v1/observability');

// --- Families ----------------------------------------------------------------

const Family = Object.freeze({
  LOGS: 'logs',
  METRICS: 'metrics',
  EVENTS: 'events',
  PROM: 'prom',
});

const V1_TOKENS = new Set(['logs', 'metrics', 'events', 'prom', 'all']);

class InvalidTokenError extends Error {
  constructor(token, reason, variable = 'OP_OBS') {
    super(`${variable}: ${reason}: ${token}`);
    this.name = 'InvalidTokenError';
    this.token = token;
    this.reason = reason;
    this.variable = variable;
  }
}

function parseOpObs(raw) {
  const out = new Set();
  if (!raw || !raw.trim()) return out;
  for (const tokRaw of raw.split(',')) {
    const tok = tokRaw.trim();
    if (!tok) continue;
    if (!V1_TOKENS.has(tok)) {
      throw new InvalidTokenError(tok, 'unknown OP_OBS token');
    }
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
  const raw = (env.OP_OBS || '').trim();
  if (!raw) return;
  for (const tokRaw of raw.split(',')) {
    const tok = tokRaw.trim();
    if (!tok) continue;
    if (!V1_TOKENS.has(tok)) {
      throw new InvalidTokenError(tok, 'unknown OP_OBS token');
    }
  }
}

// --- Severity and events ------------------------------------------------------

const Level = Object.freeze({
  UNSET: 0,
  TRACE: 1,
  DEBUG: 5,
  INFO: 9,
  WARN: 13,
  ERROR: 17,
  FATAL: 21,
});

const SEVERITY_TEXT = Object.freeze({
  [Level.TRACE]: 'TRACE',
  [Level.DEBUG]: 'DEBUG',
  [Level.INFO]: 'INFO',
  [Level.WARN]: 'WARN',
  [Level.ERROR]: 'ERROR',
  [Level.FATAL]: 'FATAL',
});

const SEVERITY_PROTO = Object.freeze({
  [Level.UNSET]: 'SEVERITY_NUMBER_UNSPECIFIED',
  [Level.TRACE]: 'SEVERITY_NUMBER_TRACE',
  [Level.DEBUG]: 'SEVERITY_NUMBER_DEBUG',
  [Level.INFO]: 'SEVERITY_NUMBER_INFO',
  [Level.WARN]: 'SEVERITY_NUMBER_WARN',
  [Level.ERROR]: 'SEVERITY_NUMBER_ERROR',
  [Level.FATAL]: 'SEVERITY_NUMBER_FATAL',
});

const PROTO_TO_SEVERITY = Object.freeze(Object.fromEntries(
  Object.entries(SEVERITY_PROTO).map(([value, name]) => [name, Number(value)]),
));

function levelName(level) {
  return SEVERITY_TEXT[Number(level)] || 'UNSPECIFIED';
}

function parseLevel(value) {
  if (!value) return Level.INFO;
  if (typeof value === 'number') return normalizeSeverity(value) || Level.INFO;
  const upper = String(value).trim().toUpperCase();
  return Level[upper] || PROTO_TO_SEVERITY[upper] || Level.INFO;
}

function normalizeSeverity(value) {
  if (typeof value === 'number') return value;
  if (typeof value === 'string') {
    const upper = value.trim().toUpperCase();
    if (/^-?\d+$/.test(upper)) return Number(upper);
    if (Object.prototype.hasOwnProperty.call(Level, upper)) return Level[upper];
    if (Object.prototype.hasOwnProperty.call(PROTO_TO_SEVERITY, upper)) return PROTO_TO_SEVERITY[upper];
    if (upper === 'WARNING') return Level.WARN;
  }
  return Level.UNSET;
}

const EventName = Object.freeze({
  INSTANCE_SPAWNED: 'instance.spawned',
  INSTANCE_READY: 'instance.ready',
  INSTANCE_EXITED: 'instance.exited',
  INSTANCE_CRASHED: 'instance.crashed',
  SESSION_STARTED: 'session.started',
  SESSION_ENDED: 'session.ended',
  HANDLER_PANIC: 'handler.panic',
  CONFIG_RELOADED: 'config.reloaded',
});

const CANONICAL_EVENT_NAMES = new Set(Object.values(EventName));

function requireCanonicalEventName(value) {
  const name = String(value || '').trim();
  if (!CANONICAL_EVENT_NAMES.has(name)) {
    throw new Error(`unknown observability event_name: ${name || '<empty>'}`);
  }
  return name;
}

// --- Attributes and AnyValue --------------------------------------------------

const Attr = Object.freeze({
  HOLONS_SLUG: 'holons.slug',
  HOLONS_INSTANCE_UID: 'holons.instance_uid',
  HOLONS_SESSION_ID: 'holons.session_id',
  HOLONS_TRANSPORT: 'holons.transport',
  SERVICE_NAME: 'service.name',
  SERVICE_INSTANCE_ID: 'service.instance.id',
  RPC_METHOD: 'rpc.method',
  LOGGER_NAME: 'logger.name',
  CODE_CALLER: 'code.caller',
});

const SYSTEM_ATTRIBUTES = new Set(Object.values(Attr));

function toAnyValue(value) {
  if (typeof value === 'boolean') return { bool_value: value };
  if (typeof value === 'number') {
    if (Number.isSafeInteger(value)) return { int_value: value };
    return { double_value: value };
  }
  if (typeof value === 'string') return { string_value: value };
  return { string_value: String(value) };
}

function keyValue(key, value) {
  return { key: String(key), value: toAnyValue(value) };
}

function anyValueToString(value) {
  if (!value) return '';
  if (Object.prototype.hasOwnProperty.call(value, 'string_value')) return String(value.string_value || '');
  if (Object.prototype.hasOwnProperty.call(value, 'bool_value')) return value.bool_value ? 'true' : 'false';
  if (Object.prototype.hasOwnProperty.call(value, 'int_value')) return String(value.int_value);
  if (Object.prototype.hasOwnProperty.call(value, 'double_value')) return String(value.double_value);
  return '';
}

function anyValueRaw(value) {
  if (!value) return '';
  if (Object.prototype.hasOwnProperty.call(value, 'string_value')) return value.string_value || '';
  if (Object.prototype.hasOwnProperty.call(value, 'bool_value')) return Boolean(value.bool_value);
  if (Object.prototype.hasOwnProperty.call(value, 'int_value')) return Number(value.int_value);
  if (Object.prototype.hasOwnProperty.call(value, 'double_value')) return Number(value.double_value);
  return '';
}

function stringAttribute(attrsOrRecord, key) {
  const attrs = Array.isArray(attrsOrRecord) ? attrsOrRecord : (attrsOrRecord && attrsOrRecord.attributes) || [];
  for (const attr of attrs) {
    if (attr && attr.key === key) return anyValueToString(attr.value);
  }
  return '';
}

function rawAttribute(attrsOrRecord, key) {
  const attrs = Array.isArray(attrsOrRecord) ? attrsOrRecord : (attrsOrRecord && attrsOrRecord.attributes) || [];
  for (const attr of attrs) {
    if (attr && attr.key === key) return anyValueRaw(attr.value);
  }
  return '';
}

function userAttributes(record) {
  const out = {};
  for (const attr of (record && record.attributes) || []) {
    if (!attr || SYSTEM_ATTRIBUTES.has(attr.key)) continue;
    out[attr.key] = anyValueToString(attr.value);
  }
  return out;
}

function bodyString(record) {
  return anyValueToString(record && record.body);
}

function resourceAttributes(cfg = {}, sessionId = '') {
  const attrs = [];
  const slug = String(cfg.slug || '').trim();
  const uid = String(cfg.instanceUid || '').trim();
  const session = String(sessionId || '').trim();
  if (slug) {
    attrs.push(keyValue(Attr.HOLONS_SLUG, slug));
    attrs.push(keyValue(Attr.SERVICE_NAME, slug));
  }
  if (uid) {
    attrs.push(keyValue(Attr.HOLONS_INSTANCE_UID, uid));
    attrs.push(keyValue(Attr.SERVICE_INSTANCE_ID, uid));
  }
  if (session) {
    attrs.push(keyValue(Attr.HOLONS_SESSION_ID, session));
  }
  return attrs;
}

function sortedObjectAttributes(values = {}, redact = new Set()) {
  const attrs = [];
  for (const key of Object.keys(values || {}).sort()) {
    const value = values[key];
    if (key === '__holons_private') continue;
    if (redact.has ? redact.has(key) : false) attrs.push(keyValue(key, '<redacted>'));
    else attrs.push(keyValue(key, value));
  }
  return attrs;
}

// --- Session context ----------------------------------------------------------

const sessionContext = new AsyncLocalStorage();

function currentSessionContext() {
  const store = sessionContext.getStore() || {};
  return {
    sessionId: String(store.sessionId || store.session_id || ''),
    rpcMethod: String(store.rpcMethod || store.rpc_method || ''),
  };
}

function withSessionContext(context, fn) {
  const normalized = {
    sessionId: String((context && (context.sessionId || context.session_id)) || ''),
    rpcMethod: String((context && (context.rpcMethod || context.rpc_method)) || ''),
  };
  return sessionContext.run(normalized, fn);
}

// --- Chain helpers ------------------------------------------------------------

function appendDirectChild(src, childSlug) {
  const chain = Array.isArray(src) ? [...src] : [];
  const slug = String(childSlug || '').trim();
  if (slug) chain.push(slug);
  return chain;
}

function enrichForMultilog(wire, streamSourceSlug) {
  return appendDirectChild(wire, streamSourceSlug);
}

// --- Ring buffer for LogRecord values ----------------------------------------

class LogRing {
  constructor(capacity = 1024) {
    this._capacity = Math.max(1, capacity);
    this._buf = [];
    this._subs = [];
  }
  push(record) {
    this._buf.push(record);
    if (this._buf.length > this._capacity) {
      this._buf.splice(0, this._buf.length - this._capacity);
    }
    for (const fn of [...this._subs]) {
      try { fn(record); } catch (_) { /* ignore subscriber failures */ }
    }
  }
  drain() { return [...this._buf]; }
  drainSince(cutoffSeconds) { return this._buf.filter((record) => recordSeconds(record) >= cutoffSeconds); }
  subscribe(fn) {
    this._subs.push(fn);
    return () => {
      const i = this._subs.indexOf(fn);
      if (i >= 0) this._subs.splice(i, 1);
    };
  }
  subscribeWithSnapshot(fn) {
    const snapshot = [...this._buf];
    const stop = this.subscribe(fn);
    return { snapshot, stop };
  }
  get length() { return this._buf.length; }
  get capacity() { return this._capacity; }
}

class EventBus {
  constructor(capacity = 256) {
    this._capacity = Math.max(1, capacity);
    this._buf = [];
    this._subs = [];
    this._closed = false;
  }
  emit(record) {
    if (this._closed) return;
    this._buf.push(record);
    if (this._buf.length > this._capacity) {
      this._buf.splice(0, this._buf.length - this._capacity);
    }
    for (const fn of [...this._subs]) {
      try { fn(record); } catch (_) { /* ignore subscriber failures */ }
    }
  }
  drain() { return [...this._buf]; }
  drainSince(cutoffSeconds) { return this._buf.filter((record) => recordSeconds(record) >= cutoffSeconds); }
  subscribe(fn) {
    this._subs.push(fn);
    return () => {
      const i = this._subs.indexOf(fn);
      if (i >= 0) this._subs.splice(i, 1);
    };
  }
  subscribeWithSnapshot(fn) {
    const snapshot = [...this._buf];
    const stop = this.subscribe(fn);
    return { snapshot, stop };
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
  inc(n = 1) {
    const amount = Math.trunc(Number(n));
    if (amount >= 0) this._v += amount;
  }
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
    this._min = 0;
    this._max = 0;
  }
  observe(value) {
    const v = Number(value);
    if (this._total === 0 || v < this._min) this._min = v;
    if (this._total === 0 || v > this._max) this._max = v;
    this._total += 1;
    this._sum += v;
    for (let i = 0; i < this._bounds.length; i += 1) {
      if (v <= this._bounds[i]) this._counts[i] += 1;
    }
  }
  observeDuration(seconds) { this.observe(seconds); }
  snapshot() {
    return {
      bounds: [...this._bounds],
      counts: [...this._counts],
      total: this._total,
      sum: this._sum,
      min: this._min,
      max: this._max,
    };
  }
  static quantile(snap, q) {
    if (!snap || snap.total === 0) return NaN;
    const target = snap.total * q;
    for (let i = 0; i < snap.counts.length; i += 1) {
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
      .sort(compareMetricSamples);
    const gauges = [...this._gauges.values()]
      .map((g) => ({ name: g.name, help: g.help, labels: { ...g.labels }, value: g.value() }))
      .sort(compareMetricSamples);
    const histograms = [...this._histograms.values()]
      .map((h) => ({ name: h.name, help: h.help, labels: { ...h.labels }, snap: h.snapshot() }))
      .sort(compareMetricSamples);
    return { capturedAt: new Date(), counters, gauges, histograms };
  }
}

function compareMetricSamples(a, b) {
  const byName = a.name.localeCompare(b.name);
  if (byName) return byName;
  return JSON.stringify(a.labels || {}).localeCompare(JSON.stringify(b.labels || {}));
}

// --- Logger ------------------------------------------------------------------

class Logger {
  constructor(obs, name) {
    this._obs = obs;
    this._name = name;
    this._level = obs ? obs.cfg.defaultLogLevel : Level.FATAL;
  }
  get name() { return this._name; }
  setLevel(level) { this._level = normalizeSeverity(level) || Level.INFO; }
  enabled(level) { return this._obs && normalizeSeverity(level) >= this._level; }
  _log(level, message, fields, options = {}) {
    const severity = normalizeSeverity(level);
    if (!this.enabled(severity)) return;
    const redact = this._obs.cfg.redactedFields || new Set();
    let isPrivate = Boolean(options.private);
    const context = currentSessionContext();
    const attrs = resourceAttributes(this._obs.cfg, context.sessionId || this._obs.cfg.sessionId);
    if (this._name) attrs.push(keyValue(Attr.LOGGER_NAME, this._name));

    for (const key of Object.keys(fields || {}).sort()) {
      if (key === '__holons_private') {
        isPrivate = isPrivate || Boolean(fields[key]);
        continue;
      }
      if (redact.has(key)) attrs.push(keyValue(key, '<redacted>'));
      else attrs.push(keyValue(key, fields[key]));
    }
    if (context.rpcMethod) attrs.push(keyValue(Attr.RPC_METHOD, context.rpcMethod));
    const caller = callerFrame();
    if (caller) attrs.push(keyValue(Attr.CODE_CALLER, caller));

    const now = nowUnixNanoString();
    this._obs.logRing.push({
      time_unix_nano: now,
      observed_time_unix_nano: now,
      severity_number: severity,
      severity_text: levelName(severity),
      body: { string_value: String(message) },
      attributes: attrs,
      chain: [],
      private: isPrivate,
    });
  }
  trace(msg, fields, options) { this._log(Level.TRACE, msg, fields, options); }
  debug(msg, fields, options) { this._log(Level.DEBUG, msg, fields, options); }
  info(msg, fields, options) { this._log(Level.INFO, msg, fields, options); }
  warn(msg, fields, options) { this._log(Level.WARN, msg, fields, options); }
  error(msg, fields, options) { this._log(Level.ERROR, msg, fields, options); }
  fatal(msg, fields, options) { this._log(Level.FATAL, msg, fields, options); }
  private() { return new PrivateLogger(this); }
}

class PrivateLogger {
  constructor(base) {
    this._base = base;
  }
  trace(msg, fields) { this._base.trace(msg, fields, { private: true }); }
  debug(msg, fields) { this._base.debug(msg, fields, { private: true }); }
  info(msg, fields) { this._base.info(msg, fields, { private: true }); }
  warn(msg, fields) { this._base.warn(msg, fields, { private: true }); }
  error(msg, fields) { this._base.error(msg, fields, { private: true }); }
  fatal(msg, fields) { this._base.fatal(msg, fields, { private: true }); }
}

function Private(fields = {}) {
  return { ...(fields || {}), __holons_private: true };
}

function callerFrame() {
  const err = new Error();
  const stack = (err.stack || '').split('\n');
  for (let i = 3; i < stack.length; i += 1) {
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
  setLevel() {},
  trace() {}, debug() {}, info() {}, warn() {}, error() {}, fatal() {},
  private() { return this; },
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
    this.startUnixNano = cfg.startUnixNano || nowUnixNanoString();
  }
  enabled(family) { return this.families.has(family); }
  isOrganismRoot() {
    return !!this.cfg.organismUid && this.cfg.organismUid === this.cfg.instanceUid;
  }
  logger(name) {
    if (!this.families.has(Family.LOGS)) return _disabledLogger;
    let logger = this._loggers.get(name);
    if (!logger) { logger = new Logger(this, name); this._loggers.set(name, logger); }
    return logger;
  }
  counter(name, help = '', labels = {}) { return this.registry ? this.registry.counter(name, help, labels) : null; }
  gauge(name, help = '', labels = {}) { return this.registry ? this.registry.gauge(name, help, labels) : null; }
  histogram(name, help = '', labels = {}, bounds = null) { return this.registry ? this.registry.histogram(name, help, labels, bounds) : null; }
  emit(eventName, payload, options = {}) {
    if (!this.eventBus) return;
    const name = requireCanonicalEventName(eventName);
    const redact = this.cfg.redactedFields || new Set();
    let isPrivate = Boolean(options.private);
    if (payload && Boolean(payload.__holons_private)) isPrivate = true;
    const context = currentSessionContext();
    const attrs = resourceAttributes(this.cfg, context.sessionId || this.cfg.sessionId);
    attrs.push(...sortedObjectAttributes(payload || {}, redact));
    const now = nowUnixNanoString();
    this.eventBus.emit({
      time_unix_nano: now,
      observed_time_unix_nano: now,
      severity_number: Level.INFO,
      severity_text: 'INFO',
      body: { string_value: name },
      attributes: attrs,
      event_name: name,
      chain: [],
      private: isPrivate,
    });
  }
  emitPrivate(eventName, payload) {
    this.emit(eventName, payload, { private: true });
  }
  close() {
    if (this.eventBus) this.eventBus.close();
  }
}

let _current = null;

function configure(cfg = {}) {
  checkEnv();
  const slug = cfg.slug || (process.argv[1] ? path.basename(process.argv[1]) : '');
  const instanceUid = cfg.instanceUid || '';
  const runRoot = cfg.runDir || '';
  const families = parseOpObs(process.env.OP_OBS || '');
  const normalized = {
    slug,
    instanceUid,
    sessionId: cfg.sessionId || nowUnixNanoString(),
    organismUid: cfg.organismUid || '',
    organismSlug: cfg.organismSlug || '',
    promAddr: cfg.promAddr || '',
    runDir: runRoot ? deriveRunDir(runRoot, slug, instanceUid) : '',
    defaultLogLevel: parseLevel(cfg.defaultLogLevel || Level.INFO),
    redactedFields: cfg.redactedFields ? new Set(cfg.redactedFields) : new Set(),
    logsRingSize: cfg.logsRingSize,
    eventsRingSize: cfg.eventsRingSize,
    startUnixNano: nowUnixNanoString(),
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
    slug: '',
    instanceUid: '',
    sessionId: '',
    organismUid: '',
    organismSlug: '',
    defaultLogLevel: Level.FATAL,
    redactedFields: new Set(),
    startUnixNano: nowUnixNanoString(),
  }, new Set());
}

function reset() {
  if (_current) _current.close();
  _current = null;
}

function deriveRunDir(root, slug, uid) {
  if (!root || !slug || !uid) return root || '';
  return path.join(root, slug, uid);
}

// --- Proto conversion and gRPC service ---------------------------------------

function nowUnixNanoString() {
  return String(BigInt(Date.now()) * 1000000n);
}

function dateToUnixNanoString(date) {
  return String(BigInt(date.getTime()) * 1000000n);
}

function recordSeconds(record) {
  const raw = record && record.time_unix_nano;
  if (!raw) return 0;
  return Number(BigInt(String(raw)) / 1000000000n);
}

function secondsToUnixNanoString(seconds) {
  const whole = Math.trunc(Number(seconds) || 0);
  const nanos = Math.trunc(((Number(seconds) || 0) - whole) * 1_000_000_000);
  return String(BigInt(whole) * 1000000000n + BigInt(nanos));
}

function durationSeconds(duration) {
  if (!duration) return 0;
  return Number(duration.seconds || 0) + Number(duration.nanos || 0) / 1e9;
}

function cloneAnyValue(value) {
  if (!value) return undefined;
  if (Object.prototype.hasOwnProperty.call(value, 'string_value')) return { string_value: String(value.string_value || '') };
  if (Object.prototype.hasOwnProperty.call(value, 'bool_value')) return { bool_value: Boolean(value.bool_value) };
  if (Object.prototype.hasOwnProperty.call(value, 'int_value')) return { int_value: value.int_value };
  if (Object.prototype.hasOwnProperty.call(value, 'double_value')) return { double_value: Number(value.double_value) };
  return undefined;
}

function cloneAttributes(attrs) {
  return (attrs || []).filter(Boolean).map((attr) => ({
    key: String(attr.key || ''),
    value: cloneAnyValue(attr.value) || { string_value: '' },
  }));
}

function cloneLogRecord(record) {
  const out = {
    time_unix_nano: record && record.time_unix_nano ? String(record.time_unix_nano) : '0',
    severity_number: normalizeSeverity(record && record.severity_number),
    severity_text: String((record && record.severity_text) || levelName(normalizeSeverity(record && record.severity_number))),
    body: cloneAnyValue(record && record.body) || { string_value: '' },
    attributes: cloneAttributes(record && record.attributes),
    dropped_attributes_count: Number((record && record.dropped_attributes_count) || 0),
    flags: Number((record && record.flags) || 0),
    trace_id: (record && record.trace_id) || Buffer.alloc(0),
    span_id: (record && record.span_id) || Buffer.alloc(0),
    observed_time_unix_nano: record && record.observed_time_unix_nano ? String(record.observed_time_unix_nano) : '0',
    event_name: String((record && record.event_name) || ''),
    chain: [...((record && record.chain) || [])].map(String),
  };
  if (record && record.private) out.private = true;
  return out;
}

function toProtoLogRecord(record) {
  const out = cloneLogRecord(record || {});
  delete out.private;
  return out;
}

function fromProtoLogRecord(record) {
  return cloneLogRecord(record || {});
}

function histogramBucketCounts(snap) {
  const cumulative = snap.counts || [];
  const counts = [];
  let prev = 0;
  for (const count of cumulative) {
    const delta = Math.max(0, Number(count || 0) - prev);
    counts.push(delta);
    prev = Number(count || 0);
  }
  counts.push(Math.max(0, Number(snap.total || 0) - prev));
  return counts;
}

function toProtoMetrics(snapshot, cfg = {}, startUnixNano = '0') {
  const out = [];
  const timeUnixNano = dateToUnixNanoString(snapshot.capturedAt || new Date());
  for (const counter of snapshot.counters || []) {
    const attrs = [...resourceAttributes(cfg), ...sortedObjectAttributes(counter.labels || {})];
    out.push({
      name: counter.name,
      description: counter.help || '',
      sum: {
        aggregation_temporality: 'AGGREGATION_TEMPORALITY_CUMULATIVE',
        is_monotonic: true,
        data_points: [{
          start_time_unix_nano: startUnixNano,
          time_unix_nano: timeUnixNano,
          as_int: Math.trunc(Number(counter.value || 0)),
          attributes: attrs,
        }],
      },
    });
  }
  for (const gauge of snapshot.gauges || []) {
    const attrs = [...resourceAttributes(cfg), ...sortedObjectAttributes(gauge.labels || {})];
    out.push({
      name: gauge.name,
      description: gauge.help || '',
      gauge: {
        data_points: [{
          start_time_unix_nano: startUnixNano,
          time_unix_nano: timeUnixNano,
          as_double: Number(gauge.value || 0),
          attributes: attrs,
        }],
      },
    });
  }
  for (const histogram of snapshot.histograms || []) {
    const snap = histogram.snap || {};
    const attrs = [...resourceAttributes(cfg), ...sortedObjectAttributes(histogram.labels || {})];
    out.push({
      name: histogram.name,
      description: histogram.help || '',
      histogram: {
        aggregation_temporality: 'AGGREGATION_TEMPORALITY_CUMULATIVE',
        data_points: [{
          start_time_unix_nano: startUnixNano,
          time_unix_nano: timeUnixNano,
          count: Number(snap.total || 0),
          sum: Number(snap.sum || 0),
          bucket_counts: histogramBucketCounts(snap),
          explicit_bounds: [...(snap.bounds || [])],
          attributes: attrs,
          min: Number(snap.min || 0),
          max: Number(snap.max || 0),
        }],
      },
    });
  }
  return out;
}

function makeHolonObservabilityHandlers(obs = current()) {
  return {
    Logs(call) {
      if (!obs.enabled(Family.LOGS) || !obs.logRing) {
        failStream(call, 'logs family is not enabled (OP_OBS)');
        return;
      }
      const request = call.request || {};
      const minSeverity = normalizeSeverity(request.min_severity_number) || Level.INFO;
      const cutoff = request.since ? Date.now() / 1000 - durationSeconds(request.since) : 0;
      const sessionIds = request.session_ids || [];
      const rpcMethods = request.rpc_methods || [];
      if (!request.follow) {
        const records = cutoff ? obs.logRing.drainSince(cutoff) : obs.logRing.drain();
        for (const record of records) {
          if (matchLog(record, minSeverity, sessionIds, rpcMethods)) call.write(toProtoLogRecord(record));
        }
        call.end();
        return;
      }
      let closed = false;
      const { snapshot, stop } = obs.logRing.subscribeWithSnapshot((record) => {
        if (closed || !matchLog(record, minSeverity, sessionIds, rpcMethods)) return;
        call.write(toProtoLogRecord(record));
      });
      for (const record of snapshot) {
        if (closed) break;
        if (cutoff && recordSeconds(record) < cutoff) continue;
        if (matchLog(record, minSeverity, sessionIds, rpcMethods)) call.write(toProtoLogRecord(record));
      }
      const cleanup = () => {
        if (closed) return;
        closed = true;
        stop();
      };
      call.on('cancelled', cleanup);
      call.on('error', cleanup);
      call.on('close', cleanup);
    },

    Metrics(call) {
      if (!obs.enabled(Family.METRICS) || !obs.registry) {
        failStream(call, 'metrics family is not enabled (OP_OBS)');
        return;
      }
      const request = call.request || {};
      let metrics = toProtoMetrics(obs.registry.snapshot(), obs.cfg, obs.startUnixNano);
      if (request.name_prefixes && request.name_prefixes.length) {
        const prefixes = request.name_prefixes.filter(Boolean);
        metrics = metrics.filter((metric) => prefixes.some((prefix) => metric.name.startsWith(prefix)));
      }
      for (const metric of metrics) call.write(metric);
      call.end();
    },

    Events(call) {
      if (!obs.enabled(Family.EVENTS) || !obs.eventBus) {
        failStream(call, 'events family is not enabled (OP_OBS)');
        return;
      }
      const request = call.request || {};
      const wanted = new Set((request.event_names || []).filter(Boolean));
      const cutoff = request.since ? Date.now() / 1000 - durationSeconds(request.since) : 0;
      if (!request.follow) {
        const records = cutoff ? obs.eventBus.drainSince(cutoff) : obs.eventBus.drain();
        for (const record of records) {
          if (matchEvent(record, wanted)) call.write(toProtoLogRecord(record));
        }
        call.end();
        return;
      }
      let closed = false;
      const { snapshot, stop } = obs.eventBus.subscribeWithSnapshot((record) => {
        if (closed || !matchEvent(record, wanted)) return;
        call.write(toProtoLogRecord(record));
      });
      for (const record of snapshot) {
        if (closed) break;
        if (cutoff && recordSeconds(record) < cutoff) continue;
        if (matchEvent(record, wanted)) call.write(toProtoLogRecord(record));
      }
      const cleanup = () => {
        if (closed) return;
        closed = true;
        stop();
      };
      call.on('cancelled', cleanup);
      call.on('error', cleanup);
      call.on('close', cleanup);
    },
  };
}

function registerService(server, obs = current()) {
  server.addService(observabilityWire.HOLON_OBSERVABILITY_SERVICE_DEF, makeHolonObservabilityHandlers(obs));
}

function matchLog(record, minSeverity, sessionIds, rpcMethods) {
  if (record.private) return false;
  if (normalizeSeverity(record.severity_number) < minSeverity) return false;
  if (sessionIds && sessionIds.length && !sessionIds.includes(stringAttribute(record, Attr.HOLONS_SESSION_ID))) return false;
  if (rpcMethods && rpcMethods.length && !rpcMethods.includes(stringAttribute(record, Attr.RPC_METHOD))) return false;
  return true;
}

function matchEvent(record, wanted) {
  if (record.private) return false;
  if (!wanted || wanted.size === 0) return true;
  return wanted.has(record.event_name || '');
}

function failedPrecondition(message) {
  const err = new Error(message);
  err.code = grpc.status.FAILED_PRECONDITION;
  err.details = message;
  return err;
}

function failStream(call, message) {
  const err = failedPrecondition(message);
  if (typeof call.destroy === 'function') {
    call.destroy(err);
    return;
  }
  call.emit('error', err);
}

// --- Prometheus exposition ----------------------------------------------------

class PromServer {
  constructor(addr = ':0') {
    this.addr = addr || ':0';
    this.server = null;
  }

  async start() {
    if (this.server) return this.addrURL();
    const { host, port } = parsePromAddr(this.addr);
    this.server = http.createServer((req, res) => {
      const pathname = (req.url || '').split('?', 1)[0];
      if (pathname !== '/metrics') {
        res.writeHead(404);
        res.end();
        return;
      }
      const obs = current();
      let status = 200;
      let body = '';
      if (!obs.enabled(Family.METRICS)) {
        status = 503;
        body = '# metrics family disabled (OP_OBS)\n';
      } else if (!obs.enabled(Family.PROM)) {
        status = 503;
        body = '# prom family disabled (OP_OBS)\n';
      } else {
        body = toPrometheusText(obs);
      }
      res.writeHead(status, {
        'content-type': 'text/plain; version=0.0.4',
        'content-length': Buffer.byteLength(body),
      });
      res.end(body);
    });
    await new Promise((resolve, reject) => {
      this.server.once('error', reject);
      this.server.listen(port, host, () => {
        this.server.off('error', reject);
        resolve();
      });
    });
    return this.addrURL();
  }

  addrURL() {
    if (!this.server || !this.server.address()) return '';
    const address = this.server.address();
    return `http://${advertisedPromHost(address.address)}:${address.port}/metrics`;
  }

  async close() {
    const server = this.server;
    this.server = null;
    if (!server) return;
    await new Promise((resolve) => server.close(() => resolve()));
  }
}

function toPrometheusText(obs) {
  if (!obs.enabled(Family.METRICS) || !obs.registry) {
    return '# metrics family disabled (OP_OBS)\n';
  }
  const snapshot = obs.registry.snapshot();
  const groups = new Map();
  const ensure = (name, help, type) => {
    let group = groups.get(name);
    if (!group) {
      group = { name, help, type, counters: [], gauges: [], histograms: [] };
      groups.set(name, group);
    }
    if (!group.help && help) group.help = help;
    return group;
  };
  for (const counter of snapshot.counters || []) ensure(counter.name, counter.help, 'counter').counters.push(counter);
  for (const gauge of snapshot.gauges || []) ensure(gauge.name, gauge.help, 'gauge').gauges.push(gauge);
  for (const histogram of snapshot.histograms || []) ensure(histogram.name, histogram.help, 'histogram').histograms.push(histogram);

  const injected = { slug: obs.cfg.slug || '' };
  if (obs.cfg.instanceUid) injected.instance_uid = obs.cfg.instanceUid;
  const lines = [];
  for (const name of [...groups.keys()].sort()) {
    const group = groups.get(name);
    lines.push(`# HELP ${name} ${promEscapeHelp(group.help || '')}`);
    lines.push(`# TYPE ${name} ${group.type}`);
    for (const counter of group.counters) {
      lines.push(`${counter.name}${promLabels(mergeLabels(counter.labels, injected))} ${Number(counter.value || 0)}`);
    }
    for (const gauge of group.gauges) {
      lines.push(`${gauge.name}${promLabels(mergeLabels(gauge.labels, injected))} ${formatPromFloat(gauge.value || 0)}`);
    }
    for (const histogram of group.histograms) {
      const labels = mergeLabels(histogram.labels, injected);
      const snap = histogram.snap || {};
      const bounds = snap.bounds || [];
      const counts = snap.counts || [];
      for (let i = 0; i < bounds.length; i += 1) {
        lines.push(`${histogram.name}_bucket${promLabels({ ...labels, le: formatPromFloat(bounds[i]) })} ${Number(counts[i] || 0)}`);
      }
      lines.push(`${histogram.name}_bucket${promLabels({ ...labels, le: '+Inf' })} ${Number(snap.total || 0)}`);
      lines.push(`${histogram.name}_sum${promLabels(labels)} ${formatPromFloat(snap.sum || 0)}`);
      lines.push(`${histogram.name}_count${promLabels(labels)} ${Number(snap.total || 0)}`);
    }
  }
  return lines.length ? `${lines.join('\n')}\n` : '';
}

function parsePromAddr(raw) {
  const trimmed = String(raw || ':0').trim() || ':0';
  if (trimmed.startsWith(':')) {
    return { host: '0.0.0.0', port: Number(trimmed.slice(1) || 0) };
  }
  const idx = trimmed.lastIndexOf(':');
  if (idx < 0) throw new Error(`invalid Prometheus address "${raw}"`);
  return { host: trimmed.slice(0, idx) || '0.0.0.0', port: Number(trimmed.slice(idx + 1)) };
}

function advertisedPromHost(host) {
  if (!host || host === '0.0.0.0') return '127.0.0.1';
  if (host === '::') return '::1';
  return host;
}

function mergeLabels(base, extra) {
  const out = {};
  for (const [key, value] of Object.entries(extra || {})) {
    if (value !== undefined && value !== null && String(value) !== '') out[key] = String(value);
  }
  for (const [key, value] of Object.entries(base || {})) out[key] = String(value);
  return out;
}

function promLabels(labels) {
  const keys = Object.keys(labels || {}).sort();
  if (!keys.length) return '';
  return `{${keys.map((key) => `${key}="${promEscapeValue(labels[key])}"`).join(',')}}`;
}

function promEscapeValue(value) {
  return String(value).replace(/\\/g, '\\\\').replace(/\n/g, '\\n').replace(/"/g, '\\"');
}

function promEscapeHelp(value) {
  return String(value).replace(/\\/g, '\\\\').replace(/\n/g, '\\n');
}

function formatPromFloat(value) {
  const number = Number(value);
  if (number === Infinity) return '+Inf';
  if (number === -Infinity) return '-Inf';
  if (Number.isNaN(number)) return 'NaN';
  return String(number);
}

// --- Member observability relay ---------------------------------------------

class MemberRelay {
  constructor({ childSlug, childUid, client, observability = current(), retryDelayMs = 2000 }) {
    this.childSlug = childSlug || '';
    this.childUid = childUid || '';
    this.client = client;
    this.observability = observability;
    this.retryDelayMs = retryDelayMs;
    this.stopped = false;
    this.active = new Set();
  }

  start() {
    const obs = this.observability;
    if (obs.enabled(Family.LOGS) && obs.logRing) this.pumpLogs();
    if (obs.enabled(Family.EVENTS) && obs.eventBus) this.pumpEvents();
  }

  stop() {
    this.stopped = true;
    for (const stream of [...this.active]) {
      if (typeof stream.cancel === 'function') stream.cancel();
      if (typeof stream.destroy === 'function') stream.destroy();
    }
    this.active.clear();
  }

  pumpLogs() {
    if (this.stopped) return;
    const stream = this.client.Logs({ follow: true });
    this.active.add(stream);
    const restart = () => {
      this.active.delete(stream);
      if (!this.stopped) setTimeout(() => this.pumpLogs(), this.retryDelayMs);
    };
    stream.on('data', (proto) => {
      const obs = this.observability;
      if (!obs.enabled(Family.LOGS) || !obs.logRing) return;
      const record = fromProtoLogRecord(proto);
      record.chain = appendDirectChild(record.chain, this.childSlug);
      obs.logRing.push(record);
    });
    stream.on('error', restart);
    stream.on('end', restart);
    stream.on('close', restart);
  }

  pumpEvents() {
    if (this.stopped) return;
    const stream = this.client.Events({ follow: true });
    this.active.add(stream);
    const restart = () => {
      this.active.delete(stream);
      if (!this.stopped) setTimeout(() => this.pumpEvents(), this.retryDelayMs);
    };
    stream.on('data', (proto) => {
      const obs = this.observability;
      if (!obs.enabled(Family.EVENTS) || !obs.eventBus) return;
      const record = fromProtoLogRecord(proto);
      record.chain = appendDirectChild(record.chain, this.childSlug);
      obs.eventBus.emit(record);
    });
    stream.on('error', restart);
    stream.on('end', restart);
    stream.on('close', restart);
  }
}

// --- Disk writers ------------------------------------------------------------

function enableDiskWriters(runDir) {
  const obs = _current;
  if (!obs || !runDir) return;
  obs.cfg.runDir = runDir;
  fs.mkdirSync(runDir, { recursive: true });
  if (obs.enabled(Family.LOGS) && obs.logRing) {
    const fp = path.join(runDir, 'stdout.log');
    obs.logRing.subscribe((record) => {
      const rec = logDiskRecord(record);
      try { fs.appendFileSync(fp, JSON.stringify(rec) + '\n'); } catch (_) {}
    });
  }
  if (obs.enabled(Family.EVENTS) && obs.eventBus) {
    const fp = path.join(runDir, 'events.jsonl');
    obs.eventBus.subscribe((record) => {
      const rec = eventDiskRecord(record);
      try { fs.appendFileSync(fp, JSON.stringify(rec) + '\n'); } catch (_) {}
    });
  }
}

function logDiskRecord(record) {
  const rec = {
    kind: 'log',
    ts: new Date(recordSeconds(record) * 1000).toISOString(),
    level: record.severity_text || levelName(record.severity_number),
    slug: stringAttribute(record, Attr.HOLONS_SLUG),
    instance_uid: stringAttribute(record, Attr.HOLONS_INSTANCE_UID),
    message: bodyString(record),
  };
  const sessionId = stringAttribute(record, Attr.HOLONS_SESSION_ID);
  const rpcMethod = stringAttribute(record, Attr.RPC_METHOD);
  const caller = stringAttribute(record, Attr.CODE_CALLER);
  const fields = userAttributes(record);
  if (sessionId) rec.session_id = sessionId;
  if (rpcMethod) rec.rpc_method = rpcMethod;
  if (Object.keys(fields).length) rec.fields = fields;
  if (caller) rec.caller = caller;
  if (record.chain && record.chain.length) rec.chain = record.chain;
  return rec;
}

function eventDiskRecord(record) {
  const rec = {
    kind: 'event',
    ts: new Date(recordSeconds(record) * 1000).toISOString(),
    event_name: record.event_name || '',
    slug: stringAttribute(record, Attr.HOLONS_SLUG),
    instance_uid: stringAttribute(record, Attr.HOLONS_INSTANCE_UID),
  };
  const sessionId = stringAttribute(record, Attr.HOLONS_SESSION_ID);
  const payload = userAttributes(record);
  if (sessionId) rec.session_id = sessionId;
  if (Object.keys(payload).length) rec.payload = payload;
  if (record.chain && record.chain.length) rec.chain = record.chain;
  return rec;
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
  Family,
  Level,
  EventName,
  Attr,
  InvalidTokenError,
  Counter,
  Gauge,
  Histogram,
  Registry,
  LogRing,
  EventBus,
  Logger,
  Observability,
  Private,
  MemberRelay,
  PromServer,
  parseOpObs,
  checkEnv,
  parseLevel,
  levelName,
  appendDirectChild,
  enrichForMultilog,
  configure,
  fromEnv,
  current,
  reset,
  deriveRunDir,
  withSessionContext,
  currentSessionContext,
  toAnyValue,
  keyValue,
  anyValueToString,
  rawAttribute,
  stringAttribute,
  userAttributes,
  bodyString,
  resourceAttributes,
  fromProtoLogRecord,
  toProtoLogRecord,
  toProtoMetrics,
  toPrometheusText,
  makeHolonObservabilityHandlers,
  registerService,
  enableDiskWriters,
  writeMetaJson,
  readMetaJson,
  DEFAULT_BUCKETS,
};
