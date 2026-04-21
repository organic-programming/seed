// Tests for js-holons SDK — using Node.js built-in test runner

'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const os = require('node:os');
const net = require('node:net');
const { spawn, spawnSync } = require('node:child_process');
const { pathToFileURL } = require('node:url');
const grpc = require('@grpc/grpc-js');
const WS = require('ws');

const {
    transport,
    describe: holonDescribe,
    identity,
    serve,
    grpcclient,
    holonrpc,
} = require('../src/index');
const { useStaticDescribeResponse } = require('./helpers/static_describe');

class FakeClient {
    constructor(target, credentials, options) {
        this.target = target;
        this.credentials = credentials;
        this.options = options;
        this.closed = false;
    }

    close() {
        this.closed = true;
    }
}

async function loadJSWebHolonClient() {
    const jsWebPath = path.join(__dirname, '..', '..', 'js-web-holons', 'src', 'index.mjs');
    const mod = await import(pathToFileURL(jsWebPath).href);
    return mod.HolonClient;
}

function isSocketPermissionError(err) {
    if (!err) return false;
    if (err.code === 'EPERM' || err.code === 'EACCES') return true;
    return /listen\s+(eperm|eacces)/i.test(String(err.message || err));
}

async function startHolonRPCServerOrSkip(t, server) {
    try {
        await server.start();
    } catch (err) {
        if (isSocketPermissionError(err)) {
            t.skip(`socket bind not permitted in this environment: ${err.message}`);
            return false;
        }
        throw err;
    }
    return true;
}

function canListenOnLoopback() {
    return new Promise((resolve) => {
        const probe = net.createServer();
        probe.once('error', (err) => {
            resolve(!isSocketPermissionError(err));
        });
        probe.listen(0, '127.0.0.1', () => {
            probe.close(() => resolve(true));
        });
    });
}

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

function readFDCount() {
    try {
        return fs.readdirSync('/dev/fd').length;
    } catch {
        return process.getActiveResourcesInfo().length;
    }
}

async function sampleFDCount(samples = 3, intervalMs = 25) {
    let min = Number.POSITIVE_INFINITY;
    for (let i = 0; i < samples; i += 1) {
        min = Math.min(min, readFDCount());
        if (i + 1 < samples) {
            await sleep(intervalMs);
        }
    }
    return Number.isFinite(min) ? min : readFDCount();
}

function reserveLoopbackPort() {
    return new Promise((resolve, reject) => {
        const probe = net.createServer();
        probe.once('error', reject);
        probe.listen(0, '127.0.0.1', () => {
            const addr = probe.address();
            if (!addr || typeof addr === 'string') {
                probe.close(() => reject(new Error('failed to reserve loopback port')));
                return;
            }

            probe.close((err) => {
                if (err) {
                    reject(err);
                    return;
                }
                resolve(addr.port);
            });
        });
    });
}

async function waitForWSReady(uri, protocol = 'grpc', timeoutMs = 5000) {
    const deadline = Date.now() + timeoutMs;
    let lastErr = null;

    while (Date.now() < deadline) {
        try {
            await new Promise((resolve, reject) => {
                const ws = new WS(uri, protocol);

                const timer = setTimeout(() => {
                    ws.terminate();
                    reject(new Error(`websocket probe timed out for ${uri}`));
                }, 300);
                timer.unref?.();

                ws.once('open', () => {
                    clearTimeout(timer);
                    ws.terminate();
                    resolve();
                });

                ws.once('error', (err) => {
                    clearTimeout(timer);
                    reject(err);
                });
            });

            return;
        } catch (err) {
            lastErr = err;
            await sleep(50);
        }
    }

    throw lastErr || new Error(`websocket endpoint not ready: ${uri}`);
}

async function terminateChildProcess(child, timeoutMs = 1000) {
    if (!child || child.exitCode !== null) return;
    sendChildSignal(child, 'SIGTERM');

    await new Promise((resolve) => {
        const onExit = () => {
            clearTimeout(timer);
            resolve();
        };

        const timer = setTimeout(() => {
            child.off('exit', onExit);
            if (child.exitCode === null) {
                sendChildSignal(child, 'SIGKILL');
                child.once('exit', () => resolve());
                return;
            }
            resolve();
        }, timeoutMs);
        timer.unref?.();

        child.once('exit', onExit);
    });
}

function sendChildSignal(child, signal) {
    if (!child || child.exitCode !== null) {
        return;
    }

    if (process.platform !== 'win32' && child.detached && Number.isInteger(child.pid) && child.pid > 0) {
        try {
            process.kill(-child.pid, signal);
            return;
        } catch (err) {
            if (err && err.code === 'ESRCH') {
                return;
            }
            throw err;
        }
    }

    child.kill(signal);
}

function isRoutingNotification(params) {
    if (!params || typeof params !== 'object' || Array.isArray(params)) {
        return false;
    }
    return Object.prototype.hasOwnProperty.call(params, 'peer')
        && (Object.prototype.hasOwnProperty.call(params, 'result')
            || Object.prototype.hasOwnProperty.call(params, 'error'));
}

function cloneMap(input) {
    const out = {};
    for (const [key, value] of Object.entries(input || {})) {
        out[key] = value;
    }
    return out;
}

function assertRoutingHintsStripped(params) {
    assert.equal(Object.prototype.hasOwnProperty.call(params, '_routing'), false);
    assert.equal(Object.prototype.hasOwnProperty.call(params, '_peer'), false);
}

async function waitForCount(getCount, want, label, timeoutMs = 2000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
        if (getCount() === want) {
            return;
        }
        await sleep(10);
    }
    assert.equal(getCount(), want, `${label} count mismatch`);
}

async function connectRoutingPeer(t, server, uri, label, onRequest = null) {
    const peer = {
        label,
        id: '',
        client: new holonrpc.HolonRPCClient(),
        requestCount: 0,
        notificationCount: 0,
        requestParams: [],
        notificationParams: [],
    };

    const handlePing = (params) => {
        const cloned = cloneMap(params);
        if (isRoutingNotification(cloned)) {
            peer.notificationCount += 1;
            peer.notificationParams.push(cloned);
            return {};
        }

        peer.requestCount += 1;
        peer.requestParams.push(cloned);

        if (typeof onRequest === 'function') {
            return onRequest(cloned);
        }
        return {
            from: label,
            message: cloned.message,
        };
    };

    peer.client.register('echo.v1.Echo/Ping', handlePing);
    peer.client.register('Echo/Ping', handlePing);

    const connected = new Promise((resolve) => {
        server.once('connection', resolve);
    });

    await peer.client.connect(uri, { timeout: 2000 });
    const connection = await connected;
    peer.id = connection.id;

    return peer;
}

async function buildGoEchoServerBinary(t, goBinary, goHolonsDir) {
    const root = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'go-holons-echo-'));
    const binaryPath = path.join(root, process.platform === 'win32' ? 'echo-server.exe' : 'echo-server');
    const build = spawnSync(goBinary, ['build', '-o', binaryPath, './cmd/echo-server'], {
        cwd: goHolonsDir,
        encoding: 'utf8',
    });

    t.after(async () => {
        await fs.promises.rm(root, { recursive: true, force: true });
    });

    if (build.error || build.status !== 0) {
        const firstErrorLine = String(build.stderr || build.error?.message || '')
            .split(/\r?\n/)
            .find(Boolean) || 'go-holons build failed';
        t.skip(firstErrorLine);
        return null;
    }

    return binaryPath;
}

async function setupRoutingScenario(t) {
    if (!await canListenOnLoopback()) {
        t.skip('socket bind not permitted in this environment');
        return null;
    }

    const server = new holonrpc.HolonRPCServer('ws://127.0.0.1:0/rpc');
    if (!await startHolonRPCServerOrSkip(t, server)) {
        return null;
    }

    const A = await connectRoutingPeer(t, server, server.address, 'A');
    const B = await connectRoutingPeer(t, server, server.address, 'B');
    const C = await connectRoutingPeer(t, server, server.address, 'C');
    const D = await connectRoutingPeer(t, server, server.address, 'D');

    t.after(async () => {
        for (const peer of [A, B, C, D]) {
            await peer.client.close();
        }
        await server.close();
    });

    return { server, peers: { A, B, C, D } };
}

// --- Transport tests ---

describe('transport', () => {
    it('scheme() extracts transport scheme', () => {
        assert.equal(transport.scheme('tcp://:9090'), 'tcp');
        assert.equal(transport.scheme('unix:///tmp/x.sock'), 'unix');
        assert.equal(transport.scheme('stdio://'), 'stdio');
        assert.equal(transport.scheme('ws://host:8080'), 'ws');
        assert.equal(transport.scheme('wss://host:8443/grpc'), 'wss');
    });

    it('DEFAULT_URI is tcp://:9090', () => {
        assert.equal(transport.DEFAULT_URI, 'tcp://:9090');
    });

    it('parseURI() parses tcp://', () => {
        const parsed = transport.parseURI('tcp://127.0.0.1:9090');
        assert.equal(parsed.scheme, 'tcp');
        assert.equal(parsed.host, '127.0.0.1');
        assert.equal(parsed.port, '9090');
    });

    it('parseURI() parses unix://', () => {
        const parsed = transport.parseURI('unix:///tmp/holon.sock');
        assert.equal(parsed.scheme, 'unix');
        assert.equal(parsed.path, '/tmp/holon.sock');
    });

    it('parseURI() parses stdio://', () => {
        const parsed = transport.parseURI('stdio://');
        assert.equal(parsed.scheme, 'stdio');
    });

    it('parseURI() parses ws:// and default path', () => {
        const parsed = transport.parseURI('ws://127.0.0.1:8080');
        assert.equal(parsed.scheme, 'ws');
        assert.equal(parsed.host, '127.0.0.1');
        assert.equal(parsed.port, '8080');
        assert.equal(parsed.path, '/grpc');
    });

    it('parseURI() parses wss:// explicit path', () => {
        const parsed = transport.parseURI('wss://example.com:8443/holon');
        assert.equal(parsed.scheme, 'wss');
        assert.equal(parsed.path, '/holon');
        assert.equal(parsed.secure, true);
    });

    it('StdioListener is single-use', () => {
        const lis = transport.listen('stdio://');
        assert.ok(lis instanceof transport.StdioListener);
        lis.accept();
        assert.throws(() => lis.accept(), /single-use/);
        lis.close();
    });

    it('listen() throws for unsupported URI', () => {
        assert.throws(() => transport.listen('ftp://host'), /unsupported/);
    });

    it('releases TCP listener resources across repeated connect/disconnect cycles', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const server = transport.listen('tcp://127.0.0.1:0');
        server.on('connection', (socket) => {
            socket.on('error', () => {
                socket.destroy();
            });
            socket.on('end', () => {
                socket.destroy();
            });
        });

        await new Promise((resolve, reject) => {
            server.once('listening', resolve);
            server.once('error', reject);
        });

        const addr = server.address();
        if (!addr || typeof addr === 'string') {
            throw new Error('failed to determine listener address');
        }

        t.after(async () => {
            await new Promise((resolve) => {
                server.close(() => resolve());
            });
        });

        const baseline = await sampleFDCount();

        for (let i = 0; i < 50; i += 1) {
            await new Promise((resolve, reject) => {
                const socket = net.createConnection({ host: '127.0.0.1', port: addr.port });
                socket.on('connect', () => {
                    socket.end();
                });
                socket.on('close', () => {
                    resolve();
                });
                socket.on('error', reject);
            });
        }

        await sleep(250);
        const after = await sampleFDCount();
        const delta = after - baseline;

        assert.ok(
            delta <= 5,
            `fd/resources leaked across 50 cycles: baseline=${baseline}, after=${after}, delta=${delta}`
        );
    });
});

// --- Identity tests ---

describe('identity', () => {
    it('parseHolon() parses holon.proto', () => {
        const tmpFile = path.join(os.tmpdir(), `test_holon_${Date.now()}.proto`);
        fs.writeFileSync(tmpFile,
            'syntax = "proto3";\n' +
            '\n' +
            'package test.v1;\n' +
            '\n' +
            'option (holons.v1.manifest) = {\n' +
            '  identity: {\n' +
            '    uuid: "abc-123"\n' +
            '    given_name: "test-holon"\n' +
            '    family_name: "Test"\n' +
            '    motto: "A test holon."\n' +
            '    clade: "deterministic/pure"\n' +
            '  }\n' +
            '  lang: "javascript"\n' +
            '};\n'
        );

        const id = identity.parseHolon(tmpFile);
        assert.equal(id.uuid, 'abc-123');
        assert.equal(id.given_name, 'test-holon');
        assert.equal(id.lang, 'javascript');

        fs.unlinkSync(tmpFile);
    });

    it('parseHolon() throws when holon.proto is missing the manifest option', () => {
        const tmpFile = path.join(os.tmpdir(), `invalid_holon_${Date.now()}.proto`);
        fs.writeFileSync(tmpFile, 'syntax = "proto3";\n\npackage test.v1;\n');
        assert.throws(() => identity.parseHolon(tmpFile), /missing holons\.v1\.manifest/);
        fs.unlinkSync(tmpFile);
    });

    it('resolve() finds the nearest holon.proto from a proto directory', () => {
        const root = fs.mkdtempSync(path.join(os.tmpdir(), 'js-holons-identity-'));
        const protoDir = path.join(root, 'protos');
        const manifestPath = path.join(root, 'holon.proto');

        fs.mkdirSync(protoDir, { recursive: true });
        fs.writeFileSync(manifestPath,
            'syntax = "proto3";\n' +
            '\n' +
            'package test.v1;\n' +
            '\n' +
            'option (holons.v1.manifest) = {\n' +
            '  identity: {\n' +
            '    uuid: "resolve-123"\n' +
            '    given_name: "Resolve"\n' +
            '    family_name: "Holon"\n' +
            '  }\n' +
            '  lang: "js"\n' +
            '};\n'
        );

        try {
            const resolved = identity.resolve(protoDir);
            assert.equal(resolved.source_path, manifestPath);
            assert.equal(resolved.identity.given_name, 'Resolve');
            assert.equal(identity.slugForIdentity(resolved.identity), 'resolve-holon');
        } finally {
            fs.rmSync(root, { recursive: true, force: true });
        }
    });
});

// --- Describe runtime tests ---

describe('describe runtime', () => {
    it('loads built-in holons descriptors from the SDK checkout', () => {
        assert.ok(holonDescribe.holons.DescribeResponse);
        assert.ok(holonDescribe.holons.HOLON_META_SERVICE_DEF);
    });
});

// --- Serve tests ---

describe('serve', () => {
    it('parseFlags() extracts --listen', () => {
        assert.equal(serve.parseFlags(['--listen', 'tcp://:8080']), 'tcp://:8080');
    });

    it('parseFlags() extracts --port', () => {
        assert.equal(serve.parseFlags(['--port', '3000']), 'tcp://:3000');
    });

    it('parseFlags() defaults', () => {
        assert.equal(serve.parseFlags([]), transport.DEFAULT_URI);
    });

    it('parseOptions() extracts --reflect', () => {
        assert.deepEqual(serve.parseOptions(['--listen', 'tcp://:8080', '--reflect']), {
            listenUri: 'tcp://:8080',
            reflect: true,
        });
    });

    it('runWithOptions() fails loudly when no Incode Description is registered', async () => {
        const logs = [];

        await assert.rejects(
            () => serve.runWithOptions(
                'tcp://127.0.0.1:0',
                () => {},
                {
                    reflect: false,
                    logger: {
                        error(message) {
                            logs.push(String(message));
                        },
                        warn() {},
                    },
                },
            ),
            /no Incode Description registered — run op build/
        );

        assert.ok(
            logs.includes('HolonMeta registration failed: no Incode Description registered — run op build'),
        );
    });

    it('rejects oversized gRPC messages with RESOURCE_EXHAUSTED and stays alive', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        useStaticDescribeResponse(t, holonDescribe);

        const echoServer = require('../cmd/echo-server');
        const EchoClient = grpc.makeGenericClientConstructor(echoServer.ECHO_SERVICE_DEF, 'Echo', {});
        const quietLogger = {
            error() {},
            warn() {},
        };

        const server = await serve.runWithOptions(
            'tcp://127.0.0.1:0',
            (grpcServer) => {
                grpcServer.addService(echoServer.ECHO_SERVICE_DEF, {
                    Ping(call, callback) {
                        const request = call.request || {};
                        callback(null, { message: String(request.message || '') });
                    },
                });
            },
            {
                reflect: false,
                logger: quietLogger,
            },
        );

        const parsed = transport.parseURI(server.__holonsRuntime.publicURI);
        const host = parsed.host === '0.0.0.0' ? '127.0.0.1' : parsed.host;
        const client = new EchoClient(`${host}:${parsed.port}`, grpc.credentials.createInsecure());

        t.after(async () => {
            client.close();
            await server.stopHolon();
        });

        const callPing = (message) => new Promise((resolve, reject) => {
            client.Ping({ message }, (err, out) => {
                if (err) {
                    reject(err);
                    return;
                }
                resolve(out || {});
            });
        });

        const payload2MB = 'x'.repeat(2 * 1024 * 1024);
        await assert.rejects(
            () => callPing(payload2MB),
            (err) => err && Number(err.code) === 8
        );

        const small = await callPing('small');
        assert.equal(small.message, 'small');
    });
});

// --- gRPC client tests ---

describe('grpcclient', () => {
    it('dial() normalizes tcp:// target', () => {
        const client = grpcclient.dial('tcp://:9090', FakeClient);
        assert.equal(client.target, '127.0.0.1:9090');
    });

    it('dial() normalizes unix:// target', () => {
        const client = grpcclient.dial('unix:///tmp/holon.sock', FakeClient);
        assert.equal(client.target, 'unix:///tmp/holon.sock');
    });

    it('dial() rejects unsupported schemes', () => {
        assert.throws(() => grpcclient.dial('ws://127.0.0.1:8080/grpc', FakeClient), /dial\(\) supports/);
    });

    it('dialURI(stdio://) requires command', async () => {
        await assert.rejects(
            () => grpcclient.dialURI('stdio://', FakeClient, {}),
            /requires options\.command/
        );
    });

    it('dialURI(ws://) performs unary Ping round-trip over gRPC tunnel', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        useStaticDescribeResponse(t, holonDescribe);

        const echoServer = require('../cmd/echo-server');
        const EchoClient = grpc.makeGenericClientConstructor(echoServer.ECHO_SERVICE_DEF, 'Echo', {});
        const quietLogger = {
            error() {},
            warn() {},
        };

        let server = null;
        try {
            server = await serve.runWithOptions(
                'ws://127.0.0.1:0/grpc',
                (grpcServer) => {
                    grpcServer.addService(echoServer.ECHO_SERVICE_DEF, {
                        Ping(call, callback) {
                            const request = call.request || {};
                            callback(null, {
                                message: String(request.message || ''),
                                sdk: 'js-holons',
                                version: '0.1.0',
                            });
                        },
                    });
                },
                {
                    reflect: false,
                    logger: quietLogger,
                },
            );
        } catch (err) {
            if (isSocketPermissionError(err)) {
                t.skip(`socket bind not permitted in this environment: ${err.message}`);
                return;
            }
            throw err;
        }

        t.after(async () => {
            await server.stopHolon();
        });

        const wsURI = server.__holonsRuntime?.publicURI;
        assert.match(wsURI, /^ws:\/\/127\.0\.0\.1:\d+\/grpc$/);

        const session = await grpcclient.dialURI(wsURI, EchoClient);
        t.after(async () => {
            await session.close();
        });

        const response = await new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                reject(new Error(`ws ping timeout for ${wsURI}`));
            }, 5000);
            timer.unref?.();

            session.client.Ping({ message: 'ws-roundtrip' }, (err, out) => {
                clearTimeout(timer);
                if (err) {
                    reject(err);
                    return;
                }
                resolve(out || {});
            });
        });

        assert.equal(response.message, 'ws-roundtrip');
        assert.equal(response.sdk, 'js-holons');
    });
});

// --- Wrapper command tests ---

describe('wrapper commands', () => {

    it('holon-rpc-client parseArgs supports method, timeout, and expected errors', () => {
        const rpcClient = require('../cmd/holon-rpc-client');
        const parsed = rpcClient.parseArgs([
            'node',
            'holon-rpc-client.js',
            'ws://127.0.0.1:8080/rpc',
            '--method',
            'rpc.heartbeat',
            '--timeout-ms',
            '4000',
            '--expect-error',
            '-32601,12',
            '--connect-only',
        ]);

        assert.equal(parsed.uri, 'ws://127.0.0.1:8080/rpc');
        assert.equal(parsed.method, 'rpc.heartbeat');
        assert.equal(parsed.timeoutMs, 4000);
        assert.deepEqual(parsed.expectErrorCodes, [-32601, 12]);
        assert.equal(parsed.connectOnly, true);
    });

    it('holon-rpc-client cert runner supports connect, echo, heartbeat, and unknown-method checks', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const server = new holonrpc.HolonRPCServer('ws://127.0.0.1:0/rpc');
        server.register('echo.v1.Echo/Ping', (params) => params);
        if (!await startHolonRPCServerOrSkip(t, server)) return;

        try {
            const rpcClient = require('../cmd/holon-rpc-client');

            const connectOnly = await rpcClient.run([
                'node',
                'holon-rpc-client.js',
                server.address,
                '--connect-only',
            ]);
            assert.equal(connectOnly.status, 'pass');
            assert.equal(connectOnly.check, 'connect');

            const echo = await rpcClient.run([
                'node',
                'holon-rpc-client.js',
                server.address,
                '--method',
                'echo.v1.Echo/Ping',
                '--message',
                'cert',
            ]);
            assert.equal(echo.status, 'pass');
            assert.equal(echo.method, 'echo.v1.Echo/Ping');

            const heartbeat = await rpcClient.run([
                'node',
                'holon-rpc-client.js',
                server.address,
                '--method',
                'rpc.heartbeat',
            ]);
            assert.equal(heartbeat.status, 'pass');
            assert.equal(heartbeat.method, 'rpc.heartbeat');

            const unknown = await rpcClient.run([
                'node',
                'holon-rpc-client.js',
                server.address,
                '--method',
                'does.not.Exist/Nope',
                '--expect-error',
                '-32601,12',
            ]);
            assert.equal(unknown.status, 'pass');
            assert.equal(unknown.method, 'does.not.Exist/Nope');
            assert.equal(unknown.error_code, -32601);
        } finally {
            await server.close();
        }
    });

    it('echo-client defaults to stdio:// and runs go-holons echo-server from module root', async () => {
        const echoClient = require('../cmd/echo-client');

        const seen = {
            uri: null,
            options: null,
        };

        const out = await echoClient.run(['node', 'echo-client.js'], {
            grpcclientModule: {
                async dialURI(uri, _ClientCtor, options) {
                    seen.uri = uri;
                    seen.options = options;
                    return {
                        client: {
                            Ping(request, cb) {
                                cb(null, { message: request.message });
                            },
                        },
                        close: async () => {},
                    };
                },
            },
            now: (() => {
                let n = 1000;
                return () => {
                    n += 7;
                    return n;
                };
            })(),
        });

        assert.equal(seen.uri, 'stdio://');
        assert.equal(seen.options.command, process.env.GO_BIN || 'go');
        assert.equal(seen.options.cwd, path.resolve(__dirname, '..', '..', 'go-holons'));
        assert.equal(seen.options.args[0], 'run');
        assert.equal(seen.options.args[1], './cmd/echo-server');
        assert.equal(seen.options.args[2], '--listen');
        assert.equal(seen.options.args[3], 'stdio://');

        assert.equal(out.status, 'pass');
        assert.equal(out.sdk, 'js-holons');
        assert.equal(out.server_sdk, 'go-holons');
        assert.equal(out.latency_ms, 7);
    });

    it('echo-client stdio:// interoperates with go-holons echo-server', async (t) => {
        const goBinary = process.env.GO_BIN || 'go';
        const probe = spawnSync(goBinary, ['version'], { stdio: 'ignore' });
        if (probe.error || probe.status !== 0) {
            t.skip(`${goBinary} binary unavailable`);
            return;
        }

        const goHolonsDir = path.resolve(__dirname, '..', '..', 'go-holons');
        const preflight = spawnSync(goBinary, ['run', './cmd/echo-server', '--help'], {
            cwd: goHolonsDir,
            encoding: 'utf8',
        });
        if (preflight.error || preflight.status !== 0) {
            const firstErrorLine = String(preflight.stderr || preflight.error?.message || '')
                .split(/\r?\n/)
                .find(Boolean) || 'go-holons preflight failed';
            t.skip(firstErrorLine);
            return;
        }

        const echoClient = require('../cmd/echo-client');
        const out = await echoClient.run([
            'node',
            'echo-client.js',
            'stdio://',
            '--message',
            'cert-js-stdio',
            '--timeout-ms',
            '15000',
            '--go',
            goBinary,
        ]);

        assert.equal(out.status, 'pass');
        assert.equal(out.sdk, 'js-holons');
        assert.equal(out.server_sdk, 'go-holons');
        assert.ok(typeof out.latency_ms === 'number');
    });

    it('echo-client ws:// interoperates with go-holons echo-server', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const goBinary = process.env.GO_BIN || 'go';
        const probe = spawnSync(goBinary, ['version'], { stdio: 'ignore' });
        if (probe.error || probe.status !== 0) {
            t.skip(`${goBinary} binary unavailable`);
            return;
        }

        const goHolonsDir = path.resolve(__dirname, '..', '..', 'go-holons');
        const echoServerBinary = await buildGoEchoServerBinary(t, goBinary, goHolonsDir);
        if (!echoServerBinary) {
            return;
        }

        const port = await reserveLoopbackPort();
        const wsURI = `ws://127.0.0.1:${port}/grpc`;

        const server = spawn(
            echoServerBinary,
            [
                '--listen',
                wsURI,
                '--sdk',
                'go-holons',
                '--version',
                '0.3.0',
            ],
            {
                stdio: ['ignore', 'pipe', 'pipe'],
            },
        );

        const stderrChunks = [];
        server.stderr?.on('data', (chunk) => {
            stderrChunks.push(Buffer.from(chunk));
        });

        t.after(async () => {
            await terminateChildProcess(server);
        });

        try {
            await waitForWSReady(wsURI, 'grpc', 20000);
        } catch (err) {
            const stderr = Buffer.concat(stderrChunks).toString('utf8').trim();
            throw new Error(`go ws echo-server did not become ready: ${err.message}\n${stderr}`);
        }

        if (server.exitCode !== null && server.exitCode !== 0) {
            const stderr = Buffer.concat(stderrChunks).toString('utf8').trim();
            throw new Error(`go ws echo-server exited early (${server.exitCode}): ${stderr}`);
        }

        const echoClient = require('../cmd/echo-client');
        const out = await echoClient.run([
            'node',
            'echo-client.js',
            wsURI,
            '--message',
            'cert-js-ws',
            '--timeout-ms',
            '15000',
            '--server-sdk',
            'go-holons',
        ]);

        assert.equal(out.status, 'pass');
        assert.equal(out.sdk, 'js-holons');
        assert.equal(out.server_sdk, 'go-holons');
        assert.ok(typeof out.latency_ms === 'number');
    });

    it('echo-client handles tcp:// targets with pass JSON payload', async () => {
        const echoClient = require('../cmd/echo-client');

        const seen = {
            uri: null,
        };

        const out = await echoClient.run(['node', 'echo-client.js', 'tcp://127.0.0.1:19090', '--message', 'interop'], {
            grpcclientModule: {
                async dialURI(uri) {
                    seen.uri = uri;
                    return {
                        client: {
                            Ping(request, cb) {
                                cb(null, { message: request.message });
                            },
                        },
                        close: async () => {},
                    };
                },
            },
        });

        assert.equal(seen.uri, 'tcp://127.0.0.1:19090');
        assert.equal(out.status, 'pass');
        assert.equal(out.sdk, 'js-holons');
        assert.equal(out.server_sdk, 'go-holons');
        assert.ok(typeof out.latency_ms === 'number');
    });

    it('echo-server parseArgs supports default and explicit flags', () => {
        const echoServer = require('../cmd/echo-server');
        assert.equal(echoServer.parseArgs(['node', 'echo-server.js']).listen, 'tcp://127.0.0.1:0');

        const parsed = echoServer.parseArgs([
            'node',
            'echo-server.js',
            '--listen',
            'tcp://127.0.0.1:8080',
            '--sdk',
            'custom',
            '--version',
            '1.2.3',
        ]);
        assert.equal(parsed.listen, 'tcp://127.0.0.1:8080');
        assert.equal(parsed.sdk, 'custom');
        assert.equal(parsed.version, '1.2.3');
    });
});

// --- Holon-RPC server tests ---

describe('holonrpc', () => {
    it('serves holon-rpc JSON-RPC requests', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const server = new holonrpc.HolonRPCServer('ws://127.0.0.1:0/rpc');
        server.register('echo.v1.Echo/Ping', (params) => params);
        if (!await startHolonRPCServerOrSkip(t, server)) return;

        const ws = new WS(server.address, 'holon-rpc');
        await new Promise((resolve, reject) => {
            ws.once('open', resolve);
            ws.once('error', reject);
        });

        const response = await new Promise((resolve, reject) => {
            ws.once('message', (data) => {
                try {
                    resolve(JSON.parse(data.toString()));
                } catch (err) {
                    reject(err);
                }
            });

            ws.send(JSON.stringify({
                jsonrpc: '2.0',
                id: 'c1',
                method: 'echo.v1.Echo/Ping',
                params: { message: 'hello' },
            }));
        });

        assert.equal(response.jsonrpc, '2.0');
        assert.equal(response.id, 'c1');
        assert.deepEqual(response.result, { message: 'hello' });

        ws.close();
        await server.close();
    });

    it('interop with js-web-holons bidirectional echo', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const HolonClient = await loadJSWebHolonClient();
        const server = new holonrpc.HolonRPCServer('ws://127.0.0.1:0/rpc');
        server.register('echo.v1.Echo/Ping', (params) => params);
        if (!await startHolonRPCServerOrSkip(t, server)) return;

        const connectionPromise = new Promise((resolve) => {
            server.once('connection', resolve);
        });

        const client = new HolonClient(server.address, {
            WebSocket: WS,
            reconnect: false,
            heartbeat: false,
        });
        client.register('client.v1.Client/Hello', (payload) => ({
            message: `hello ${payload.name}`,
        }));

        await client.connect();
        const conn = await connectionPromise;

        const out = await client.invoke('echo.v1.Echo/Ping', { message: 'from-web' });
        assert.equal(out.message, 'from-web');

        const back = await server.invoke(conn, 'client.v1.Client/Hello', { name: 'browser' });
        assert.equal(back.message, 'hello browser');

        client.close();
        await server.close();
    });

    it('client invokes echo, unknown method, and heartbeat', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const server = new holonrpc.HolonRPCServer('ws://127.0.0.1:0/rpc');
        server.register('echo.v1.Echo/Ping', (params) => params);
        if (!await startHolonRPCServerOrSkip(t, server)) return;

        const client = new holonrpc.HolonRPCClient();
        await client.connect(server.address);

        try {
            const echoed = await client.invoke('echo.v1.Echo/Ping', { message: 'hello' });
            assert.deepEqual(echoed, { message: 'hello' });

            await assert.rejects(
                () => client.invoke('does.not.Exist/Nope', {}),
                (err) => err && err.code === -32601
            );

            const heartbeat = await client.invoke('rpc.heartbeat', {});
            assert.deepEqual(heartbeat, {});
        } finally {
            await client.close();
            await server.close();
        }
    });

    it('routes unicast requests to the targeted peer', async (t) => {
        const scenario = await setupRoutingScenario(t);
        if (!scenario) return;

        const { A, B, C, D } = scenario.peers;
        const response = await A.client.invoke('echo.v1.Echo/Ping', {
            _peer: B.id,
            message: 'hello-unicast',
        });

        assert.equal(response.from, 'B');
        await waitForCount(() => B.requestCount, 1, 'peer B request');
        assert.equal(C.requestCount, 0);
        assert.equal(D.requestCount, 0);
        assertRoutingHintsStripped(B.requestParams[0]);
    });

    it('routes fan-out requests and returns aggregated per-peer entries', async (t) => {
        const scenario = await setupRoutingScenario(t);
        if (!scenario) return;

        const { A, B, C, D } = scenario.peers;
        const response = await A.client.invoke('*.Echo/Ping', { message: 'hello-fanout' });

        assert.ok(Array.isArray(response));
        assert.equal(response.length, 3);

        const entriesByPeer = new Map();
        for (const entry of response) {
            entriesByPeer.set(entry.peer, entry);
        }

        for (const peer of [B, C, D]) {
            const entry = entriesByPeer.get(peer.id);
            assert.ok(entry, `missing fan-out entry for ${peer.label}`);
            assert.ok(entry.result, `fan-out entry missing result for ${peer.label}`);
        }

        await waitForCount(() => B.requestCount, 1, 'peer B request');
        await waitForCount(() => C.requestCount, 1, 'peer C request');
        await waitForCount(() => D.requestCount, 1, 'peer D request');
        assertRoutingHintsStripped(B.requestParams[0]);
        assertRoutingHintsStripped(C.requestParams[0]);
        assertRoutingHintsStripped(D.requestParams[0]);
    });

    it('broadcasts targeted peer response to other peers when _routing is broadcast-response', async (t) => {
        const scenario = await setupRoutingScenario(t);
        if (!scenario) return;

        const { A, B, C, D } = scenario.peers;
        const response = await A.client.invoke('echo.v1.Echo/Ping', {
            _peer: B.id,
            _routing: 'broadcast-response',
            message: 'hello-broadcast-response',
        });

        assert.equal(response.from, 'B');
        await waitForCount(() => B.requestCount, 1, 'peer B request');
        assertRoutingHintsStripped(B.requestParams[0]);

        await waitForCount(() => C.notificationCount, 1, 'peer C notification');
        await waitForCount(() => D.notificationCount, 1, 'peer D notification');
        assert.equal(B.notificationCount, 0);

        const notifC = C.notificationParams[0];
        const notifD = D.notificationParams[0];
        for (const notif of [notifC, notifD]) {
            assert.equal(notif.peer, B.id);
            assert.ok(notif.result);
            assert.equal(notif.result.from, 'B');
        }
    });

    it('supports full-broadcast fan-out with cross-peer notifications', async (t) => {
        const scenario = await setupRoutingScenario(t);
        if (!scenario) return;

        const { A, B, C, D } = scenario.peers;
        const response = await A.client.invoke('*.Echo/Ping', {
            message: 'hello-full-broadcast',
            _routing: 'full-broadcast',
        });

        assert.ok(Array.isArray(response));
        assert.equal(response.length, 3);

        for (const peer of [B, C, D]) {
            await waitForCount(() => peer.requestCount, 1, `peer ${peer.label} request`);
            assertRoutingHintsStripped(peer.requestParams[0]);
        }

        await waitForCount(() => B.notificationCount, 2, 'peer B notifications', 3000);
        await waitForCount(() => C.notificationCount, 2, 'peer C notifications', 3000);
        await waitForCount(() => D.notificationCount, 2, 'peer D notifications', 3000);

        for (const peer of [B, C, D]) {
            const seenFrom = new Set();
            for (const notif of peer.notificationParams) {
                assert.ok(notif.result, `missing result payload for ${peer.label}`);
                assert.notEqual(notif.peer, peer.id, `peer ${peer.label} received its own notification`);
                seenFrom.add(notif.peer);
            }
            assert.equal(seenFrom.size, 2, `peer ${peer.label} should receive two distinct peer notifications`);
        }
    });

    it('client reconnects automatically after server restart', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const port = await reserveLoopbackPort();
        const uri = `ws://127.0.0.1:${port}/rpc`;

        const makeServer = () => {
            const server = new holonrpc.HolonRPCServer(uri);
            server.register('echo.v1.Echo/Ping', (params) => ({
                message: String(params.message || ''),
            }));
            return server;
        };

        const client = new holonrpc.HolonRPCClient();
        let server = makeServer();

        await server.start();
        await client.connectWithReconnect(uri, { timeout: 2000 });

        try {
            const first = await client.invoke('echo.v1.Echo/Ping', { message: 'before' }, { timeout: 2000 });
            assert.equal(first.message, 'before');

            await server.close();
            await assert.rejects(
                () => client.invoke('echo.v1.Echo/Ping', { message: 'down' }, { timeout: 1000 }),
                (err) => err && Number(err.code) === 14
            );

            await sleep(1000);
            server = makeServer();
            await server.start();

            const deadline = Date.now() + 10000;
            let recovered = null;
            while (Date.now() < deadline) {
                try {
                    recovered = await client.invoke('echo.v1.Echo/Ping', { message: 'after' }, { timeout: 1000 });
                    break;
                } catch (err) {
                    assert.equal(Number(err.code), 14);
                    await sleep(200);
                }
            }

            assert.ok(recovered, 'client did not reconnect within 10s');
            assert.equal(recovered.message, 'after');
        } finally {
            await client.close();
            await server.close();
        }
    });

    it('client handles server-initiated calls with directional IDs', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const server = new holonrpc.HolonRPCServer('ws://127.0.0.1:0/rpc');
        if (!await startHolonRPCServerOrSkip(t, server)) return;

        const connected = new Promise((resolve) => {
            server.once('connection', resolve);
        });

        const client = new holonrpc.HolonRPCClient();
        client.register('client.v1.Client/Hello', (params) => ({
            message: `hello ${params.name}`,
        }));

        await client.connect(server.address);
        const conn = await connected;

        try {
            const response = await server.invoke(conn, 'client.v1.Client/Hello', { name: 'js' });
            assert.deepEqual(response, { message: 'hello js' });
        } finally {
            await client.close();
            await server.close();
        }
    });
});
