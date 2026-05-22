// Standard gRPC server runner for Node.js holons.

'use strict';

const net = require('node:net');
const fs = require('node:fs');
const path = require('node:path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');

const describe = require('./describe');
const observability = require('./observability');
const observabilityWire = require('./gen/holons/v1/observability');
const transport = require('./transport');

const DEFAULT_URI = transport.DEFAULT_URI;
const MAX_GRPC_MESSAGE_BYTES = 1 * 1024 * 1024;

let currentTransport = '';

function CurrentTransport() {
    return currentTransport;
}

function parseFlags(args) {
    return parseOptions(args).listenUri;
}

function parseOptions(args) {
    let listenUri = DEFAULT_URI;
    let reflect = false;

    for (let i = 0; i < args.length; i += 1) {
        if (args[i] === '--listen' && i + 1 < args.length) listenUri = args[i + 1];
        if (args[i] === '--port' && i + 1 < args.length) listenUri = `tcp://:${args[i + 1]}`;
        if (args[i] === '--reflect') reflect = true;
    }
    return { listenUri, reflect };
}

function parseChildFlags(args) {
    const children = [];
    const remaining = [];
    for (let i = 0; i < args.length; i += 1) {
        const arg = args[i];
        if (arg === '--child') {
            i += 1;
            if (i >= args.length) throw new Error('--child requires <slug>=<binary>');
            children.push(parseChildSpec(args[i]));
            continue;
        }
        if (arg.startsWith('--child=')) {
            children.push(parseChildSpec(arg.slice('--child='.length)));
            continue;
        }
        remaining.push(arg);
    }
    return { children, remaining };
}

function parseChildSpec(raw) {
    const idx = String(raw).indexOf('=');
    if (idx < 0) throw new Error('--child requires <slug>=<binary>');
    const slug = raw.slice(0, idx).trim();
    const binary = raw.slice(idx + 1).trim();
    if (!slug || !binary) throw new Error('--child requires non-empty slug and binary');
    return { slug, binary };
}

function run(listenUri, registerFn) {
    return runWithOptions(listenUri, registerFn, false);
}

async function runWithOptions(listenUri, registerFn, reflectOrOptions = false) {
    const options = normalizeRunOptions(reflectOrOptions);
    const parsed = transport.parseURI(listenUri || DEFAULT_URI);
    const logger = options.logger || console;
    currentTransport = parsed.scheme;

    try {
        const server = new grpc.Server({
            'grpc.max_receive_message_length': MAX_GRPC_MESSAGE_BYTES,
            'grpc.max_send_message_length': MAX_GRPC_MESSAGE_BYTES,
        });
        observability.checkEnv();
        const obs = (process.env.OP_OBS || '').trim()
            ? configuredObservability(options.slug || '')
            : null;
        registerFn(server);
        try {
            autoRegisterHolonMeta(server);
        } catch (err) {
            logger.error(`HolonMeta registration failed: ${err.message}`);
            throw err;
        }
        if (obs && obs.families && obs.families.size > 0) {
            observability.registerService(server, obs);
        }

        const reflectionEnabled = maybeEnableReflection(server, options);

        let runtime;

        if (parsed.scheme === 'tcp') {
            runtime = await startTCPServer(server, parsed);
        } else if (parsed.scheme === 'unix') {
            runtime = await startUnixServer(server, parsed);
        } else if (parsed.scheme === 'stdio') {
            runtime = await startStdioServer(server);
        } else if (parsed.scheme === 'ws' || parsed.scheme === 'wss') {
            runtime = await startWSServer(server, parsed.uri, options.ws);
        } else {
            throw new Error(`unsupported serve transport: ${parsed.scheme}://`);
        }

        const promServer = await startPromServer(obs, logger);
        const metricsAddr = promServer ? promServer.addrURL() : '';
        if (obs && obs.families && obs.families.size > 0) {
            startObservabilityRuntime(obs, runtime.publicURI, parsed.scheme, metricsAddr);
        }
        const startedRelays = await startMemberRelays(obs, options.memberEndpoints, logger);
        const closeRuntime = runtime.close;
        runtime.close = async () => {
            try {
                for (const relay of startedRelays) relay.stop();
                for (const relay of startedRelays) {
                    if (relay.client && typeof relay.client.close === 'function') relay.client.close();
                }
                if (promServer) await promServer.close();
                await closeRuntime.call(runtime);
            } finally {
                currentTransport = '';
            }
        };
        attachRuntime(server, runtime, options.logger || console);

        const mode = reflectionEnabled ? 'reflection ON' : 'reflection OFF';
        logger.error(`gRPC server listening on ${runtime.publicURI} (${mode})`);

        return server;
    } catch (err) {
        currentTransport = '';
        throw err;
    }
}

function configuredObservability(slug) {
    const existing = observability.current();
    if (existing && existing.families && existing.families.size > 0) {
        if (!slug || !existing.cfg.slug || existing.cfg.slug === slug) {
            return existing;
        }
    }
    return observability.fromEnv({ slug });
}

function autoRegisterHolonMeta(server) {
    describe.register(server);
}

function startObservabilityRuntime(obs, publicURI, transportName, metricsAddr = '') {
    if (!obs.cfg.runDir) return;
    observability.enableDiskWriters(obs.cfg.runDir);
    if (obs.enabled(observability.Family.EVENTS)) {
        obs.emit(observability.EventName.INSTANCE_READY, {
            listener: publicURI,
            metrics_addr: metricsAddr || '',
        });
    }
    observability.writeMetaJson(obs.cfg.runDir, {
        slug: obs.cfg.slug || '',
        uid: obs.cfg.instanceUid || '',
        pid: process.pid,
        started_at: new Date().toISOString(),
        mode: 'persistent',
        transport: transportName || '',
        address: publicURI || '',
        ...(metricsAddr ? { metrics_addr: metricsAddr } : {}),
        ...(obs.enabled(observability.Family.LOGS)
            ? { log_path: path.join(obs.cfg.runDir, 'stdout.log') }
            : {}),
        ...(obs.cfg.organismUid ? { organism_uid: obs.cfg.organismUid } : {}),
        ...(obs.cfg.organismSlug ? { organism_slug: obs.cfg.organismSlug } : {}),
    });
}

function pathJoin(...parts) {
    return path.join(...parts);
}

function normalizeRunOptions(input) {
    if (typeof input === 'boolean') {
        return {
            reflect: input,
            reflectionPackageDefinition: null,
            ws: {},
            logger: console,
        };
    }

    const options = input || {};

    return {
        reflect: options.reflect !== undefined ? Boolean(options.reflect) : false,
        reflectionPackageDefinition: options.reflectionPackageDefinition || null,
        ws: options.ws || {},
        logger: options.logger || console,
        memberEndpoints: Array.isArray(options.memberEndpoints) ? options.memberEndpoints : [],
        slug: options.slug || '',
    };
}

async function startPromServer(obs, logger) {
    if (!obs || !obs.enabled(observability.Family.PROM)) return null;
    const server = new observability.PromServer(obs.cfg.promAddr || ':0');
    try {
        await server.start();
        return server;
    } catch (err) {
        logger.warn(`warning: prom HTTP bind failed: ${err.message}`);
        return null;
    }
}

async function startMemberRelays(obs, members, logger) {
    if (!obs || (!obs.enabled(observability.Family.LOGS) && !obs.enabled(observability.Family.EVENTS))) {
        return [];
    }
    const started = [];
    for (const raw of members || []) {
        let member = normalizeMemberRef(raw);
        if (!member.slug || !member.address) {
            logger.warn(`warning: observability relay skipped incomplete member ref: slug="${member.slug}" uid="${member.uid}" address="${member.address}"`);
            continue;
        }
        const client = new ObservabilityClient(
            normalizeRelayDialTarget(member.address),
            grpc.credentials.createInsecure(),
        );
        try {
            member = await resolveRelayMemberIdentity(client, member);
            if (!member.uid) {
                logger.warn(`warning: observability relay uid unresolved for ${member.slug} at ${member.address}; chain hops will have empty uid`);
            }
            const relay = new observability.MemberRelay({
                childSlug: member.slug,
                childUid: member.uid,
                client,
                observability: obs,
            });
            relay.start();
            started.push(relay);
        } catch (err) {
            if (typeof client.close === 'function') client.close();
            logger.warn(`warning: observability relay start ${member.slug}/${member.uid}: ${err.message}`);
        }
    }
    return started;
}

function normalizeMemberRef(raw) {
    return {
        slug: String(raw && raw.slug ? raw.slug : '').trim(),
        uid: String(raw && raw.uid ? raw.uid : '').trim(),
        address: String(raw && raw.address ? raw.address : '').trim(),
    };
}

async function resolveRelayMemberIdentity(client, member) {
    if (member.uid) return member;
    try {
        const events = await collectStream(client.Events.bind(client), { event_names: [observability.EventName.INSTANCE_READY], follow: false }, 2000);
        for (const event of events) {
            const uid = observability.stringAttribute(event, observability.Attr.HOLONS_INSTANCE_UID);
            if (uid && !(event.chain || []).length) {
                return {
                    ...member,
                    slug: observability.stringAttribute(event, observability.Attr.HOLONS_SLUG) || member.slug,
                    uid,
                };
            }
        }
    } catch {
        // fall back to Metrics
    }
    try {
        const metrics = await collectStream(client.Metrics.bind(client), {}, 2000);
        for (const metric of metrics) {
            const attrs = metric.sum?.data_points?.[0]?.attributes
                || metric.gauge?.data_points?.[0]?.attributes
                || metric.histogram?.data_points?.[0]?.attributes
                || [];
            const uid = observability.stringAttribute(attrs, observability.Attr.HOLONS_INSTANCE_UID);
            if (uid) {
                return {
                    ...member,
                    slug: observability.stringAttribute(attrs, observability.Attr.HOLONS_SLUG) || member.slug,
                    uid,
                };
            }
        }
    } catch {
        // leave unresolved
    }
    return member;
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

function normalizeRelayDialTarget(address) {
    const trimmed = String(address || '').trim();
    if (!trimmed.includes('://')) return trimmed;
    const parsed = transport.parseURI(trimmed);
    if (parsed.scheme === 'tcp') {
        const host = !parsed.host || parsed.host === '0.0.0.0' || parsed.host === '::'
            ? '127.0.0.1'
            : parsed.host;
        return `${host}:${parsed.port}`;
    }
    if (parsed.scheme === 'unix') return `unix://${parsed.path}`;
    return trimmed;
}

const ObservabilityClient = grpc.makeGenericClientConstructor(
    observabilityWire.HOLON_OBSERVABILITY_SERVICE_DEF,
    'HolonObservability',
    {},
);

function maybeEnableReflection(server, options) {
    if (!options.reflect) return false;

    let reflectionPackageDefinition = options.reflectionPackageDefinition;
    if (!reflectionPackageDefinition) {
        reflectionPackageDefinition = loadReflectionPackageDefinition(pathJoin(process.cwd(), 'protos'));
    }
    if (!reflectionPackageDefinition) return false;

    try {
        const { ReflectionService } = require('@grpc/reflection');
        const reflection = new ReflectionService(reflectionPackageDefinition);
        reflection.addToServer(server);
        return true;
    } catch (err) {
        (options.logger || console).warn(`reflection could not be enabled: ${err.message}`);
        return false;
    }
}

function loadReflectionPackageDefinition(protoDir) {
    const absDir = path.resolve(String(protoDir));
    if (!fs.existsSync(absDir) || !fs.statSync(absDir).isDirectory()) {
        return null;
    }

    const relFiles = collectProtoFiles(absDir);
    if (relFiles.length === 0) {
        return null;
    }

    const absFiles = relFiles.map((file) => path.resolve(absDir, file));
    const packageDefinition = protoLoader.loadSync(absFiles, {
        includeDirs: [absDir],
        keepCase: true,
        longs: String,
        enums: String,
        defaults: true,
        oneofs: true,
    });
    return grpc.loadPackageDefinition(packageDefinition);
}

function collectProtoFiles(rootDir) {
    const files = [];

    walk(rootDir);
    files.sort();
    return files;

    function walk(currentDir) {
        const entries = fs.readdirSync(currentDir, { withFileTypes: true });
        entries.sort((a, b) => a.name.localeCompare(b.name));

        for (const entry of entries) {
            const currentPath = path.join(currentDir, entry.name);
            if (entry.isDirectory()) {
                if (entry.name.startsWith('.')) {
                    continue;
                }
                walk(currentPath);
                continue;
            }
            if (path.extname(entry.name) !== '.proto') {
                continue;
            }
            files.push(path.relative(rootDir, currentPath));
        }
    }
}

async function startTCPServer(server, parsed) {
    const host = parsed.host || '0.0.0.0';
    const target = `${host}:${parsed.port}`;
    const boundPort = await bindAndStart(server, target);

    return {
        publicURI: `tcp://${normalizePublicHost(host)}:${boundPort}`,
        async close() {
            await tryShutdown(server);
        },
    };
}

async function startUnixServer(server, parsed) {
    try {
        fs.unlinkSync(parsed.path);
    } catch {
        // ignore stale file cleanup errors
    }

    await bindAndStart(server, `unix:${parsed.path}`);

    return {
        publicURI: `unix://${parsed.path}`,
        async close() {
            await tryShutdown(server);
        },
    };
}

async function startStdioServer(server) {
    const internalTarget = await bindInternalTCP(server);
    const bridge = new StdioBridge(internalTarget);
    bridge.start();

    return {
        publicURI: 'stdio://',
        async close() {
            bridge.close();
            await tryShutdown(server);
        },
    };
}

async function startWSServer(server, wsURI, wsOptions) {
    const internalTarget = await bindInternalTCP(server);
    const bridge = new WSBridge(wsURI, internalTarget, wsOptions);
    await bridge.start();

    return {
        publicURI: bridge.address,
        async close() {
            bridge.close();
            await tryShutdown(server);
        },
    };
}

function attachRuntime(server, runtime, logger) {
    server.__holonsRuntime = runtime;
    server.stopHolon = async () => {
        if (!server.__holonsRuntime) return;
        const rt = server.__holonsRuntime;
        server.__holonsRuntime = null;
        await rt.close();
    };

    const shutdown = async () => {
        try {
            await server.stopHolon();
        } catch (err) {
            logger.error(`gRPC shutdown error: ${err.message}`);
        }
    };

    const sigTerm = () => { shutdown(); };
    const sigInt = () => { shutdown(); };

    process.on('SIGTERM', sigTerm);
    process.on('SIGINT', sigInt);

    server.__holonsDetachSignals = () => {
        process.off('SIGTERM', sigTerm);
        process.off('SIGINT', sigInt);
    };
}

function bindAndStart(server, target) {
    return new Promise((resolve, reject) => {
        server.bindAsync(target, grpc.ServerCredentials.createInsecure(), (err, port) => {
            if (err) {
                reject(err);
                return;
            }
            maybeStartServer(server);
            resolve(port);
        });
    });
}

async function bindInternalTCP(server) {
    const port = await bindAndStart(server, '127.0.0.1:0');
    return `127.0.0.1:${port}`;
}

function maybeStartServer(server) {
    if (typeof server.start !== 'function') {
        return;
    }
    try {
        server.start();
    } catch (err) {
        const msg = String(err && err.message ? err.message : err);
        if (!/already started/i.test(msg) && !/deprecated/i.test(msg)) {
            throw err;
        }
    }
}

function tryShutdown(server) {
    return new Promise((resolve) => {
        if (typeof server.tryShutdown !== 'function') {
            resolve();
            return;
        }

        if (typeof server.__holonsDetachSignals === 'function') {
            server.__holonsDetachSignals();
            delete server.__holonsDetachSignals;
        }

        server.tryShutdown(() => resolve());
    });
}

class StdioBridge {
    constructor(target) {
        this.target = target;
        this.socket = null;
        this.started = false;
    }

    start() {
        if (this.started) return;
        this.started = true;

        this.socket = net.createConnection(parseTCPHostPort(this.target), () => {
            process.stdin.pipe(this.socket);
            this.socket.pipe(process.stdout);
        });

        this.socket.on('error', () => {
            this.close();
        });
    }

    close() {
        if (!this.started) return;
        this.started = false;

        try { process.stdin.unpipe(this.socket); } catch {
            // no-op
        }
        try { this.socket.unpipe(process.stdout); } catch {
            // no-op
        }

        if (this.socket && !this.socket.destroyed) {
            this.socket.destroy();
        }
        this.socket = null;
    }
}

class WSBridge {
    constructor(publicURI, internalTarget, wsOptions = {}) {
        this.listener = new transport.WSListener(publicURI, wsOptions);
        this.internalTarget = internalTarget;
        this.streams = new Set();
        this.sockets = new Set();
        this.address = publicURI;
    }

    async start() {
        this.listener.on('connection', (stream) => {
            const socket = net.createConnection(parseTCPHostPort(this.internalTarget));

            this.streams.add(stream);
            this.sockets.add(socket);

            stream.pipe(socket);
            socket.pipe(stream);

            const cleanup = () => {
                stream.destroy();
                socket.destroy();
                this.streams.delete(stream);
                this.sockets.delete(socket);
            };

            stream.on('error', cleanup);
            socket.on('error', cleanup);
            stream.on('close', cleanup);
            socket.on('close', cleanup);
        });

        await this.listener.ready();
        this.address = this.listener.address;
    }

    close() {
        this.listener.close();
        for (const stream of this.streams) {
            stream.destroy();
        }
        for (const socket of this.sockets) {
            socket.destroy();
        }
        this.streams.clear();
        this.sockets.clear();
    }
}

function parseTCPHostPort(target) {
    const idx = target.lastIndexOf(':');
    if (idx < 0) {
        return { host: '127.0.0.1', port: Number(target) };
    }
    return {
        host: target.slice(0, idx),
        port: Number(target.slice(idx + 1)),
    };
}

function normalizePublicHost(host) {
    if (!host || host === '0.0.0.0') {
        return '0.0.0.0';
    }
    return host;
}

module.exports = {
    CurrentTransport,
    parseFlags,
    parseOptions,
    parseChildFlags,
    run,
    runWithOptions,
    DEFAULT_URI,
};
