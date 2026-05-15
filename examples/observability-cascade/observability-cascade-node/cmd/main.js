#!/usr/bin/env node
'use strict';

const childProcess = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

function findRepoRoot(start) {
  let current = path.resolve(start);
  for (;;) {
    if (fs.existsSync(path.join(current, 'sdk', 'node-holons'))) return current;
    const parent = path.dirname(current);
    if (parent === current) return null;
    current = parent;
  }
}

const HERE = path.resolve(path.join(__dirname, '..'));
const ROOT = findRepoRoot(HERE);

const grpc = require('@grpc/grpc-js');
const describeGenerated = require(path.join(HERE, 'gen/describe_generated.js'));
const cascadePb = require(path.join(HERE, 'gen/node/observability_cascade/v1/service_pb.js'));
const cascadeGrpc = require(path.join(HERE, 'gen/node/observability_cascade/v1/service_grpc_pb.js'));
const { composite, describe, observability, relay, serve } = require('@organic-programming/holons');

const RUN_TICKS = 3;
const NODE_SLUG = 'observability-cascade-node-node';
const GO_SLUG = 'observability-cascade-go-node';

class LanguageMember {
  constructor(lang, slug, binary) {
    this.lang = lang;
    this.slug = slug;
    this.binary = binary;
  }
}

class CascadeService {
  async runDefault(_call, callback) {
    callback(null, toCascadeReport(await runReport('default', ownLanguageMembers(), false, false)));
  }

  async runLiveStream(_call, callback) {
    callback(null, toCascadeReport(await runReport('live-stream', ownLanguageMembers(), true, false)));
  }

  async runMultiPattern(_call, callback) {
    callback(null, toMultiPatternReport(await runMultiPatternReport(false)));
  }
}

async function serveComposite(args) {
  describe.useStaticResponse(describeGenerated.staticDescribeResponse());
  const options = serve.parseOptions(args);
  await serve.runWithOptions(normalizeListenUri(options.listenUri), (server) => {
    server.addService(cascadeGrpc.ObservabilityCascadeServiceService, new CascadeService());
  }, {
    reflect: options.reflect,
    slug: 'observability-cascade-node',
  });
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length > 0 && canonicalCommand(args[0]) === 'serve') {
    await serveComposite(args.slice(1));
    return 0;
  }
  if (args.includes('--multi-pattern')) {
    const report = await runMultiPatternReport(true);
    return report.totalFail > 0 ? 1 : 0;
  }
  const live = args.includes('--live-stream');
  const report = await runReport(live ? 'live-stream' : 'default', ownLanguageMembers(), live, true);
  return report.fail > 0 ? 1 : 0;
}

async function runMultiPatternReport(emit) {
  const totalStart = nowMicros();
  const patterns = nodePatterns();
  const out = { patterns: [], totalPass: 0, totalFail: 0, totalElapsedUs: 0 };
  output(emit, '=== observability-cascade-node --multi-pattern ===');
  output(emit);
  for (let index = 0; index < patterns.length; index += 1) {
    const pattern = patterns[index];
    output(emit, `Pattern ${index + 1}/${patterns.length}: ${pattern.name}`);
    const report = await runReport(pattern.name, pattern.members, true, emit);
    out.patterns.push(report);
    out.totalPass += report.pass;
    out.totalFail += report.fail;
    output(emit, `Pattern ${pattern.name}: ${report.pass}/${report.ticks} ${report.fail ? 'FAIL' : 'PASS'} (elapsed=${elapsedText(report.elapsedUs)})`);
    output(emit);
  }
  out.totalElapsedUs = nowMicros() - totalStart;
  output(emit, `Summary: ${out.totalPass} PASS / ${out.totalFail} FAIL across ${out.totalPass + out.totalFail} ticks (total elapsed=${elapsedText(out.totalElapsedUs)})`);
  return out;
}

async function runReport(name, members, live, emit) {
  ensureCascadeObservability();
  const reportStart = nowMicros();
  const report = { name, ticks: 0, pass: 0, fail: 0, phases: [], elapsedUs: 0 };
  const timeoutMs = live ? 1000 : 3000;
  const pollIntervalMs = live ? 50 : 100;
  const runRoot = fs.mkdtempSync(path.join(os.tmpdir(), `observability-cascade-node-${name}-`));
  output(emit, `=== observability-cascade-node ${modeSuffix(name)}===`);
  output(emit);

  for (let phaseIndex = 0; phaseIndex < composite.TransportCoverageSequence.length; phaseIndex += 1) {
    const phaseStart = nowMicros();
    const transportName = composite.TransportCoverageSequence[phaseIndex];
    const from = phaseIndex > 0 ? composite.TransportCoverageSequence[phaseIndex - 1] : transportName;
    const phase = {
      name: `${String(phaseIndex + 1).padStart(2, '0')}-${from}→${transportName}`,
      pass: 0,
      fail: 0,
      failures: [],
      elapsedUs: 0,
    };
    output(emit, `Phase ${phaseIndex + 1}/${composite.TransportCoverageSequence.length}: ${phase.name}`);

    let cascade = null;
    let tickClient = null;
    try {
      cascade = await composite.BuildCascade({
        transport: transportName,
        members: childSpecs(members),
        extraEnv: {
          OP_OBS: 'logs,events,metrics,prom',
          OP_PROM_ADDR: '127.0.0.1:0',
          OP_RUN_DIR: runRoot,
        },
      });
      tickClient = new relay.relayGrpc.RelayServiceClient(cascade.top.target, grpc.credentials.createInsecure());
      const previous = {};
      for (let tick = 1; tick <= RUN_TICKS; tick += 1) {
        const sender = `${name}-phase-${String(phaseIndex + 1).padStart(2, '0')}-tick-${tick}`;
        const result = await runTick(tickClient, sender, transportName, members, previous, timeoutMs, pollIntervalMs);
        if (result.pass) phase.pass += 1;
        else {
          phase.fail += 1;
          phase.failures.push(evidenceLine(tick, result));
        }
        output(emit, `  Tick ${tick}/${RUN_TICKS}: ${result.pass ? 'PASS' : 'FAIL'}`);
        if (emit && !result.pass) process.stderr.write(`    ${evidenceLine(tick, result)}\n`);
      }
    } catch (error) {
      phase.fail += RUN_TICKS;
      for (let tick = 1; tick <= RUN_TICKS; tick += 1) {
        phase.failures.push(`tick=${tick} log=spawn event=spawn hops=${compactEvidence(error.message)}`);
      }
      output(emit, `  spawn FAIL: ${error.message}`);
    } finally {
      if (tickClient) tickClient.close();
      if (cascade) await cascade.stop().catch(() => {});
    }

    phase.elapsedUs = nowMicros() - phaseStart;
    addPhase(report, phase);
    output(emit, `Phase ${phase.name}: ${phase.pass}/${phase.pass + phase.fail} ${phase.fail ? 'FAIL' : 'PASS'} (elapsed=${elapsedText(phase.elapsedUs)})`);
    output(emit);
  }
  report.elapsedUs = nowMicros() - reportStart;
  output(emit, `Summary: ${report.ticks} ticks, ${report.pass} PASS, ${report.fail} FAIL (total elapsed=${elapsedText(report.elapsedUs)})`);
  return report;
}

async function runTick(client, sender, note, members, previous, timeoutMs, pollIntervalMs) {
  const request = new relay.relayPb.TickRequest();
  request.setSender(sender);
  request.setNote(note);
  let response;
  try {
    response = await unary(client.tick.bind(client), request, 5000);
  } catch (error) {
    const out = { pass: false, evidence: compactEvidence(error.message) };
    return { pass: false, log: out, event: out, hops: out };
  }
  const hops = checkHops(response.getHopsList(), members, previous);
  if (!hops.pass) {
    return {
      pass: false,
      hops,
      log: { pass: false, evidence: 'skipped' },
      event: { pass: false, evidence: 'skipped' },
    };
  }
  const expectedChain = response.getHopsList().map((hop) => ({ slug: hop.getSlug(), instance_uid: hop.getUid() }));
  const leafUID = expectedChain[0].instance_uid;
  const log = await composite.CheckRelayedLog({
    sender,
    leafUID,
    expectedChain,
    timeoutMs,
    pollIntervalMs,
  });
  const event = await composite.CheckRelayedEvent({
    eventType: observability.EventType.INSTANCE_READY,
    leafUID,
    expectedChain,
    timeoutMs,
    pollIntervalMs,
  });
  return { pass: hops.pass && log.pass && event.pass, hops, log, event };
}

function checkHops(hops, members, previous) {
  if (hops.length !== members.length) {
    return { pass: false, evidence: `hops length ${hops.length} want ${members.length}` };
  }
  for (let index = 0; index < hops.length; index += 1) {
    const hop = hops[index];
    const want = members[members.length - 1 - index];
    if (hop.getSlug() !== want.slug) return { pass: false, evidence: `hop ${index} slug=${hop.getSlug()} want ${want.slug}` };
    if (!hop.getUid()) return { pass: false, evidence: `hop ${index} uid empty` };
    if (Number(hop.getReceived()) <= Number(previous[hop.getUid()] || 0)) {
      return { pass: false, evidence: `hop ${index} received=${hop.getReceived()} previous=${previous[hop.getUid()] || 0}` };
    }
    previous[hop.getUid()] = Number(hop.getReceived());
  }
  return { pass: true, evidence: 'ok' };
}

function ownLanguageMembers() {
  const nodeBinary = findBinary('node-node', NODE_SLUG);
  return [
    new LanguageMember('node', NODE_SLUG, nodeBinary),
    new LanguageMember('node', NODE_SLUG, nodeBinary),
    new LanguageMember('node', NODE_SLUG, nodeBinary),
  ];
}

function nodePatterns() {
  const nodeBinary = findBinary('node-node', NODE_SLUG);
  const goBinary = findBinary('go-node', GO_SLUG);
  const bins = {
    node: new LanguageMember('node', NODE_SLUG, nodeBinary),
    go: new LanguageMember('go', GO_SLUG, goBinary),
  };
  const names = [
    'node-node-node', 'node-node-go', 'node-go-node', 'node-go-go',
    'go-node-node', 'go-node-go', 'go-go-node', 'go-go-go',
  ];
  return names.map((name) => ({
    name,
    members: name.split('-').map((part) => bins[part]),
  }));
}

function childSpecs(members) {
  return members.map((member) => ({ slug: member.slug, binary: member.binary }));
}

function addPhase(report, phase) {
  report.phases.push(phase);
  report.pass += phase.pass;
  report.fail += phase.fail;
  report.ticks += phase.pass + phase.fail;
}

function ensureCascadeObservability() {
  const current = observability.current();
  if (current.enabled(observability.Family.LOGS) && current.enabled(observability.Family.EVENTS)) return;
  process.env.OP_OBS = 'logs,events,metrics,prom';
  process.env.OP_PROM_ADDR = process.env.OP_PROM_ADDR || '127.0.0.1:0';
  observability.fromEnv({ slug: 'observability-cascade-node' });
}

function toCascadeReport(report) {
  const out = new cascadePb.CascadeReport();
  out.setName(report.name || '');
  out.setTicks(report.ticks);
  out.setPass(report.pass);
  out.setFail(report.fail);
  out.setElapsedUs(report.elapsedUs);
  out.setPhasesList(report.phases.map((phase) => {
    const item = new cascadePb.PhaseResult();
    item.setName(phase.name);
    item.setPass(phase.pass);
    item.setFail(phase.fail);
    item.setFailuresList(phase.failures || []);
    item.setElapsedUs(phase.elapsedUs);
    return item;
  }));
  return out;
}

function toMultiPatternReport(report) {
  const out = new cascadePb.MultiPatternReport();
  out.setPatternsList(report.patterns.map(toCascadeReport));
  out.setTotalPass(report.totalPass);
  out.setTotalFail(report.totalFail);
  out.setTotalElapsedUs(report.totalElapsedUs);
  return out;
}

function findBinary(memberID, slug) {
  try {
    return composite.member(memberID);
  } catch (_) {
    // Fall through to installed or op-resolved binaries for direct source runs.
  }
  const opBin = process.env.OPBIN || path.join(os.homedir(), '.op', 'bin');
  const installed = findExecutable(path.join(opBin, `${slug}.holon`, 'bin'), slug);
  if (installed) return installed;
  try {
    const out = childProcess.execFileSync('op', ['--bin', slug], {
      cwd: ROOT || HERE,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
    if (out) return out;
  } catch (_) {
    // leave unresolved
  }
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
      } catch (_) {
        return '';
      }
    }
  }
  return '';
}

function evidenceLine(tick, result) {
  return `tick=${tick} log=${evidenceText(result.log)} event=${evidenceText(result.event)} hops=${evidenceText(result.hops)}`;
}

function evidenceText(out) {
  return out && out.pass ? 'ok' : compactEvidence(out && out.evidence ? out.evidence : '<empty>');
}

function compactEvidence(value) {
  const out = String(value || '').replace(/\s+/g, ' ').trim();
  return out.length <= 240 ? out : `${out.slice(0, 240)}...`;
}

function unary(method, request, timeoutMs) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('timeout')), timeoutMs);
    method(request, (err, out) => {
      clearTimeout(timer);
      if (err) reject(err);
      else resolve(out || {});
    });
  });
}

function normalizeListenUri(listenUri) {
  const match = String(listenUri || '').match(/^tcp:\/\/:(\d+)$/);
  if (match) return `tcp://0.0.0.0:${match[1]}`;
  return listenUri;
}

function canonicalCommand(raw) {
  return String(raw || '').trim().toLowerCase().replace(/[-_ ]/g, '');
}

function modeSuffix(name) {
  return name === 'default' ? '' : `--${name} `;
}

function elapsedText(elapsedUs) {
  const ms = Number(elapsedUs || 0) / 1000;
  if (ms < 1000) return `${Math.trunc(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;
  return `${(ms / 60000).toFixed(1)}m`;
}

function nowMicros() {
  return Number(process.hrtime.bigint() / 1000n);
}

function output(emit, value = '') {
  if (emit) console.log(value);
}

main()
  .then((code) => {
    process.exitCode = code;
  })
  .catch((error) => {
    process.stderr.write(`FAIL: ${error.stack || error.message}\n`);
    process.exitCode = 1;
  });
