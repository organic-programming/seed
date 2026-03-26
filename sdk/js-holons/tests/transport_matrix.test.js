'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const grpc = require('@grpc/grpc-js');

const sdk = require('../src/index');
const echoServer = require('../cmd/echo-server');

const SDK_ROOT = path.resolve(__dirname, '..');
const EchoClient = grpc.makeGenericClientConstructor(echoServer.ECHO_SERVICE_DEF, 'Echo', {});

function sampleStaticDescribeResponse() {
    return {
        manifest: {
            identity: {
                schema: 'holon/v1',
                uuid: 'transport-audit-0000',
                given_name: 'Transport',
                family_name: 'Audit',
                motto: 'Prove every transport.',
                composer: 'transport-matrix-test',
                status: 'draft',
                born: '2026-03-23',
            },
            lang: 'js',
        },
        services: [{
            name: 'echo.v1.Echo',
            description: 'Unary echo transport audit service.',
            methods: [{
                name: 'Ping',
                input_type: 'echo.v1.PingRequest',
                output_type: 'echo.v1.PingResponse',
            }],
        }],
    };
}

function quietLogger() {
    return {
        error() {},
        warn() {},
    };
}

function canListenOnLoopback() {
    return new Promise((resolve) => {
        const probe = require('node:net').createServer();
        probe.once('error', (err) => {
            const code = err && err.code;
            resolve(code !== 'EPERM' && code !== 'EACCES');
        });
        probe.listen(0, '127.0.0.1', () => {
            probe.close(() => resolve(true));
        });
    });
}

async function withStaticDescribeResponse(t) {
    sdk.describe.useStaticResponse(sampleStaticDescribeResponse());
    t.after(() => {
        sdk.describe.useStaticResponse(null);
    });
}

async function startEchoServer(t, listenUri, options = {}) {
    await withStaticDescribeResponse(t);

    const server = await sdk.serve.runWithOptions(
        listenUri,
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
            logger: quietLogger(),
            ...options,
        },
    );

    t.after(async () => {
        await server.stopHolon();
    });

    return server;
}

async function dialEcho(t, uri, options = {}) {
    const session = await sdk.grpcclient.dialURI(uri, EchoClient, options);
    t.after(async () => {
        await session.close();
    });
    return session.client;
}

function invokePing(client, message) {
    return new Promise((resolve, reject) => {
        client.Ping({ message }, (err, out) => {
            if (err) {
                reject(err);
                return;
            }
            resolve(out || {});
        });
    });
}

function writeStaticEchoServerBootstrap(runtimeRoot) {
    const scriptPath = path.join(runtimeRoot, 'static-echo-server.js');
    fs.writeFileSync(scriptPath, [
        "'use strict';",
        '',
        `const sdk = require(${JSON.stringify(path.join(SDK_ROOT, 'src', 'index.js'))});`,
        `const echoServer = require(${JSON.stringify(path.join(SDK_ROOT, 'cmd', 'echo-server.js'))});`,
        '',
        `sdk.describe.useStaticResponse(${JSON.stringify(sampleStaticDescribeResponse())});`,
        '',
        'echoServer.run(process.argv).catch((err) => {',
        "    process.stderr.write(`${err.stack || err.message}\\n`);",
        '    process.exit(1);',
        '});',
        '',
    ].join('\n'));
    return scriptPath;
}

function createSelfSignedTLSFiles(root) {
    const keyPath = path.join(root, 'tls.key');
    const certPath = path.join(root, 'tls.crt');
    const result = spawnSync('openssl', [
        'req',
        '-x509',
        '-newkey',
        'rsa:2048',
        '-nodes',
        '-keyout',
        keyPath,
        '-out',
        certPath,
        '-subj',
        '/CN=127.0.0.1',
        '-days',
        '1',
        '-addext',
        'subjectAltName = IP:127.0.0.1,DNS:localhost',
    ], {
        encoding: 'utf8',
    });

    if (result.status !== 0) {
        throw new Error(`openssl req failed: ${result.stderr || result.stdout || 'unknown error'}`);
    }

    return {
        key: fs.readFileSync(keyPath),
        cert: fs.readFileSync(certPath),
    };
}

async function startHTTPRPCServer(t) {
    const server = http.createServer(async (req, res) => {
        if (req.method === 'POST' && req.url === '/api/v1/rpc/echo.v1.Echo/Ping') {
            const chunks = [];
            req.on('data', (chunk) => chunks.push(Buffer.from(chunk)));
            req.on('end', () => {
                const payload = JSON.parse(Buffer.concat(chunks).toString('utf8') || '{}');
                res.writeHead(200, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({
                    jsonrpc: '2.0',
                    id: 'h1',
                    result: {
                        message: String(payload.message || ''),
                    },
                }));
            });
            return;
        }

        res.writeHead(404, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
            jsonrpc: '2.0',
            id: 'h0',
            error: { code: 5, message: 'method not found' },
        }));
    });

    await new Promise((resolve, reject) => {
        server.once('error', reject);
        server.listen(0, '127.0.0.1', resolve);
    });

    t.after(async () => {
        await new Promise((resolve) => server.close(() => resolve()));
    });

    const address = server.address();
    return `rest+sse://127.0.0.1:${address.port}/api/v1/rpc`;
}

describe('transport capability matrix', { concurrency: 1 }, () => {
    it('proves tcp:// supports serve and dial', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const server = await startEchoServer(t, 'tcp://127.0.0.1:0');
        const client = await dialEcho(t, server.__holonsRuntime.publicURI);
        const response = await invokePing(client, 'tcp-audit');
        assert.equal(response.message, 'tcp-audit');
        assert.equal(response.sdk, 'js-holons');
    });

    it('proves unix:// supports serve and dial', async (t) => {
        const socketPath = path.join(os.tmpdir(), `js-holons-audit-${process.pid}-${Date.now()}.sock`);
        const server = await startEchoServer(t, `unix://${socketPath}`);
        const client = await dialEcho(t, server.__holonsRuntime.publicURI);
        const response = await invokePing(client, 'unix-audit');
        assert.equal(response.message, 'unix-audit');
        assert.equal(response.sdk, 'js-holons');
    });

    it('proves stdio:// supports serve and dial', async (t) => {
        const runtimeRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'js-holons-stdio-audit-'));
        const scriptPath = writeStaticEchoServerBootstrap(runtimeRoot);

        t.after(() => {
            fs.rmSync(runtimeRoot, { recursive: true, force: true });
        });

        const client = await dialEcho(t, 'stdio://', {
            command: process.execPath,
            args: [scriptPath, '--listen', 'stdio://'],
            cwd: runtimeRoot,
        });
        const response = await invokePing(client, 'stdio-audit');
        assert.equal(response.message, 'stdio-audit');
        assert.equal(response.sdk, 'js-holons');
    });

    it('proves ws:// supports serve and dial', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const server = await startEchoServer(t, 'ws://127.0.0.1:0/grpc');
        const client = await dialEcho(t, server.__holonsRuntime.publicURI);
        const response = await invokePing(client, 'ws-audit');
        assert.equal(response.message, 'ws-audit');
        assert.equal(response.sdk, 'js-holons');
    });

    it('proves wss:// supports serve and dial', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const tlsRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'js-holons-wss-audit-'));
        t.after(() => {
            fs.rmSync(tlsRoot, { recursive: true, force: true });
        });

        const tls = createSelfSignedTLSFiles(tlsRoot);
        const server = await startEchoServer(t, 'wss://127.0.0.1:0/grpc', {
            ws: { tls },
        });
        const client = await dialEcho(t, server.__holonsRuntime.publicURI, {
            ws: { rejectUnauthorized: false },
        });
        const response = await invokePing(client, 'wss-audit');
        assert.equal(response.message, 'wss-audit');
        assert.equal(response.sdk, 'js-holons');
    });

    it('proves rest+sse supports dial', async (t) => {
        const uri = await startHTTPRPCServer(t);
        const client = new sdk.holonrpc.HolonRPCClient();
        t.after(async () => {
            await client.close();
        });

        await client.connect(uri);
        const result = await client.invoke('echo.v1.Echo/Ping', { message: 'rest-audit' });
        assert.equal(result.message, 'rest-audit');
    });
});
