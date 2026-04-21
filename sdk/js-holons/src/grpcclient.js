'use strict';

const net = require('node:net');
const { spawn } = require('node:child_process');

const grpc = require('@grpc/grpc-js');
const { WebSocket, createWebSocketStream } = require('ws');

const transport = require('./transport');

function dial(addressOrURI, ClientCtor, options = {}) {
    assertClientCtor(ClientCtor);

    const target = normalizeDialTarget(addressOrURI || transport.DEFAULT_URI);
    return createClient(ClientCtor, target, options);
}

async function dialWebSocket(uri, ClientCtor, options = {}) {
    assertClientCtor(ClientCtor);

    const proxy = new WSDialProxy(uri, options.ws || {});
    await proxy.start();

    const client = createClient(ClientCtor, proxy.target, options);

    return {
        client,
        close: async () => {
            try {
                if (typeof client.close === 'function') {
                    client.close();
                }
            } finally {
                await proxy.close();
            }
        },
    };
}

async function dialStdio(binaryPath, ClientCtor, options = {}) {
    assertClientCtor(ClientCtor);

    const args = Array.isArray(options.args)
        ? options.args
        : ['serve', '--listen', 'stdio://'];

    const child = spawn(binaryPath, args, {
        stdio: ['pipe', 'pipe', 'inherit'],
        cwd: options.cwd,
        env: options.env || process.env,
    });

    const proxy = new StdioDialProxy(child);
    await proxy.start();

    const client = createClient(ClientCtor, proxy.target, options);

    return {
        client,
        process: child,
        close: async () => {
            try {
                if (typeof client.close === 'function') {
                    client.close();
                }
            } finally {
                await proxy.close();
                if (!child.killed) {
                    child.kill();
                }
            }
        },
    };
}

async function dialURI(uri, ClientCtor, options = {}) {
    const s = transport.scheme(uri || '');

    if (!s || s === 'tcp' || s === 'unix') {
        return { client: dial(uri, ClientCtor, options), close: async () => {} };
    }

    if (s === 'ws' || s === 'wss') {
        return dialWebSocket(uri, ClientCtor, options);
    }

    if (s === 'stdio') {
        const command = options.command;
        if (!command) {
            throw new Error('dialURI(stdio://) requires options.command');
        }
        return dialStdio(command, ClientCtor, options);
    }

    throw new Error(`unsupported dial URI: ${uri}`);
}

function createClient(ClientCtor, target, options = {}) {
    const credentials = options.credentials || grpc.credentials.createInsecure();
    const channelOptions = options.channelOptions || {};
    return new ClientCtor(target, credentials, channelOptions);
}

function normalizeDialTarget(addressOrURI) {
    if (!addressOrURI.includes('://')) {
        return addressOrURI;
    }

    const parsed = transport.parseURI(addressOrURI);

    if (parsed.scheme === 'tcp') {
        const host = normalizeDialHost(parsed.host);
        return `${host}:${parsed.port}`;
    }

    if (parsed.scheme === 'unix') {
        return `unix://${parsed.path}`;
    }

    throw new Error(
        `dial() supports tcp:// and unix:// targets. Use dialStdio/dialWebSocket for ${parsed.scheme}://.`
    );
}

function normalizeDialHost(host) {
    if (!host || host === '0.0.0.0') return '127.0.0.1';
    return host;
}

function assertClientCtor(ClientCtor) {
    if (typeof ClientCtor !== 'function') {
        throw new Error('Client constructor is required');
    }
}

class WSDialProxy {
    constructor(uri, wsOptions = {}) {
        this.uri = normalizeWSURI(uri);
        this.wsOptions = wsOptions;
        this.server = null;
        this.target = null;
        this.active = new Set();
    }

    async start() {
        if (this.server) return;

        this.server = net.createServer((socket) => {
            const ws = new WebSocket(this.uri, 'grpc', this.wsOptions);
            const wsStream = createWebSocketStream(ws);

            this.active.add(socket);
            this.active.add(wsStream);

            socket.pipe(wsStream);
            wsStream.pipe(socket);

            const cleanup = () => {
                socket.destroy();
                wsStream.destroy();
                this.active.delete(socket);
                this.active.delete(wsStream);
            };

            socket.on('close', cleanup);
            socket.on('error', cleanup);
            wsStream.on('close', cleanup);
            wsStream.on('error', cleanup);
        });

        await new Promise((resolve, reject) => {
            this.server.once('error', reject);
            this.server.listen({ host: '127.0.0.1', port: 0 }, () => {
                this.server.off('error', reject);
                const addr = this.server.address();
                this.target = `127.0.0.1:${addr.port}`;
                resolve();
            });
        });
    }

    async close() {
        for (const item of this.active) {
            item.destroy();
        }
        this.active.clear();

        if (!this.server) return;

        await new Promise((resolve) => this.server.close(() => resolve()));
        this.server = null;
    }
}

class StdioDialProxy {
    constructor(child) {
        this.child = child;
        this.server = null;
        this.target = null;
        this.socket = null;
        this.pendingStdout = [];
        this.stdoutHandler = null;
        this.stderrHandler = null;
    }

    async start() {
        if (!this.child || !this.child.stdin || !this.child.stdout) {
            throw new Error('stdio process must expose stdin/stdout pipes');
        }

        this.stdoutHandler = (chunk) => {
            if (this.socket && !this.socket.destroyed) {
                this.socket.write(chunk);
            } else {
                this.pendingStdout.push(Buffer.from(chunk));
            }
        };

        this.stderrHandler = () => {};

        this.child.stdout.on('data', this.stdoutHandler);
        this.child.stderr?.on('data', this.stderrHandler);

        this.server = net.createServer((socket) => {
            if (this.socket && !this.socket.destroyed) {
                socket.destroy();
                return;
            }

            this.socket = socket;
            for (const chunk of this.pendingStdout) {
                socket.write(chunk);
            }
            this.pendingStdout = [];

            socket.on('data', (chunk) => {
                this.child.stdin.write(chunk);
            });

            socket.on('close', () => {
                if (this.socket === socket) {
                    this.socket = null;
                }
            });

            socket.on('error', () => {
                socket.destroy();
            });
        });

        await new Promise((resolve, reject) => {
            this.server.once('error', reject);
            this.server.listen({ host: '127.0.0.1', port: 0 }, () => {
                this.server.off('error', reject);
                const addr = this.server.address();
                this.target = `127.0.0.1:${addr.port}`;
                resolve();
            });
        });
    }

    async close() {
        if (this.socket && !this.socket.destroyed) {
            this.socket.destroy();
        }
        this.socket = null;

        if (this.child && this.stdoutHandler) {
            this.child.stdout.off('data', this.stdoutHandler);
        }
        if (this.child && this.stderrHandler && this.child.stderr) {
            this.child.stderr.off('data', this.stderrHandler);
        }

        if (!this.server) return;

        await new Promise((resolve) => this.server.close(() => resolve()));
        this.server = null;
    }
}

function normalizeWSURI(uri) {
    const parsed = transport.parseURI(uri);
    if (parsed.scheme !== 'ws' && parsed.scheme !== 'wss') {
        throw new Error(`expected ws:// or wss:// URI, got ${uri}`);
    }

    const host = parsed.host || '127.0.0.1';
    const scheme = parsed.scheme;
    return `${scheme}://${host}:${parsed.port}${parsed.path || '/grpc'}`;
}

module.exports = {
    dial,
    dialURI,
    dialStdio,
    dialWebSocket,
};
