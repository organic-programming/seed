'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');

const { holonrpc } = require('../src/index');

function startHTTPRPCServer(t) {
    let nextID = 1;

    const server = http.createServer(async (req, res) => {
        setCORSHeaders(req, res);

        if (req.method === 'OPTIONS') {
            res.writeHead(204);
            res.end();
            return;
        }

        const url = new URL(req.url, 'http://127.0.0.1');
        const method = url.pathname.replace(/^\/api\/v1\/rpc\//, '');

        if (req.method === 'POST' && method === 'echo.v1.Echo/Ping') {
            const params = await readJSON(req);
            writeJSON(res, 200, {
                jsonrpc: '2.0',
                id: `h${nextID++}`,
                result: {
                    message: String(params.message || ''),
                    sdk: 'rest+sse-server',
                },
            });
            return;
        }

        if (acceptsSSE(req) && method === 'build.v1.Build/Watch') {
            res.writeHead(200, {
                'Content-Type': 'text/event-stream',
                'Cache-Control': 'no-cache',
                Connection: 'keep-alive',
            });

            if (req.method === 'POST') {
                const params = await readJSON(req);
                assert.equal(params.project, 'myapp');
                writeSSE(res, 'message', '1', {
                    jsonrpc: '2.0',
                    id: 'h1',
                    result: { status: 'building', progress: 42 },
                });
                writeSSE(res, 'message', '2', {
                    jsonrpc: '2.0',
                    id: 'h1',
                    result: { status: 'done', progress: 100 },
                });
                writeDone(res);
                return;
            }

            if (req.method === 'GET') {
                assert.equal(url.searchParams.get('project'), 'myapp');
                writeSSE(res, 'message', '1', {
                    jsonrpc: '2.0',
                    id: 'h2',
                    result: { status: 'watching' },
                });
                writeDone(res);
                return;
            }
        }

        writeJSON(res, 404, {
            jsonrpc: '2.0',
            id: 'h0',
            error: {
                code: 5,
                message: `method "${method}" not found`,
            },
        });
    });

    return new Promise((resolve, reject) => {
        server.once('error', reject);
        server.listen(0, '127.0.0.1', () => {
            const address = server.address();
            const baseURL = `http://127.0.0.1:${address.port}/api/v1/rpc`;

            t.after(async () => {
                await new Promise((done) => server.close(() => done()));
            });

            resolve(baseURL);
        });
    });
}

function setCORSHeaders(req, res) {
    const origin = req.headers.origin || '*';
    res.setHeader('Access-Control-Allow-Origin', origin);
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Accept, Last-Event-ID');
    res.setHeader('Access-Control-Max-Age', '86400');
    res.setHeader('Vary', 'Origin');
}

function acceptsSSE(req) {
    return String(req.headers.accept || '').toLowerCase().includes('text/event-stream');
}

function readJSON(req) {
    return new Promise((resolve, reject) => {
        const chunks = [];
        req.on('data', (chunk) => chunks.push(Buffer.from(chunk)));
        req.on('end', () => {
            const text = Buffer.concat(chunks).toString('utf8').trim();
            resolve(text ? JSON.parse(text) : {});
        });
        req.on('error', reject);
    });
}

function writeJSON(res, statusCode, payload) {
    res.writeHead(statusCode, {
        'Content-Type': 'application/json',
    });
    res.end(JSON.stringify(payload));
}

function writeSSE(res, event, id, payload) {
    res.write(`event: ${event}\n`);
    res.write(`id: ${id}\n`);
    res.write(`data: ${JSON.stringify(payload)}\n\n`);
}

function writeDone(res) {
    res.write('event: done\n');
    res.write('data:\n\n');
    res.end();
}

describe('holonrpc http+sse', () => {
    it('invokes unary methods over http://', async (t) => {
        const baseURL = await startHTTPRPCServer(t);
        const client = new holonrpc.HolonRPCClient();
        t.after(async () => client.close());

        await client.connect(baseURL);
        assert.equal(client.connected(), true);

        const result = await client.invoke('echo.v1.Echo/Ping', { message: 'http-audit' });
        assert.equal(result.message, 'http-audit');
        assert.equal(result.sdk, 'rest+sse-server');
    });

    it('accepts rest+sse:// URLs as an http:// alias', async (t) => {
        const baseURL = await startHTTPRPCServer(t);
        const client = new holonrpc.HolonRPCClient();
        t.after(async () => client.close());

        await client.connect(baseURL.replace('http://', 'rest+sse://'));
        const result = await client.invoke('echo.v1.Echo/Ping', { message: 'alias-audit' });
        assert.equal(result.message, 'alias-audit');
    });

    it('streams POST SSE responses', async (t) => {
        const baseURL = await startHTTPRPCServer(t);
        const client = new holonrpc.HolonRPCClient();
        t.after(async () => client.close());

        await client.connect(baseURL);
        const events = await client.stream('build.v1.Build/Watch', { project: 'myapp' });

        assert.equal(events.length, 3);
        assert.equal(events[0].event, 'message');
        assert.equal(events[0].id, '1');
        assert.equal(events[0].result.status, 'building');
        assert.equal(events[1].result.status, 'done');
        assert.equal(events[2].event, 'done');
    });

    it('streams GET SSE responses', async (t) => {
        const baseURL = await startHTTPRPCServer(t);
        const client = new holonrpc.HolonRPCClient();
        t.after(async () => client.close());

        await client.connect(baseURL);
        const events = await client.streamQuery('build.v1.Build/Watch', { project: 'myapp' });

        assert.equal(events.length, 2);
        assert.equal(events[0].event, 'message');
        assert.equal(events[0].result.status, 'watching');
        assert.equal(events[1].event, 'done');
    });
});
