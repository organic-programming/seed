'use strict';

const fs = require('node:fs');
const http = require('node:http');
const https = require('node:https');
const { EventEmitter } = require('node:events');
const { WebSocketServer } = require('ws');

const transport = require('./transport');

const DEFAULT_URI = 'ws://127.0.0.1:0/rpc';
const ROUTE_MODE_DEFAULT = '';
const ROUTE_MODE_BROADCAST_RESPONSE = 'broadcast-response';
const ROUTE_MODE_FULL_BROADCAST = 'full-broadcast';

class HolonRPCServer extends EventEmitter {
    constructor(uri = DEFAULT_URI, options = {}) {
        super();
        this.uri = uri || DEFAULT_URI;
        this.options = options || {};

        this._handlers = new Map();
        this._clients = new Map();

        this._nextClientID = 1;
        this._nextServerRequestID = 1;
        this._running = false;

        this._httpServer = null;
        this._wsServer = null;
        this.address = this.uri;
    }

    register(method, handler) {
        if (typeof method !== 'string' || method.trim() === '') {
            throw new Error('method must be a non-empty string');
        }
        if (typeof handler !== 'function') {
            throw new Error('handler must be a function');
        }
        this._handlers.set(method, handler);
    }

    unregister(method) {
        this._handlers.delete(method);
    }

    listClients() {
        return [...this._clients.values()];
    }

    async start() {
        if (this._running) return;

        const parsed = transport.parseURI(this.uri);
        if (parsed.scheme !== 'ws' && parsed.scheme !== 'wss') {
            throw new Error(`HolonRPCServer expects ws:// or wss:// URI, got ${this.uri}`);
        }

        const tls = parsed.scheme === 'wss' ? resolveTLSOptions(this.options.tls || {}) : null;
        this._httpServer = parsed.scheme === 'wss' ? https.createServer(tls) : http.createServer();

        this._wsServer = new WebSocketServer({
            server: this._httpServer,
            path: parsed.path || '/rpc',
            handleProtocols: (protocols) => {
                if (protocols.has('holon-rpc')) return 'holon-rpc';
                return false;
            },
        });

        this._wsServer.on('connection', (ws) => {
            const client = {
                id: `c${this._nextClientID++}`,
                protocol: 'holon-rpc',
                ws,
                pending: new Map(),
            };

            this._clients.set(client.id, client);
            this.emit('connection', client);

            ws.on('message', async (data, isBinary) => {
                if (isBinary) return;
                await this._handleMessage(client, data.toString('utf8'));
            });

            ws.on('close', () => {
                this._dropClient(client, new Error('connection closed'));
            });

            ws.on('error', (err) => {
                this._dropClient(client, err);
            });
        });

        await new Promise((resolve, reject) => {
            this._httpServer.once('error', reject);
            this._httpServer.listen(
                {
                    host: parsed.host || '0.0.0.0',
                    port: Number(parsed.port || 0),
                },
                () => {
                    this._httpServer.off('error', reject);
                    resolve();
                },
            );
        });

        const addr = this._httpServer.address();
        if (!addr || typeof addr === 'string') {
            throw new Error('failed to determine HolonRPCServer address');
        }

        const host = normalizePublicHost(parsed.host || '0.0.0.0');
        const scheme = parsed.scheme;
        const path = parsed.path || '/rpc';
        this.address = `${scheme}://${host}:${addr.port}${path}`;
        this._running = true;
    }

    async close() {
        if (!this._running) return;
        this._running = false;

        for (const client of this._clients.values()) {
            this._dropClient(client, new Error('server closed'));
            try {
                client.ws.close(1001, 'server shutdown');
            } catch {
                // no-op
            }
        }
        this._clients.clear();

        if (this._wsServer) {
            await new Promise((resolve) => this._wsServer.close(() => resolve()));
            this._wsServer = null;
        }

        if (this._httpServer) {
            await new Promise((resolve) => this._httpServer.close(() => resolve()));
            this._httpServer = null;
        }
    }

    async invoke(clientOrID, method, params = {}, options = {}) {
        const client = this._resolveClient(clientOrID);
        if (!client) {
            throw new Error(`unknown holon-rpc client: ${String(clientOrID)}`);
        }
        if (client.ws.readyState !== client.ws.OPEN) {
            throw new Error(`holon-rpc client ${client.id} is not connected`);
        }
        if (typeof method !== 'string' || method.trim() === '') {
            throw new Error('method must be a non-empty string');
        }

        const id = `s${this._nextServerRequestID++}`;
        const timeout = Number(options.timeout || 5000);
        const payload = { jsonrpc: '2.0', id, method, params: ensureObject(params) };

        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                client.pending.delete(id);
                reject(new Error(`invoke timeout for ${method}`));
            }, timeout);

            client.pending.set(id, { resolve, reject, timer });
            client.ws.send(JSON.stringify(payload), (err) => {
                if (!err) return;
                clearTimeout(timer);
                client.pending.delete(id);
                reject(err);
            });
        });
    }

    _resolveClient(clientOrID) {
        if (!clientOrID) return null;
        if (typeof clientOrID === 'string') {
            return this._clients.get(clientOrID) || null;
        }
        if (typeof clientOrID === 'object' && typeof clientOrID.id === 'string') {
            return this._clients.get(clientOrID.id) || null;
        }
        return null;
    }

    _dropClient(client, err) {
        if (!this._clients.has(client.id)) return;
        this._clients.delete(client.id);

        for (const [id, pending] of client.pending.entries()) {
            clearTimeout(pending.timer);
            pending.reject(err || new Error('connection closed'));
            client.pending.delete(id);
        }

        this.emit('disconnect', client, err);
    }

    async _handleMessage(client, raw) {
        let msg;
        try {
            msg = JSON.parse(raw);
        } catch {
            await this._sendError(client, null, -32700, 'parse error');
            return;
        }

        if (!msg || Array.isArray(msg) || typeof msg !== 'object') {
            await this._sendError(client, null, -32600, 'invalid request');
            return;
        }

        if (Object.prototype.hasOwnProperty.call(msg, 'method')) {
            await this._handleRequest(client, msg);
            return;
        }
        if (Object.prototype.hasOwnProperty.call(msg, 'result') || Object.prototype.hasOwnProperty.call(msg, 'error')) {
            this._handleResponse(client, msg);
            return;
        }
        await this._sendError(client, msg.id ?? null, -32600, 'invalid request');
    }

    async _handleRequest(client, msg) {
        const method = msg.method;
        const id = msg.id;

        if (typeof method !== 'string' || method.trim() === '') {
            await this._sendError(client, id ?? null, -32600, 'invalid request');
            return;
        }

        if (msg.jsonrpc !== '2.0') {
            await this._sendError(client, id ?? null, -32600, 'invalid request');
            return;
        }

        if (method === 'rpc.heartbeat') {
            if (id !== undefined && id !== null) {
                await this._sendResult(client, id, {});
            }
            return;
        }

        let route;
        try {
            route = parseRouteHints(method, msg.params);
        } catch (err) {
            if (hasResponseID(id)) {
                await this._sendRPCError(client, id, err);
            }
            return;
        }

        const routed = await this._routeRequest(client, id, route);
        if (routed) {
            return;
        }

        const handler = this._handlers.get(route.method);
        if (!handler) {
            if (id !== undefined && id !== null) {
                await this._sendError(client, id, -32601, `method "${route.method}" not found`);
            }
            return;
        }

        try {
            const result = await Promise.resolve(handler(route.params, client));
            if (id !== undefined && id !== null) {
                await this._sendResult(client, id, ensureObject(result));
            }
        } catch (err) {
            if (id !== undefined && id !== null) {
                await this._sendError(client, id, 13, err?.message || String(err));
            }
        }
    }

    _handleResponse(client, msg) {
        const id = msg.id;
        if (id === undefined || id === null) return;

        const key = String(id);
        const pending = client.pending.get(key);
        if (!pending) return;

        clearTimeout(pending.timer);
        client.pending.delete(key);

        if (msg.jsonrpc !== '2.0') {
            pending.reject(new Error('invalid response'));
            return;
        }

        if (msg.error) {
            const error = new Error(msg.error.message || 'rpc error');
            error.code = msg.error.code;
            error.data = msg.error.data;
            pending.reject(error);
            return;
        }

        pending.resolve(normalizeResult(msg.result));
    }

    async _sendResult(client, id, result) {
        const payload = { jsonrpc: '2.0', id, result: ensureObject(result) };
        await this._sendJSON(client, payload);
    }

    async _sendResultAny(client, id, result) {
        const payload = { jsonrpc: '2.0', id, result: normalizeResult(result) };
        await this._sendJSON(client, payload);
    }

    async _sendError(client, id, code, message, data) {
        const error = { code: Number(code), message: String(message) };
        if (data !== undefined) {
            error.data = data;
        }
        const payload = { jsonrpc: '2.0', id, error };
        await this._sendJSON(client, payload);
    }

    async _sendJSON(client, payload) {
        if (client.ws.readyState !== client.ws.OPEN) return;
        await new Promise((resolve, reject) => {
            client.ws.send(JSON.stringify(payload), (err) => (err ? reject(err) : resolve()));
        });
    }

    async _sendRPCError(client, id, err) {
        const rpcErr = toRPCError(err);
        await this._sendError(client, id, rpcErr.code, rpcErr.message, rpcErr.data);
    }

    async _routeRequest(caller, requestID, route) {
        if (route.fanOut) {
            let entries;
            try {
                entries = await this._dispatchFanOut(caller, route.method, route.params);
            } catch (err) {
                if (hasResponseID(requestID)) {
                    await this._sendRPCError(caller, requestID, err);
                }
                return true;
            }

            if (route.routing.mode === ROUTE_MODE_FULL_BROADCAST) {
                for (const entry of entries) {
                    const payload = { peer: entry.peer };
                    if (entry.error) {
                        payload.error = entry.error;
                    } else {
                        payload.result = entry.result;
                    }
                    await this._broadcastNotificationMany([caller.id, entry.peer], route.method, payload);
                }
            }

            if (hasResponseID(requestID)) {
                await this._sendResultAny(caller, requestID, entries);
            }
            return true;
        }

        const targetPeerID = route.routing.targetPeerID;
        if (!targetPeerID) {
            return false;
        }

        const target = this._resolveClient(targetPeerID);
        if (!target) {
            if (hasResponseID(requestID)) {
                await this._sendError(caller, requestID, 5, `peer "${targetPeerID}" not found`);
            }
            return true;
        }

        let result;
        try {
            result = await this.invoke(target, route.method, route.params);
        } catch (err) {
            if (hasResponseID(requestID)) {
                await this._sendRPCError(caller, requestID, err);
            }
            return true;
        }

        if (route.routing.mode === ROUTE_MODE_BROADCAST_RESPONSE) {
            await this._broadcastNotificationMany(
                [caller.id, target.id],
                route.method,
                { peer: target.id, result },
            );
        }

        if (hasResponseID(requestID)) {
            await this._sendResult(caller, requestID, result);
        }
        return true;
    }

    async _dispatchFanOut(caller, method, params) {
        const targets = [];
        for (const peer of this._clients.values()) {
            if (peer.id === caller.id) {
                continue;
            }
            targets.push(peer);
        }

        if (targets.length === 0) {
            throw createRPCError(5, 'no connected peers');
        }

        const entries = [];
        await Promise.all(targets.map(async (target) => {
            try {
                const result = await this.invoke(target, method, params);
                entries.push({
                    peer: target.id,
                    result,
                });
            } catch (err) {
                entries.push({
                    peer: target.id,
                    error: toRPCError(err),
                });
            }
        }));
        return entries;
    }

    async _broadcastNotificationMany(excludedIDs, method, params) {
        const excluded = new Set(excludedIDs || []);
        const payload = {
            jsonrpc: '2.0',
            method,
            params: ensureObject(params),
        };

        const writes = [];
        for (const client of this._clients.values()) {
            if (excluded.has(client.id)) {
                continue;
            }
            writes.push(
                this._sendJSON(client, payload).catch(() => {
                    // Best-effort broadcast.
                }),
            );
        }
        await Promise.all(writes);
    }
}

function ensureObject(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
        return {};
    }
    return value;
}

function normalizeResult(result) {
    if (result === undefined) {
        return {};
    }
    return result;
}

function hasResponseID(id) {
    return id !== undefined && id !== null;
}

function parseRouteHints(method, rawParams) {
    let dispatchMethod = String(method || '').trim();
    if (dispatchMethod === '') {
        throw createRPCError(-32600, 'invalid request');
    }

    const params = { ...ensureObject(rawParams) };
    const routing = {
        mode: ROUTE_MODE_DEFAULT,
        targetPeerID: '',
    };

    if (Object.prototype.hasOwnProperty.call(params, '_routing')) {
        const rawMode = params._routing;
        if (typeof rawMode !== 'string') {
            throw createRPCError(-32602, '_routing must be a string');
        }

        const mode = rawMode.trim();
        if (
            mode !== ROUTE_MODE_DEFAULT &&
            mode !== ROUTE_MODE_BROADCAST_RESPONSE &&
            mode !== ROUTE_MODE_FULL_BROADCAST
        ) {
            throw createRPCError(-32602, `unsupported _routing "${mode}"`);
        }
        routing.mode = mode;
        delete params._routing;
    }

    if (Object.prototype.hasOwnProperty.call(params, '_peer')) {
        const rawPeer = params._peer;
        if (typeof rawPeer !== 'string') {
            throw createRPCError(-32602, '_peer must be a string');
        }
        const peerID = rawPeer.trim();
        if (peerID === '') {
            throw createRPCError(-32602, '_peer must be non-empty');
        }
        routing.targetPeerID = peerID;
        delete params._peer;
    }

    const fanOut = dispatchMethod.startsWith('*.');
    if (fanOut) {
        dispatchMethod = dispatchMethod.slice(2).trim();
        if (dispatchMethod === '') {
            throw createRPCError(-32600, 'invalid fan-out method');
        }
    }

    if (routing.mode === ROUTE_MODE_FULL_BROADCAST && !fanOut) {
        throw createRPCError(-32602, 'full-broadcast requires a fan-out method');
    }

    return {
        method: dispatchMethod,
        params,
        routing,
        fanOut,
    };
}

function createRPCError(code, message, data) {
    const err = new Error(String(message || 'rpc error'));
    err.code = Number(code);
    if (data !== undefined) {
        err.data = data;
    }
    return err;
}

function toRPCError(err) {
    const code = Number(err?.code);
    const message = typeof err?.message === 'string' && err.message.trim() !== ''
        ? err.message
        : 'rpc error';

    if (Number.isFinite(code)) {
        const out = { code, message };
        if (err && Object.prototype.hasOwnProperty.call(err, 'data')) {
            out.data = err.data;
        }
        return out;
    }

    return {
        code: 14,
        message,
    };
}

function normalizePublicHost(host) {
    if (!host || host === '0.0.0.0') {
        return '127.0.0.1';
    }
    return host;
}

function resolveTLSOptions(tlsOptions = {}) {
    const key = tlsOptions.key || readFileMaybe(tlsOptions.keyFile || process.env.HOLONS_TLS_KEY_FILE);
    const cert = tlsOptions.cert || readFileMaybe(tlsOptions.certFile || process.env.HOLONS_TLS_CERT_FILE);
    if (!key || !cert) {
        throw new Error(
            'wss:// requires TLS key/cert. Provide options.tls.{key,cert} or HOLONS_TLS_KEY_FILE/HOLONS_TLS_CERT_FILE.',
        );
    }
    return { key, cert };
}

function readFileMaybe(filePath) {
    if (!filePath) return undefined;
    return fs.readFileSync(filePath);
}

module.exports = {
    HolonRPCServer,
    DEFAULT_URI,
};
