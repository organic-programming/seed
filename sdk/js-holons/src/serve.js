// Standard gRPC server runner for Node.js holons.

'use strict';

const net = require('node:net');
const fs = require('node:fs');
const path = require('node:path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');

const describe = require('./describe');
const transport = require('./transport');

const DEFAULT_URI = transport.DEFAULT_URI;
const MAX_GRPC_MESSAGE_BYTES = 1 * 1024 * 1024;

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

function run(listenUri, registerFn) {
    return runWithOptions(listenUri, registerFn, false);
}

async function runWithOptions(listenUri, registerFn, reflectOrOptions = false) {
    const options = normalizeRunOptions(reflectOrOptions);
    const parsed = transport.parseURI(listenUri || DEFAULT_URI);
    const logger = options.logger || console;

    const server = new grpc.Server({
        'grpc.max_receive_message_length': MAX_GRPC_MESSAGE_BYTES,
        'grpc.max_send_message_length': MAX_GRPC_MESSAGE_BYTES,
    });
    registerFn(server);
    try {
        autoRegisterHolonMeta(server);
    } catch (err) {
        logger.error(`HolonMeta registration failed: ${err.message}`);
        throw err;
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

    attachRuntime(server, runtime, options.logger || console);

    const mode = reflectionEnabled ? 'reflection ON' : 'reflection OFF';
    logger.error(`gRPC server listening on ${runtime.publicURI} (${mode})`);

    return server;
}

function autoRegisterHolonMeta(server) {
    describe.register(server);
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
    };
}

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
    parseFlags,
    parseOptions,
    run,
    runWithOptions,
    DEFAULT_URI,
};
