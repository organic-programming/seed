'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const grpc = require('@grpc/grpc-js');

const grpcclient = require('./grpcclient');
const observability = require('./observability');
const observabilityWire = require('./gen/holons/v1/observability');
const describeWire = require('./gen/holons/v1/describe');

const TransportCoverageSequence = Object.freeze([
    'stdio', 'stdio', 'tcp', 'unix', 'tcp', 'tcp',
    'stdio', 'unix', 'unix', 'stdio',
]);

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

class SpawnedMember {
    constructor({ slug, uid, listenURI, target, process: child, session, relay }) {
        this.slug = slug;
        this.uid = uid;
        this.listenURI = listenURI;
        this.target = target;
        this.clientTarget = target;
        this.process = child || null;
        this.session = session || null;
        this.relay = relay || null;
        this.stopped = false;
    }

    clientFor(ClientCtor, options = {}) {
        return new ClientCtor(this.target, options.credentials || grpc.credentials.createInsecure(), options.channelOptions || {});
    }

    async stop(timeoutMs = 3000) {
        if (this.stopped) return;
        this.stopped = true;
        if (this.relay) this.relay.stop();
        const child = this.process;
        if (this.session && typeof this.session.close === 'function') {
            await this.session.close();
            if (child && child.exitCode === null) {
                await waitForExit(child, timeoutMs).catch(() => {
                    if (child.exitCode === null) child.kill('SIGKILL');
                });
            }
            return;
        }
        if (!child || child.exitCode !== null) return;
        child.kill('SIGTERM');
        await waitForExit(child, timeoutMs).catch(() => {
            if (child.exitCode === null) child.kill('SIGKILL');
        });
    }
}

class Cascade {
    constructor(top) {
        this.Top = top;
        this.top = top;
    }

    async stop(timeoutMs = 3000) {
        if (this.top) await this.top.stop(timeoutMs);
    }
}

async function SpawnMember(options = {}) {
    const slug = String(options.slug || options.Slug || path.basename(options.binaryPath || options.binary || '')).trim();
    const binary = String(options.binaryPath || options.BinaryPath || options.binary || options.Binary || '').trim();
    if (!slug) throw new Error('spawn member: slug is required');
    if (!binary) throw new Error(`spawn member ${slug}: binary path is required`);

    const uid = String(options.instanceUid || options.InstanceUID || crypto.randomUUID()).trim();
    const transport = String(options.transport || options.Transport || 'stdio').trim().toLowerCase();
    const downstream = options.downstreamChain || options.DownstreamChain || [];
    const listenURI = listenURIForSpawn(transport, uid);
    const args = ['serve', '--listen', listenURI, '--transport', transport];
    for (const child of downstream) {
        const childSlug = child.slug || child.Slug;
        const childBinary = child.binary || child.Binary;
        if (!childSlug || !childBinary) throw new Error(`spawn member ${slug}: downstream child requires slug and binary`);
        args.push('--child', `${childSlug}=${childBinary}`);
    }
    const env = spawnEnvironment(uid, options.extraEnv || options.ExtraEnv || {});
    let member;

    if (transport === 'stdio') {
        const session = await grpcclient.dialStdio(binary, HolonMetaClient, {
            args,
            cwd: path.dirname(binary),
            env,
        });
        await describeReady(session.client, 10000);
        member = new SpawnedMember({
            slug,
            uid,
            listenURI,
            target: session.target,
            process: session.process,
            session,
        });
    } else {
        const child = spawn(binary, args, {
            cwd: path.dirname(binary),
            env,
            stdio: ['ignore', 'ignore', 'inherit'],
        });
        const runRoot = runRootFromEnv(env);
        const meta = await waitSpawnMeta(runRoot, slug, uid, 10000).catch(async (err) => {
            child.kill('SIGTERM');
            throw err;
        });
        const target = normalizeDialTarget(meta.address);
        await dialReady(target, 10000);
        member = new SpawnedMember({
            slug,
            uid,
            listenURI: meta.address || listenURI,
            target,
            process: child,
        });
    }

    const opts = applyDialOptions(options.dialOptions || options.DialOptions || []);
    const transitive = opts.transitiveObservability === undefined ? true : opts.transitiveObservability;
    if (transitive) {
        member.relay = startRelayOn(slug, uid, member.target);
    }
    return member;
}

async function BuildCascade(options = {}) {
    const members = options.members || options.Members || [];
    if (!members.length) throw new Error('build cascade: at least one member is required');
    const top = members[0];
    const spawned = await SpawnMember({
        slug: top.slug || top.Slug,
        binaryPath: top.binary || top.Binary,
        transport: options.transport || options.Transport || 'stdio',
        downstreamChain: members.slice(1),
        extraEnv: options.extraEnv || options.ExtraEnv || {},
        dialOptions: options.dialOptions || options.DialOptions || [],
    });
    return new Cascade(spawned);
}

async function Dial(address, ...rawOptions) {
    const target = normalizeDialAddress(address);
    const client = new HolonMetaClient(target, grpc.credentials.createInsecure());
    const desc = await describeReady(client, 10000);
    client.close();
    const peer = {
        target,
        clientTarget: target,
        describe: desc,
        clientFor(ClientCtor, options = {}) {
            return new ClientCtor(target, options.credentials || grpc.credentials.createInsecure(), options.channelOptions || {});
        },
        async close() {},
    };
    const opts = applyDialOptions(rawOptions);
    if (opts.transitiveObservability) {
        const identity = await resolveRelayIdentity(target, desc);
        peer.relay = startRelayOn(identity.slug, identity.uid, target);
        peer.close = async () => peer.relay.stop();
    }
    return peer;
}

function WithTransitiveObservability(value) {
    return { transitiveObservability: Boolean(value) };
}

function member(id) {
    const executable = (process.env.OP_HOLON_EXECUTABLE || '').trim() || process.argv[1] || process.execPath;
    return memberFromExecutable(executable, id);
}

function memberFromExecutable(executable, id) {
    if (!String(id || '').trim()) {
        throw new Error('member id is required');
    }
    const memberDir = path.join(path.dirname(path.resolve(String(executable))), 'holons', String(id));
    const entries = fs.readdirSync(memberDir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
        if (entry.isDirectory()) continue;
        const candidate = path.join(memberDir, entry.name);
        if (entry.name.endsWith('.exe') || isExecutable(candidate)) {
            return candidate;
        }
    }
    throw new Error(`no executable found in ${memberDir}`);
}

function CheckRelayedLog(options = {}) {
    return pollCheck(options, async () => matchRelayedLog(await readLogs(options), options));
}

function CheckRelayedEvent(options = {}) {
    return pollCheck(options, async () => matchRelayedEvent(await readEvents(options), options));
}

async function pollCheck(options, fn) {
    const timeoutMs = options.timeoutMs || options.TimeoutMs || durationToMs(options.timeout) || 3000;
    const intervalMs = options.pollIntervalMs || options.PollIntervalMs || durationToMs(options.pollInterval) || 100;
    const deadline = Date.now() + timeoutMs;
    let last = { pass: false, evidence: '' };
    for (;;) {
        last = await fn();
        if (last.pass || Date.now() >= deadline) return last;
        await sleep(intervalMs);
    }
}

async function readLogs(options) {
    const target = targetFromOptions(options);
    if (!target) {
        const ring = observability.current().logRing;
        if (!ring) throw new Error('logs family is not enabled');
        return ring.drain();
    }
    const client = new ObservabilityClient(target, grpc.credentials.createInsecure());
    try {
        return await collectStream(client.Logs.bind(client), { min_severity_number: observability.Level.INFO, follow: false }, 2000);
    } finally {
        client.close();
    }
}

async function readEvents(options) {
    const target = targetFromOptions(options);
    if (!target) {
        const bus = observability.current().eventBus;
        if (!bus) throw new Error('events family is not enabled');
        return bus.drain();
    }
    const client = new ObservabilityClient(target, grpc.credentials.createInsecure());
    try {
        return await collectStream(client.Events.bind(client), { follow: false }, 2000);
    } finally {
        client.close();
    }
}

function matchRelayedLog(entries, options) {
    const sender = options.sender || options.Sender || '';
    const leafUID = options.leafUID || options.LeafUID || '';
    const expected = options.expectedChain || options.ExpectedChain || [];
    for (const entry of entries) {
        if (observability.bodyString(entry) !== 'tick received') continue;
        if (observability.stringAttribute(entry, 'sender') !== sender || observability.stringAttribute(entry, 'responder_uid') !== leafUID) continue;
        const evidence = compareChain(entry.chain || [], expected);
        if (evidence) return { pass: false, evidence: compactEvidence(`matching log bad chain: ${evidence}`) };
        return { pass: true, evidence: JSON.stringify(entry) };
    }
    return { pass: false, evidence: compactEvidence(`no relayed tick log sender=${sender} leaf_uid=${leafUID} entries=${entries.length}`) };
}

function matchRelayedEvent(events, options) {
    const eventName = normalizeEventName(options.eventName || options.EventName || observability.EventName.INSTANCE_READY);
    const leafUID = options.leafUID || options.LeafUID || '';
    const expected = options.expectedChain || options.ExpectedChain || [];
    for (const event of events) {
        if ((event.event_name || '') !== eventName || observability.stringAttribute(event, observability.Attr.HOLONS_INSTANCE_UID) !== leafUID) continue;
        const evidence = compareChain(event.chain || [], expected);
        if (evidence) return { pass: false, evidence: compactEvidence(`matching event bad chain: ${evidence}`) };
        return { pass: true, evidence: JSON.stringify(event) };
    }
    return { pass: false, evidence: compactEvidence(`no relayed ${eventName} event leaf_uid=${leafUID} events=${events.length}`) };
}

function listenURIForSpawn(transport, uid) {
    if (transport === 'stdio') return 'stdio://';
    if (transport === 'tcp') return 'tcp://127.0.0.1:0';
    if (transport === 'unix') return `unix://${path.join(os.tmpdir(), `op-${cleanSocketToken(uid)}.sock`)}`;
    throw new Error(`unsupported transport ${transport}`);
}

function spawnEnvironment(uid, extra = {}) {
    const env = { ...process.env };
    env.OP_INSTANCE_UID = uid;
    env.OP_RUN_DIR = runRootFromEnv(env);
    env.HOLONS_PARENT_PID = String(process.pid);
    const families = activeObservabilityFamilies();
    if (families) env.OP_OBS = families;
    for (const [key, value] of Object.entries(extra || {})) env[key] = String(value);
    return env;
}

function activeObservabilityFamilies() {
    const obs = observability.current();
    const families = [observability.Family.LOGS, observability.Family.METRICS, observability.Family.EVENTS, observability.Family.PROM];
    return families.filter((family) => obs.enabled(family)).join(',');
}

function runRootFromEnv(env = process.env) {
    if (env.OP_RUN_DIR) return env.OP_RUN_DIR;
    if (env.OPPATH) return path.join(env.OPPATH, 'run');
    if (env.HOME) return path.join(env.HOME, '.op', 'run');
    return path.join(os.tmpdir(), '.op', 'run');
}

async function waitSpawnMeta(runRoot, slug, uid, timeoutMs) {
    const metaPath = path.join(runRoot, slug, uid, 'meta.json');
    const deadline = Date.now() + timeoutMs;
    let lastError = null;
    while (Date.now() < deadline) {
        try {
            const meta = JSON.parse(fs.readFileSync(metaPath, 'utf8'));
            if (meta.uid === uid && meta.address) return meta;
        } catch (err) {
            lastError = err;
        }
        await sleep(50);
    }
    throw new Error(`meta not ready for ${slug}/${uid}: ${lastError ? lastError.message : 'timeout'}`);
}

async function dialReady(target, timeoutMs) {
    const deadline = Date.now() + timeoutMs;
    let lastError = null;
    while (Date.now() < deadline) {
        const client = new HolonMetaClient(target, grpc.credentials.createInsecure());
        try {
            await describeReady(client, 500);
            client.close();
            return;
        } catch (err) {
            lastError = err;
            client.close();
            await sleep(50);
        }
    }
    throw new Error(`dial ${target}: ${lastError ? lastError.message : 'timeout'}`);
}

function describeReady(client, timeoutMs) {
    return unary(client.Describe.bind(client), {}, timeoutMs);
}

async function resolveRelayIdentity(target, desc) {
    const client = new ObservabilityClient(target, grpc.credentials.createInsecure());
    try {
        const events = await collectStream(client.Events.bind(client), { follow: false }, 1000).catch(() => []);
        for (const event of events) {
            const uid = observability.stringAttribute(event, observability.Attr.HOLONS_INSTANCE_UID);
            if ((event.chain || []).length === 0 && uid) {
                return { slug: observability.stringAttribute(event, observability.Attr.HOLONS_SLUG) || slugFromDescribe(desc), uid };
            }
        }
        const logs = await collectStream(client.Logs.bind(client), { follow: false }, 1000).catch(() => []);
        for (const entry of logs) {
            const uid = observability.stringAttribute(entry, observability.Attr.HOLONS_INSTANCE_UID);
            if ((entry.chain || []).length === 0 && uid) {
                return { slug: observability.stringAttribute(entry, observability.Attr.HOLONS_SLUG) || slugFromDescribe(desc), uid };
            }
        }
        throw new Error('peer did not expose a local log or event with holons.slug and holons.instance_uid');
    } finally {
        client.close();
    }
}

function startRelayOn(slug, uid, target) {
    const client = new ObservabilityClient(target, grpc.credentials.createInsecure());
    const relay = new observability.MemberRelay({
        childSlug: slug,
        childUid: uid,
        client,
        observability: observability.current(),
    });
    relay.start();
    const stop = relay.stop.bind(relay);
    relay.stop = () => {
        stop();
        client.close();
    };
    return relay;
}

function normalizeDialAddress(address) {
    const trimmed = String(address || '').trim();
    if (!trimmed) throw new Error('dial address is required');
    if (trimmed.startsWith('stdio://')) throw new Error('composite.Dial does not support stdio addresses; use SpawnMember');
    if (!trimmed.includes('://')) {
        if (!/^[^:]+:\d+$/.test(trimmed)) throw new Error(`dial address must be tcp://host:port, unix:///path, or host:port: ${trimmed}`);
        return trimmed;
    }
    if (trimmed.startsWith('tcp://') || trimmed.startsWith('unix://')) return normalizeDialTarget(trimmed);
    throw new Error(`unsupported dial address ${trimmed}`);
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

function applyDialOptions(rawOptions) {
    const opts = {};
    for (const option of rawOptions || []) {
        if (!option) continue;
        if (Object.prototype.hasOwnProperty.call(option, 'transitiveObservability')) {
            opts.transitiveObservability = Boolean(option.transitiveObservability);
        }
    }
    return opts;
}

function targetFromOptions(options) {
    const raw = options.conn || options.Conn || options.connection || options.member || options.Member || null;
    if (!raw) return options.target || options.Target || '';
    return raw.target || raw.clientTarget || raw.address || raw;
}

function compareChain(got, want) {
    if (got.length !== want.length) return `chain length ${got.length} want ${want.length}`;
    for (let i = 0; i < want.length; i += 1) {
        const g = String(got[i] || '');
        const w = want[i];
        const wantSlug = typeof w === 'string' ? w : (w.slug || w.Slug || '');
        if (g !== wantSlug) {
            return `hop ${i}=${g} want ${wantSlug}`;
        }
    }
    return '';
}

function normalizeEventName(value) {
    if (typeof value === 'string' && Object.values(observability.EventName).includes(value)) {
        return value;
    }
    if (typeof value === 'string' && Object.prototype.hasOwnProperty.call(observability.EventName, value)) {
        return observability.EventName[value];
    }
    return observability.EventName.INSTANCE_READY;
}

function slugFromDescribe(desc) {
    const identity = desc && desc.manifest && desc.manifest.identity ? desc.manifest.identity : {};
    const alias = (identity.aliases || []).find((value) => String(value || '').trim());
    if (alias) return String(alias).trim();
    return slugify(`${identity.given_name || ''}-${identity.family_name || ''}`) || slugify(identity.family_name || '');
}

function slugify(value) {
    return String(value || '').trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
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
        stream.on('error', (err) => {
            clearTimeout(timer);
            reject(err);
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
        method(request, (err, out) => {
            clearTimeout(timer);
            if (err) reject(err);
            else resolve(out || {});
        });
    });
}

function durationToMs(value) {
    if (!value) return 0;
    if (typeof value === 'number') return value;
    if (typeof value === 'object' && value.seconds !== undefined) return Number(value.seconds) * 1000 + Number(value.nanos || 0) / 1e6;
    return 0;
}

function waitForExit(child, timeoutMs) {
    if (!child || child.exitCode !== null) return Promise.resolve({ code: child ? child.exitCode : 0, signal: null });
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error('timeout')), timeoutMs);
        child.once('exit', (code, signal) => {
            clearTimeout(timer);
            resolve({ code, signal });
        });
    });
}

function isExecutable(candidate) {
    try {
        fs.accessSync(candidate, fs.constants.X_OK);
        return true;
    } catch (_) {
        return false;
    }
}

function cleanSocketToken(value) {
    return String(value || '').slice(0, 24).replace(/[/:\\ ]+/g, '-');
}

function compactEvidence(value) {
    const out = String(value || '').replace(/\s+/g, ' ').trim();
    return out.length <= 240 ? out : `${out.slice(0, 240)}...`;
}

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

module.exports = {
    member,
    memberFromExecutable,
    SpawnMember,
    BuildCascade,
    Dial,
    WithTransitiveObservability,
    CheckRelayedLog,
    CheckRelayedEvent,
    TransportCoverageSequence,
    normalizeDialTarget,
};
