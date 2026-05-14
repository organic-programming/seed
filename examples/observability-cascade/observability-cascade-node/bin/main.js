#!/usr/bin/env node
'use strict';

const childProcess = require('node:child_process');
const fs = require('node:fs');
const http = require('node:http');
const Module = require('node:module');
const os = require('node:os');
const path = require('node:path');

function findRepoRoot(start) {
  let current = path.resolve(start);
  for (;;) {
    if (fs.existsSync(path.join(current, 'sdk', 'js-holons'))) return current;
    const parent = path.dirname(current);
    if (parent === current) throw new Error('could not locate repository root');
    current = parent;
  }
}

function findCascadeRoot(start) {
  let current = path.resolve(start);
  for (;;) {
    const parent = path.dirname(current);
    if (fs.existsSync(path.join(parent, 'observability-cascade-node-node'))) return parent;
    if (parent === current) throw new Error('could not locate observability-cascade examples root');
    current = parent;
  }
}

const ROOT = findRepoRoot(__dirname);
const CASCADE_ROOT = findCascadeRoot(__dirname);
const NODE_NODE = path.join(CASCADE_ROOT, 'observability-cascade-node-node');
const SDK_ROOT = path.join(ROOT, 'sdk', 'js-holons');
const NODE_NODE_MODULES = path.join(NODE_NODE, '.op', 'build', 'npm', 'node_modules');

process.env.NODE_PATH = [
  NODE_NODE_MODULES,
  path.join(NODE_NODE, 'node_modules'),
  path.join(SDK_ROOT, 'node_modules'),
  process.env.NODE_PATH || '',
].filter(Boolean).join(path.delimiter);
Module._initPaths();

const grpc = require(path.join(NODE_NODE_MODULES, '@grpc/grpc-js'));
const relayPb = require(path.join(NODE_NODE, 'gen/node/relay/v1/relay_pb.js'));
const relayGrpc = require(path.join(NODE_NODE, 'gen/node/relay/v1/relay_grpc_pb.js'));
const observabilityWire = require(path.join(SDK_ROOT, 'src/gen/holons/v1/observability'));
const describeWire = require(path.join(SDK_ROOT, 'src/gen/holons/v1/describe'));

const RUN_PHASES = 4;
const RUN_TICKS = 3;
const ROLE_ORDER = ['D', 'C', 'B', 'A'];
const TRANSPORTS = ['tcp', 'unix', 'tcp', 'unix'];
const NODE_SLUG = 'observability-cascade-node-node';
const GO_SLUG = 'observability-cascade-node-go';

const ObservabilityClient = grpc.makeGenericClientConstructor(
  observabilityWire.HOLON_OBSERVABILITY_SERVICE_DEF,
  'HolonObservability',
  {},
);
const HolonMetaClient = grpc.makeGenericClientConstructor(
  describeWire.HOLON_META_SERVICE_DEF,
  'HolonMeta',
  {},
);

class RoleSpec {
  constructor(slug, binaryPath) {
    this.slug = slug;
    this.binaryPath = binaryPath;
  }
}

class RoleRuntime {
  constructor({ role, uid, slug, binaryPath, listenUri, relayAddress, clientTarget }) {
    this.role = role;
    this.uid = uid;
    this.slug = slug;
    this.binaryPath = binaryPath;
    this.listenUri = listenUri;
    this.relayAddress = relayAddress;
    this.clientTarget = clientTarget;
    this.memberAddress = '';
    this.memberSlug = '';
    this.metricsAddr = '';
    this.process = null;
    this.relayClient = null;
    this.obsClient = null;
    this.stderrPath = path.join(os.tmpdir(), `observability-cascade-node-${process.pid}-${uid}.stderr`);
  }
}

class CheckResult {
  constructor(pass = false, evidence = '') {
    this.pass = pass;
    this.evidence = evidence;
  }
}

class TickOutcome {
  constructor(log, event, metric, metricValue) {
    this.log = log;
    this.event = event;
    this.metric = metric;
    this.metricValue = metricValue;
  }
}

class Cascade {
  constructor(phase, transport, runRoot, roles) {
    this.phase = phase;
    this.transport = transport;
    this.runRoot = runRoot;
    this.roles = roles;
  }

  async runTick(tick, previousMetric) {
    return this.runTickWithSender(`phase-${this.phase}-tick-${tick}`, previousMetric);
  }

  async runTickWithSender(sender, previousMetric) {
    const request = new relayPb.TickRequest();
    request.setSender(sender);
    request.setNote(this.transport);
    try {
      await unary(this.roles.D.relayClient.tick.bind(this.roles.D.relayClient), request, 5000);
    } catch (error) {
      const err = new CheckResult(false, error.message);
      return new TickOutcome(err, err, err, previousMetric);
    }

    const log = await waitFor(3000, () => this.checkLog(sender));
    const event = await waitFor(3000, () => this.checkEvent());
    let metricValue = previousMetric;
    const metric = await waitFor(3000, async () => {
      const [result, value] = await this.checkMetric(previousMetric);
      if (result.pass) metricValue = value;
      return result;
    });
    return new TickOutcome(log, event, metric, metricValue);
  }

  async runLiveTick(streams, streamOpenError, tick, previousMetric) {
    return this.runLiveTickWithSender(streams, streamOpenError, `phase-${this.phase}-tick-${tick}`, previousMetric);
  }

  async runLiveTickWithSender(streams, streamOpenError, sender, previousMetric) {
    const request = new relayPb.TickRequest();
    request.setSender(sender);
    request.setNote(this.transport);
    try {
      await unary(this.roles.D.relayClient.tick.bind(this.roles.D.relayClient), request, 5000);
    } catch (error) {
      const err = new CheckResult(false, error.message);
      return new TickOutcome(err, err, err, previousMetric);
    }

    let log;
    let event;
    if (!streamOpenError && streams) {
      log = await waitFor(1000, () => this.checkLiveLog(streams, sender), 50);
      event = await waitFor(1000, () => this.checkLiveEvent(streams), 50);
    } else {
      const evidence = `stream re-open failed: ${streamOpenError || 'streams not open'}`;
      log = new CheckResult(false, evidence);
      event = new CheckResult(false, evidence);
    }

    let metricValue = previousMetric;
    const metric = await waitFor(1000, async () => {
      const [result, value] = await this.checkMetric(previousMetric);
      if (result.pass) metricValue = value;
      return result;
    }, 50);
    return new TickOutcome(log, event, metric, metricValue);
  }

  async checkLog(sender) {
    const entries = await readLogs(this.roles.A.obsClient);
    for (const entry of entries) {
      if (entry.message !== 'tick received') continue;
      if ((entry.fields || {}).sender !== sender) continue;
      if ((entry.fields || {}).responder_uid !== this.roles.D.uid) continue;
      const err = this.checkChain(entry.chain || []);
      if (err) return new CheckResult(false, `matching log has bad chain: ${err} entry=${JSON.stringify(entry)}`);
      return new CheckResult(true, JSON.stringify(entry));
    }
    return new CheckResult(false, `no relayed D tick log for sender=${sender} in ${entries.length} A log entries`);
  }

  async checkEvent() {
    const events = await readEvents(this.roles.A.obsClient);
    for (const event of events) {
      if (event.type !== 'INSTANCE_READY' || event.instance_uid !== this.roles.D.uid) continue;
      const err = this.checkChain(event.chain || []);
      if (err) return new CheckResult(false, `matching event has bad chain: ${err} event=${JSON.stringify(event)}`);
      return new CheckResult(true, JSON.stringify(event));
    }
    return new CheckResult(false, `no relayed D INSTANCE_READY event in ${events.length} A events`);
  }

  checkLiveLog(streams, sender) {
    const entries = streams.logEntries();
    for (const entry of entries) {
      if (entry.message !== 'tick received') continue;
      if ((entry.fields || {}).sender !== sender) continue;
      if ((entry.fields || {}).responder_uid !== this.roles.D.uid) continue;
      const err = this.checkChain(entry.chain || []);
      if (err) return new CheckResult(false, `matching live log has bad chain: ${err} entry=${JSON.stringify(entry)}`);
      return new CheckResult(true, JSON.stringify(entry));
    }
    return new CheckResult(false, `no live log found for sender=${sender}; buffer=${entries.length} errors=${JSON.stringify(streams.errors())}`);
  }

  checkLiveEvent(streams) {
    const events = streams.eventEntries();
    for (const event of events) {
      if (event.type !== 'INSTANCE_READY' || event.instance_uid !== this.roles.D.uid) continue;
      const err = this.checkChain(event.chain || []);
      if (err) return new CheckResult(false, `matching live event has bad chain: ${err} event=${JSON.stringify(event)}`);
      return new CheckResult(true, JSON.stringify(event));
    }
    return new CheckResult(false, `no live INSTANCE_READY event for D; buffer=${events.length} errors=${JSON.stringify(streams.errors())}`);
  }

  async checkMetric(previous) {
    try {
      const body = await fetchMetrics(this.roles.D.metricsAddr);
      const value = parseCascadeTicks(body, this.roles.D.uid);
      if (value === null) return [new CheckResult(false, body), previous];
      if (value <= previous) {
        return [new CheckResult(false, `cascade_ticks_total=${value} did not increase beyond ${previous}\n${body}`), value];
      }
      return [new CheckResult(true, `cascade_ticks_total=${value}`), value];
    } catch (error) {
      return [new CheckResult(false, error.message), previous];
    }
  }

  checkChain(chain) {
    for (let index = 0; index < 3; index += 1) {
      const role = ['D', 'C', 'B'][index];
      if (index >= chain.length) return `chain length ${chain.length} < 3`;
      const hop = chain[index];
      const want = this.roles[role];
      if (hop.slug !== want.slug || hop.instance_uid !== want.uid) {
        return `hop ${index} = ${hop.slug}/${hop.instance_uid}, want ${want.slug}/${want.uid}`;
      }
    }
    return '';
  }

  stop() {
    for (const role of [...ROLE_ORDER].reverse()) {
      const runtime = this.roles[role];
      if (runtime.relayClient) runtime.relayClient.close();
      if (runtime.obsClient) runtime.obsClient.close();
      if (runtime.process && !runtime.process.killed) runtime.process.kill('SIGTERM');
    }
    for (const runtime of Object.values(this.roles)) {
      if (runtime.process && !runtime.process.killed) runtime.process.kill('SIGKILL');
    }
  }
}

class LiveStreams {
  constructor(client) {
    this.client = client;
    this.logs = [];
    this.events = [];
    this.errs = [];
    this.streams = [];
  }

  start() {
    const logStream = this.client.Logs({ min_level: 'INFO', follow: true });
    const eventStream = this.client.Events({ follow: true });
    this.streams = [logStream, eventStream];
    logStream.on('data', (entry) => this.logs.push(entry));
    logStream.on('error', (error) => this.errs.push(`logs stream ended: ${error.message}`));
    eventStream.on('data', (event) => this.events.push(event));
    eventStream.on('error', (error) => this.errs.push(`events stream ended: ${error.message}`));
  }

  stop() {
    for (const stream of this.streams) {
      if (typeof stream.cancel === 'function') stream.cancel();
      if (typeof stream.destroy === 'function') stream.destroy();
    }
  }

  logEntries() { return [...this.logs]; }
  eventEntries() { return [...this.events]; }
  errors() { return [...this.errs]; }
}

async function main() {
  try {
    if (process.argv.includes('--multi-pattern')) await runMultiPattern();
    else if (process.argv.includes('--live-stream')) await runLiveStream();
    else await runDefault();
    return 0;
  } catch (error) {
    process.stderr.write(`\nFAIL: ${error.message}\n`);
    return 1;
  }
}

async function runDefault() {
  const binary = findBinary(NODE_SLUG);
  const runRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'observability-cascade-node-'));
  console.log('=== observability-cascade-node ===');
  console.log();
  let totalPass = 0;
  let totalFail = 0;
  let previous = '';
  for (let index = 0; index < TRANSPORTS.length; index += 1) {
    const phase = index + 1;
    const transport = TRANSPORTS[index];
    console.log(previous ? `Phase ${phase}/${RUN_PHASES}: transport=${transport} (switching from ${previous})` : `Phase ${phase}/${RUN_PHASES}: transport=${transport}`);
    const started = performanceNow();
    let cascade;
    try {
      const specs = Object.fromEntries(ROLE_ORDER.map((role) => [role, new RoleSpec(NODE_SLUG, binary)]));
      cascade = await spawnCascade(phase, transport, specs, runRoot);
    } catch (error) {
      totalFail += RUN_TICKS;
      console.log(`  spawn FAIL: ${error.message}\n`);
      previous = transport;
      continue;
    }
    console.log(`  spawned 4 nodes in ${elapsed(started)}`);
    let previousMetric = 0;
    for (let tick = 1; tick <= RUN_TICKS; tick += 1) {
      const tickStart = performanceNow();
      const outcome = await cascade.runTick(tick, previousMetric);
      if (outcome.metric.pass) previousMetric = outcome.metricValue;
      const overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
      if (overall) totalPass += 1; else totalFail += 1;
      console.log(`  Tick ${tick}/${RUN_TICKS}: log ${passText(outcome.log.pass)}, event ${passText(outcome.event.pass)}, metric ${passText(outcome.metric.pass)} (overall ${passText(overall)} in ${elapsed(tickStart)})`);
      printFailureEvidence('log', outcome.log);
      printFailureEvidence('event', outcome.event);
      printFailureEvidence('metric', outcome.metric);
    }
    cascade.stop();
    console.log();
    previous = transport;
  }
  console.log(`Summary: ${totalPass + totalFail} ticks, ${totalPass} PASS, ${totalFail} FAIL`);
  if (totalFail) throw new Error(`${totalFail} tick(s) failed`);
}

async function runLiveStream() {
  const binary = findBinary(NODE_SLUG);
  const runRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'observability-cascade-node-live-'));
  console.log('=== observability-cascade-node --live-stream ===');
  console.log();
  console.log('Setup: opening long-lived Follow:true streams on A');
  console.log('       (initial transport: tcp)');
  console.log();
  let totalPass = 0;
  let totalFail = 0;
  let cascade = null;
  let streams = null;
  const specs = Object.fromEntries(ROLE_ORDER.map((role) => [role, new RoleSpec(NODE_SLUG, binary)]));
  for (let index = 0; index < TRANSPORTS.length; index += 1) {
    const phase = index + 1;
    const transport = TRANSPORTS[index];
    if (phase === 1) {
      console.log(`Phase ${phase}/${RUN_PHASES}: initial chain (${transport})`);
    } else {
      console.log(`Phase ${phase}/${RUN_PHASES}: respawn on ${transport}`);
      const killStart = performanceNow();
      if (streams) streams.stop();
      if (cascade) cascade.stop();
      console.log(`  killed 4 nodes in ${elapsed(killStart)}`);
    }
    const spawnStart = performanceNow();
    let phaseCascade;
    try {
      phaseCascade = await spawnCascade(phase, transport, specs, runRoot);
    } catch (error) {
      totalFail += RUN_TICKS;
      console.log(`  spawn FAIL: ${error.message}\n`);
      streams = null;
      continue;
    }
    console.log(`  spawned 4 nodes in ${elapsed(spawnStart)}`);
    if (phase > 1) console.log('  re-opening Follow:true streams on new A');
    let streamError = null;
    try {
      streams = new LiveStreams(phaseCascade.roles.A.obsClient);
      streams.start();
    } catch (error) {
      streams = null;
      streamError = error.message;
      console.log(`  stream re-open failed: ${error.message}`);
    }
    let previousMetric = 0;
    for (let tick = 1; tick <= RUN_TICKS; tick += 1) {
      const tickStart = performanceNow();
      const outcome = await phaseCascade.runLiveTick(streams, streamError, tick, previousMetric);
      if (outcome.metric.pass) previousMetric = outcome.metricValue;
      const overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
      if (overall) totalPass += 1; else totalFail += 1;
      console.log(`  Tick ${tick}/${RUN_TICKS}: log ${passText(outcome.log.pass)}, event ${passText(outcome.event.pass)}, metric ${passText(outcome.metric.pass)} (overall ${passText(overall)} in ${elapsed(tickStart)})`);
      printFailureEvidence('log', outcome.log);
      printFailureEvidence('event', outcome.event);
      printFailureEvidence('metric', outcome.metric);
    }
    console.log();
    cascade = phaseCascade;
  }
  if (streams) streams.stop();
  if (cascade) cascade.stop();
  console.log(`Summary: ${totalPass} PASS / ${totalFail} FAIL across ${totalPass + totalFail} ticks`);
  if (totalFail) throw new Error(`${totalFail} tick(s) failed`);
}

async function runMultiPattern() {
  const nodeBinary = findBinary(NODE_SLUG);
  const goBinary = findBinary(GO_SLUG);
  const patterns = [
    ['node-node-node-node', Object.fromEntries(ROLE_ORDER.map((role) => [role, new RoleSpec(NODE_SLUG, nodeBinary)]))],
    ['node-node-go-node', {
      A: new RoleSpec(NODE_SLUG, nodeBinary), B: new RoleSpec(NODE_SLUG, nodeBinary),
      C: new RoleSpec(GO_SLUG, goBinary), D: new RoleSpec(NODE_SLUG, nodeBinary),
    }],
    ['node-node-go-go', {
      A: new RoleSpec(NODE_SLUG, nodeBinary), B: new RoleSpec(NODE_SLUG, nodeBinary),
      C: new RoleSpec(GO_SLUG, goBinary), D: new RoleSpec(GO_SLUG, goBinary),
    }],
  ];
  const runRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'observability-cascade-node-multi-'));
  console.log('=== observability-cascade-node (multi-pattern) ===');
  console.log();
  let totalPass = 0;
  let totalFail = 0;
  for (let patternIndex = 0; patternIndex < patterns.length; patternIndex += 1) {
    const [name, specs] = patterns[patternIndex];
    console.log(`Pattern ${patternIndex + 1}/${patterns.length}: ${name}`);
    let patternPass = 0;
    for (let index = 0; index < TRANSPORTS.length; index += 1) {
      const phase = index + 1;
      const transport = TRANSPORTS[index];
      const started = performanceNow();
      let cascade;
      try {
        cascade = await spawnCascade(phase, transport, specs, runRoot);
      } catch (error) {
        totalFail += RUN_TICKS;
        console.log(`  Phase ${phase}/${RUN_PHASES} (${transport}): spawn FAIL (${error.message})`);
        continue;
      }
      let streamError = null;
      let streams = null;
      try {
        streams = new LiveStreams(cascade.roles.A.obsClient);
        streams.start();
        const ready = await waitFor(5000, () => cascade.checkLiveEvent(streams), 50);
        if (!ready.pass) streamError = `live relay readiness: ${ready.evidence}`;
      } catch (error) {
        streamError = error.message;
      }
      let previousMetric = 0;
      const results = [];
      const evidence = [];
      for (let tick = 1; tick <= RUN_TICKS; tick += 1) {
        const sender = `${name}-phase-${phase}-tick-${tick}`;
        const outcome = await cascade.runLiveTickWithSender(streams, streamError, sender, previousMetric);
        if (outcome.metric.pass) previousMetric = outcome.metricValue;
        const overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
        if (overall) {
          patternPass += 1;
          totalPass += 1;
          results.push(`Tick ${tick} PASS`);
        } else {
          totalFail += 1;
          results.push(`Tick ${tick} FAIL (${failureSummary(outcome)})`);
          evidence.push(`      Tick ${tick} evidence: ${compactEvidence(outcome)}`);
        }
      }
      console.log(`  Phase ${phase}/${RUN_PHASES} (${transport}): ${results.join(', ')} (spawned in ${elapsed(started)})`);
      evidence.forEach((line) => console.log(line));
      if (streams) streams.stop();
      cascade.stop();
    }
    console.log(`  Subtotal: ${patternPass}/12 PASS`);
    console.log();
  }
  console.log(`Summary: ${totalPass} PASS / ${totalFail} FAIL across ${totalPass + totalFail} ticks`);
  if (totalFail) throw new Error(`${totalFail} tick(s) failed`);
}

async function spawnCascade(phase, transport, specs, runRoot) {
  const roles = {};
  for (const role of ROLE_ORDER) roles[role] = newRoleRuntime(phase, transport, role, specs[role]);
  for (const runtime of Object.values(roles)) {
    fs.rmSync(path.join(runRoot, runtime.slug, runtime.uid), { recursive: true, force: true });
  }
  const cascade = new Cascade(phase, transport, runRoot, roles);
  try {
    for (const role of ROLE_ORDER) {
      const runtime = roles[role];
      const child = childRole(role);
      if (child) {
        runtime.memberAddress = roles[child].relayAddress;
        runtime.memberSlug = roles[child].slug;
      }
      await startRole(cascade, runtime);
    }
  } catch (error) {
    cascade.stop();
    throw error;
  }
  await sleep(150);
  return cascade;
}

function newRoleRuntime(phase, transport, role, spec) {
  const uid = `relay-p${String(phase).padStart(2, '0')}-${role.toLowerCase()}`;
  if (transport === 'tcp') {
    return new RoleRuntime({
      role, uid, slug: spec.slug, binaryPath: spec.binaryPath,
      listenUri: 'tcp://127.0.0.1:0',
      relayAddress: '',
      clientTarget: '',
    });
  }
  if (transport === 'unix') {
    const socketPath = `/tmp/observability-cascade-node-p${phase}-${role.toLowerCase()}-${process.pid}.sock`;
    fs.rmSync(socketPath, { force: true });
    return new RoleRuntime({
      role, uid, slug: spec.slug, binaryPath: spec.binaryPath,
      listenUri: `unix://${socketPath}`,
      relayAddress: `unix://${socketPath}`,
      clientTarget: `unix://${socketPath}`,
    });
  }
  throw new Error(`unknown transport ${transport}`);
}

async function startRole(cascade, runtime) {
  const args = [runtime.binaryPath, 'serve', '--listen', runtime.listenUri];
  if (runtime.memberAddress) args.push('--member', `${runtime.memberSlug}=${runtime.memberAddress}`);
  const stderr = fs.openSync(runtime.stderrPath, 'w');
  runtime.process = childProcess.spawn(args[0], args.slice(1), {
    cwd: ROOT,
    env: {
      ...process.env,
      OP_OBS: 'logs,events,metrics,prom',
      OP_RUN_DIR: cascade.runRoot,
      OP_INSTANCE_UID: runtime.uid,
      OP_ORGANISM_UID: cascade.roles.A.uid,
      OP_ORGANISM_SLUG: cascade.roles.A.slug,
      OP_PROM_ADDR: '127.0.0.1:0',
    },
    stdio: ['ignore', 'ignore', stderr],
  });
  runtime.process.once('exit', () => {
    try { fs.closeSync(stderr); } catch (_) {}
  });
  try {
    const meta = await waitMeta(cascade.runRoot, runtime.slug, runtime.uid, 10000);
    runtime.metricsAddr = meta.metrics_addr;
    runtime.relayAddress = meta.address;
    runtime.clientTarget = normalizeDialTarget(meta.address);
    runtime.relayClient = new relayGrpc.RelayServiceClient(runtime.clientTarget, grpc.credentials.createInsecure());
    runtime.obsClient = new ObservabilityClient(runtime.clientTarget, grpc.credentials.createInsecure());
    await dialReady(runtime.clientTarget, 10000);
  } catch (error) {
    const stderrText = fs.existsSync(runtime.stderrPath) ? fs.readFileSync(runtime.stderrPath, 'utf8') : '';
    const detail = [error.message, stderrText].filter(Boolean).join('\n');
    throw new Error(`start ${runtime.role}: ${detail}`);
  }
}

function normalizeDialTarget(uri) {
  if (!String(uri).includes('://')) return uri;
  if (uri.startsWith('tcp://')) {
    const rest = uri.slice('tcp://'.length);
    const idx = rest.lastIndexOf(':');
    let host = idx >= 0 ? rest.slice(0, idx) : rest;
    const port = idx >= 0 ? rest.slice(idx + 1) : '';
    if (!host || host === '0.0.0.0' || host === '::') host = '127.0.0.1';
    return `${host}:${port}`;
  }
  if (uri.startsWith('unix://')) return uri;
  return uri;
}

function childRole(role) {
  return { A: 'B', B: 'C', C: 'D' }[role] || '';
}

async function waitMeta(runRoot, slug, uid, timeoutMs) {
  const metaPath = path.join(runRoot, slug, uid, 'meta.json');
  const deadline = Date.now() + timeoutMs;
  let lastError = null;
  while (Date.now() < deadline) {
    try {
      const data = JSON.parse(fs.readFileSync(metaPath, 'utf8'));
      if (data.uid === uid && data.metrics_addr) return data;
    } catch (error) {
      lastError = error;
    }
    await sleep(50);
  }
  throw new Error(`meta not ready for ${slug}/${uid}: ${lastError && lastError.message}`);
}

async function dialReady(target, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let lastError = null;
  while (Date.now() < deadline) {
    const client = new HolonMetaClient(target, grpc.credentials.createInsecure());
    try {
      await unary(client.Describe.bind(client), {}, 500);
      client.close();
      return;
    } catch (error) {
      lastError = error;
      client.close();
      await sleep(50);
    }
  }
  throw new Error(`dial ${target}: ${lastError && lastError.message}`);
}

function readLogs(client) {
  return collectStream(client.Logs.bind(client), { min_level: 'INFO', follow: false }, 2000);
}

function readEvents(client) {
  return collectStream(client.Events.bind(client), { follow: false }, 2000);
}

function collectStream(method, request, timeoutMs) {
  return new Promise((resolve, reject) => {
    const out = [];
    const stream = method(request);
    const timer = setTimeout(() => {
      if (typeof stream.cancel === 'function') stream.cancel();
      resolve(out);
    }, timeoutMs);
    stream.on('data', (entry) => out.push(entry));
    stream.on('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    stream.on('end', () => {
      clearTimeout(timer);
      resolve(out);
    });
  });
}

function unary(method, request, timeoutMs) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('timeout')), timeoutMs);
    method(request, (error, response) => {
      clearTimeout(timer);
      if (error) reject(error);
      else resolve(response || {});
    });
  });
}

function fetchMetrics(addr) {
  return new Promise((resolve, reject) => {
    const req = http.get(addr, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => { body += chunk; });
      res.on('end', () => resolve(body));
    });
    req.setTimeout(2000, () => {
      req.destroy(new Error('metrics timeout'));
    });
    req.on('error', reject);
  });
}

function parseCascadeTicks(body, uid) {
  const needle = `responder_uid="${uid}"`;
  for (const line of body.split(/\r?\n/)) {
    if (!line.startsWith('cascade_ticks_total{') || !line.includes(needle)) continue;
    const parts = line.trim().split(/\s+/);
    if (parts.length >= 2) return Number(parts[parts.length - 1]);
  }
  return null;
}

async function waitFor(timeoutMs, fn, intervalMs = 100) {
  const deadline = Date.now() + timeoutMs;
  let last = new CheckResult(false, '');
  while (true) {
    last = await fn();
    if (last.pass || Date.now() > deadline) return last;
    await sleep(intervalMs);
  }
}

function findBinary(slug) {
  const envName = `OBSERVABILITY_CASCADE_NODE_${slug.replace(/^observability-cascade-node-/, '').toUpperCase().replace(/-/g, '_')}_BIN`;
  if ((process.env[envName] || '').trim()) return process.env[envName].trim();
  try {
    const out = childProcess.execFileSync('op', ['--bin', slug], { cwd: ROOT, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();
    if (out) return out;
  } catch (_) {}
  const root = path.join(os.homedir(), '.op', 'bin', `${slug}.holon`, 'bin');
  const match = findExecutable(root, slug);
  if (match) return match;
  throw new Error(`${slug} binary not found; run op build ${slug} --install`);
}

function findExecutable(root, name) {
  if (!fs.existsSync(root)) return '';
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const full = path.join(root, entry.name);
    if (entry.isDirectory()) {
      const nested = findExecutable(full, name);
      if (nested) return nested;
    } else if (entry.name === name) {
      try {
        fs.accessSync(full, fs.constants.X_OK);
        return full;
      } catch (_) {}
    }
  }
  return '';
}

function elapsed(start) {
  const seconds = Math.max(0, (performanceNow() - start) / 1000);
  if (seconds < 1) return `${Math.trunc(seconds * 1000)}ms`;
  return `${seconds.toFixed(1)}s`;
}

function passText(value) {
  return value ? 'PASS' : 'FAIL';
}

function printFailureEvidence(family, result) {
  if (!result.pass) console.log(`    ${family} evidence: ${result.evidence || '<empty>'}`);
}

function failureSummary(outcome) {
  const missing = [];
  if (!outcome.log.pass) missing.push('log family');
  if (!outcome.event.pass) missing.push('event family');
  if (!outcome.metric.pass) missing.push('metric family');
  return missing.length ? missing.join(', ') : 'unknown';
}

function compactEvidence(outcome) {
  const parts = [];
  if (!outcome.log.pass) parts.push(`log=${outcome.log.evidence}`);
  if (!outcome.event.pass) parts.push(`event=${outcome.event.evidence}`);
  if (!outcome.metric.pass) parts.push(`metric=${outcome.metric.evidence}`);
  return parts.join(' | ');
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function performanceNow() {
  return Number(process.hrtime.bigint()) / 1e6;
}

main().then((code) => {
  process.exitCode = code;
});
