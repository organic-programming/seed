'use strict';

const { EventEmitter } = require('node:events');
const { WebSocket } = require('ws');

const DEFAULT_CONNECT_TIMEOUT_MS = 5000;
const DEFAULT_INVOKE_TIMEOUT_MS = 5000;
const DEFAULT_MAX_PAYLOAD_BYTES = 1 << 20;
const DEFAULT_HTTP_RPC_PATH = '/api/v1/rpc';
const DEFAULT_RECONNECT_MIN_DELAY_MS = 500;
const DEFAULT_RECONNECT_MAX_DELAY_MS = 30 * 1000;
const DEFAULT_RECONNECT_FACTOR = 2;
const DEFAULT_RECONNECT_JITTER = 0.1;

class HolonRPCError extends Error {
    constructor(code, message, data) {
        super(String(message || 'rpc error'));
        this.name = 'HolonRPCError';
        this.code = Number.isFinite(Number(code)) ? Number(code) : 13;
        if (data !== undefined) {
            this.data = data;
        }
    }
}

class HolonRPCClient extends EventEmitter {
    constructor(options = {}) {
        super();
        this.options = options || {};

        this._handlers = new Map();
        this._pending = new Map();
        this._nextClientID = 1;

        this._ws = null;
        this._httpState = null;
        this._connectedURL = '';
        this._closingExplicitly = false;
        this._reconnectState = null;
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

    connected() {
        return (this._ws !== null && this._ws.readyState === WebSocket.OPEN) || this._httpState !== null;
    }

    async connect(url, options = {}) {
        this._assertConnectable();
        const settings = resolveConnectSettings(this.options, url, options);
        if (settings.transport === 'http') {
            this._connectHTTP(settings.target, settings.httpOptions);
            return;
        }
        await this._connectSocket(settings.target, settings.wsOptions);
    }

    async connectWithReconnect(url, options = {}) {
        this._assertConnectable();
        const settings = resolveConnectSettings(this.options, url, options);
        if (settings.transport === 'http') {
            this._connectHTTP(settings.target, settings.httpOptions);
            return;
        }

        const reconnectState = {
            target: settings.target,
            wsOptions: settings.wsOptions,
            attempt: 0,
            timer: null,
            inFlight: false,
        };
        this._reconnectState = reconnectState;

        try {
            await this._connectSocket(settings.target, settings.wsOptions, reconnectState);
        } catch (err) {
            this._disableReconnect();
            throw err;
        }
    }

    async close() {
        this._closingExplicitly = true;
        this._disableReconnect();

        const ws = this._ws;
        const httpState = this._httpState;
        this._ws = null;
        this._httpState = null;
        this._connectedURL = '';

        this._failAllPending(new HolonRPCError(14, 'connection closed'));

        if (httpState) {
            this.emit('disconnect');
            this._closingExplicitly = false;
            return;
        }

        if (!ws) {
            this._closingExplicitly = false;
            return;
        }

        try {
            await closeSocket(ws, 1000, 'client close');
        } finally {
            this._closingExplicitly = false;
        }
    }

    async invoke(method, params = {}, options = {}) {
        if (typeof method !== 'string' || method.trim() === '') {
            throw new Error('holon-rpc: method is required');
        }

        const httpState = this._httpState;
        if (httpState) {
            return invokeHTTP(httpState, method, params, options, this.options);
        }

        const ws = this._ws;
        if (!ws || ws.readyState !== WebSocket.OPEN) {
            throw new HolonRPCError(14, 'connection closed');
        }

        const timeout = positiveInt(
            options.timeout ?? this.options.invokeTimeout ?? DEFAULT_INVOKE_TIMEOUT_MS,
            'invoke timeout'
        );

        const id = `c${this._nextClientID++}`;
        const payload = {
            jsonrpc: '2.0',
            id,
            method,
            params: ensureObject(params),
        };

        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                const pending = this._pending.get(id);
                if (!pending) return;
                this._pending.delete(id);
                pending.reject(new HolonRPCError(4, `timeout after ${timeout}ms`));
            }, timeout);

            timer.unref?.();
            this._pending.set(id, { resolve, reject, timer });

            ws.send(JSON.stringify(payload), (err) => {
                if (!err) return;

                const pending = this._pending.get(id);
                if (!pending) return;

                clearTimeout(pending.timer);
                this._pending.delete(id);
                pending.reject(new HolonRPCError(14, err.message || 'connection closed'));
            });
        });
    }

    async stream(method, params = {}, options = {}) {
        const httpState = this._httpState;
        if (!httpState) {
            throw new Error('holon-rpc: HTTP+SSE streaming requires an http://, https://, or rest+sse:// connection');
        }
        return streamHTTP(httpState, method, params, options, this.options);
    }

    async streamQuery(method, params = {}, options = {}) {
        const httpState = this._httpState;
        if (!httpState) {
            throw new Error('holon-rpc: HTTP+SSE streaming requires an http://, https://, or rest+sse:// connection');
        }
        return streamHTTPQuery(httpState, method, params, options, this.options);
    }

    async _handleMessage(data, isBinary) {
        if (isBinary) {
            return;
        }

        let msg;
        try {
            msg = JSON.parse(decodeMessage(data));
        } catch {
            return;
        }

        if (!msg || Array.isArray(msg) || typeof msg !== 'object') {
            return;
        }

        if (Object.prototype.hasOwnProperty.call(msg, 'method')) {
            await this._handleRequest(msg);
            return;
        }

        if (Object.prototype.hasOwnProperty.call(msg, 'result') || Object.prototype.hasOwnProperty.call(msg, 'error')) {
            this._handleResponse(msg);
        }
    }

    async _handleRequest(msg) {
        const id = msg.id;
        const hasID = id !== undefined && id !== null;

        if (msg.jsonrpc !== '2.0') {
            if (hasID) {
                await this._sendError(id, -32600, 'invalid request');
            }
            return;
        }

        const method = msg.method;
        if (typeof method !== 'string' || method.trim() === '') {
            if (hasID) {
                await this._sendError(id, -32600, 'invalid request');
            }
            return;
        }

        if (hasID && (typeof id !== 'string' || !id.startsWith('s'))) {
            await this._sendError(id, -32600, 'invalid request id');
            return;
        }

        if (method === 'rpc.heartbeat') {
            if (hasID) {
                await this._sendResult(id, {});
            }
            return;
        }

        const handler = this._handlers.get(method);
        if (!handler) {
            if (hasID) {
                await this._sendError(id, -32601, `method "${method}" not found`);
            }
            return;
        }

        let params;
        try {
            params = decodeParams(msg.params);
        } catch (err) {
            if (hasID) {
                await this._sendError(id, -32602, err.message || 'invalid params');
            }
            return;
        }

        try {
            const result = await Promise.resolve(handler(params));
            if (hasID) {
                await this._sendResult(id, ensureObject(result));
            }
        } catch (err) {
            if (hasID) {
                await this._sendError(id, 13, err?.message || String(err));
            }
        }
    }

    _handleResponse(msg) {
        const id = msg.id;
        if (id === undefined || id === null) {
            return;
        }

        const key = String(id);
        const pending = this._pending.get(key);
        if (!pending) {
            return;
        }

        clearTimeout(pending.timer);
        this._pending.delete(key);

        if (msg.jsonrpc !== '2.0') {
            pending.reject(new Error('invalid response')); // protocol violation
            return;
        }

        if (msg.error) {
            pending.reject(new HolonRPCError(msg.error.code, msg.error.message || 'rpc error', msg.error.data));
            return;
        }

        pending.resolve(normalizeResult(msg.result));
    }

    async _sendResult(id, result) {
        await this._sendJSON({
            jsonrpc: '2.0',
            id,
            result: ensureObject(result),
        });
    }

    async _sendError(id, code, message, data) {
        const payload = {
            jsonrpc: '2.0',
            id,
            error: {
                code: Number(code),
                message: String(message || 'rpc error'),
            },
        };

        if (data !== undefined) {
            payload.error.data = data;
        }

        await this._sendJSON(payload);
    }

    async _sendJSON(payload) {
        const ws = this._ws;
        if (!ws || ws.readyState !== WebSocket.OPEN) {
            throw new HolonRPCError(14, 'connection closed');
        }

        await new Promise((resolve, reject) => {
            ws.send(JSON.stringify(payload), (err) => {
                if (err) {
                    reject(new HolonRPCError(14, err.message || 'connection closed'));
                    return;
                }
                resolve();
            });
        });
    }

    _failAllPending(err) {
        for (const pending of this._pending.values()) {
            clearTimeout(pending.timer);
            pending.reject(err);
        }
        this._pending.clear();
    }

    _assertConnectable() {
        if (this.connected()) {
            throw new Error('holon-rpc: client already connected');
        }
        if (this._ws && this._ws.readyState === WebSocket.CONNECTING) {
            throw new Error('holon-rpc: client is already connecting');
        }
        if (this._reconnectState) {
            throw new Error('holon-rpc: reconnect loop already active');
        }
    }

    async _connectSocket(target, wsOptions, reconnectState = null) {
        const ws = new WebSocket(target, 'holon-rpc', wsOptions);
        await waitForSocketOpen(ws);

        if (ws.protocol !== 'holon-rpc') {
            try {
                ws.close(1002, 'missing holon-rpc subprotocol');
            } catch {
                // no-op
            }
            throw new Error('holon-rpc: server did not negotiate holon-rpc');
        }

        if ((reconnectState && this._reconnectState !== reconnectState) || this._closingExplicitly) {
            await closeSocket(ws, 1000, 'connection canceled');
            throw new Error('holon-rpc: connection attempt canceled');
        }

        this._ws = ws;
        this._connectedURL = target;
        this._attachSocket(ws, target);
        this.emit('connect', { url: target });
    }

    _connectHTTP(target, httpOptions) {
        this._httpState = {
            baseURL: target,
            fetchImpl: httpOptions.fetchImpl,
            headers: httpOptions.headers,
        };
        this._connectedURL = target;
        this.emit('connect', { url: target });
    }

    _attachSocket(ws, target) {
        ws.on('message', (data, isBinary) => {
            void this._handleMessage(data, isBinary);
        });

        ws.on('close', () => {
            if (this._ws !== ws) return;

            this._ws = null;
            this._connectedURL = '';
            this._failAllPending(new HolonRPCError(14, 'connection closed'));
            this.emit('disconnect');

            if (this._closingExplicitly || !this._reconnectState) {
                return;
            }

            this._scheduleReconnect(target);
        });

        // Prevent unhandled "error" events from taking down the process.
        ws.on('error', () => {});
    }

    _scheduleReconnect(target) {
        const state = this._reconnectState;
        if (!state || state.timer || state.inFlight || this._closingExplicitly) {
            return;
        }

        const delay = reconnectDelayMs(state.attempt);
        state.attempt += 1;

        state.timer = setTimeout(async () => {
            if (!this._reconnectState || this._reconnectState !== state) {
                return;
            }

            state.timer = null;
            state.inFlight = true;
            try {
                await this._connectSocket(target, state.wsOptions, state);
                state.attempt = 0;
            } catch {
                // Keep retrying while reconnect mode is active.
            } finally {
                state.inFlight = false;
            }

            if (this._reconnectState && this._reconnectState === state && !this.connected()) {
                this._scheduleReconnect(target);
            }
        }, delay);

        state.timer.unref?.();
    }

    _disableReconnect() {
        const state = this._reconnectState;
        this._reconnectState = null;
        if (!state || !state.timer) return;
        clearTimeout(state.timer);
        state.timer = null;
    }
}

function decodeMessage(data) {
    if (typeof data === 'string') {
        return data;
    }
    if (Buffer.isBuffer(data)) {
        return data.toString('utf8');
    }
    if (data instanceof ArrayBuffer) {
        return Buffer.from(data).toString('utf8');
    }
    if (ArrayBuffer.isView(data)) {
        return Buffer.from(data.buffer, data.byteOffset, data.byteLength).toString('utf8');
    }
    throw new Error('unsupported message type');
}

function decodeParams(params) {
    if (params === undefined || params === null) {
        return {};
    }
    if (Array.isArray(params) || typeof params !== 'object') {
        throw new Error('params must be an object');
    }
    return params;
}

function ensureObject(value) {
    if (!value || Array.isArray(value) || typeof value !== 'object') {
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

function positiveInt(value, fieldName) {
    const parsed = Number(value);
    if (!Number.isFinite(parsed) || parsed <= 0) {
        throw new Error(`${fieldName} must be a positive number`);
    }
    return Math.trunc(parsed);
}

function normalizeHolonRPCTarget(url) {
    const raw = normalizeRestSSEURL(url);
    let parsed;
    try {
        parsed = new URL(raw);
    } catch {
        throw new Error(`invalid holon-rpc URL: ${url}`);
    }

    if (parsed.protocol === 'ws:' || parsed.protocol === 'wss:') {
        if (!parsed.pathname || parsed.pathname === '/') {
            parsed.pathname = '/rpc';
        }

        return {
            transport: 'ws',
            target: parsed.toString(),
        };
    }

    if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
        if (!parsed.pathname || parsed.pathname === '/') {
            parsed.pathname = DEFAULT_HTTP_RPC_PATH;
        }

        return {
            transport: 'http',
            target: trimTrailingSlash(parsed.toString()),
        };
    }

    throw new Error(`holon-rpc client expects ws://, wss://, http://, https://, or rest+sse:// URL, got ${url}`);
}

function normalizeRestSSEURL(url) {
    const trimmed = String(url || '').trim();
    if (trimmed.startsWith('rest+sse://')) {
        return `http://${trimmed.slice('rest+sse://'.length)}`;
    }
    if (trimmed.startsWith('rest+sses://')) {
        return `https://${trimmed.slice('rest+sses://'.length)}`;
    }
    return trimmed;
}

function trimTrailingSlash(url) {
    return url.endsWith('/') ? url.slice(0, -1) : url;
}

function resolveConnectSettings(defaultOptions, url, options) {
    if (typeof url !== 'string' || url.trim() === '') {
        throw new Error('holon-rpc: url is required');
    }

    const normalized = normalizeHolonRPCTarget(url);
    const timeout = positiveInt(
        options.timeout ?? defaultOptions.connectTimeout ?? DEFAULT_CONNECT_TIMEOUT_MS,
        'connect timeout'
    );

    if (normalized.transport === 'http') {
        const httpDefaults = defaultOptions.http || {};
        const httpOverrides = options.http || {};
        const fetchImpl = httpOverrides.fetch || httpDefaults.fetch || globalThis.fetch;
        if (typeof fetchImpl !== 'function') {
            throw new Error('holon-rpc: fetch implementation is required for HTTP+SSE');
        }

        return {
            transport: 'http',
            target: normalized.target,
            httpOptions: {
                fetchImpl,
                headers: {
                    ...(httpDefaults.headers || {}),
                    ...(httpOverrides.headers || {}),
                },
                timeout,
            },
        };
    }

    const wsOptions = {
        ...(defaultOptions.ws || {}),
        ...(options.ws || {}),
    };

    if (wsOptions.handshakeTimeout === undefined) {
        wsOptions.handshakeTimeout = timeout;
    }
    if (wsOptions.maxPayload === undefined) {
        wsOptions.maxPayload = DEFAULT_MAX_PAYLOAD_BYTES;
    }

    return {
        transport: 'ws',
        target: normalized.target,
        wsOptions,
    };
}

async function invokeHTTP(state, method, params, options, defaultOptions) {
    const timeout = positiveInt(
        options.timeout ?? defaultOptions.invokeTimeout ?? DEFAULT_INVOKE_TIMEOUT_MS,
        'invoke timeout'
    );

    const response = await fetchWithTimeout(
        state.fetchImpl,
        methodURL(state.baseURL, method),
        {
            method: 'POST',
            headers: {
                ...state.headers,
                Accept: 'application/json',
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(ensureObject(params)),
        },
        timeout,
    );

    const text = await response.text();
    return decodeHTTPRPCResponse(response, text);
}

async function streamHTTP(state, method, params, options, defaultOptions) {
    const timeout = positiveInt(
        options.timeout ?? defaultOptions.invokeTimeout ?? DEFAULT_INVOKE_TIMEOUT_MS,
        'invoke timeout'
    );

    const response = await fetchWithTimeout(
        state.fetchImpl,
        methodURL(state.baseURL, method),
        {
            method: 'POST',
            headers: {
                ...state.headers,
                Accept: 'text/event-stream',
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(ensureObject(params)),
        },
        timeout,
    );

    const text = await response.text();
    return readSSEEvents(response, text);
}

async function streamHTTPQuery(state, method, params, options, defaultOptions) {
    const timeout = positiveInt(
        options.timeout ?? defaultOptions.invokeTimeout ?? DEFAULT_INVOKE_TIMEOUT_MS,
        'invoke timeout'
    );

    const endpoint = new URL(methodURL(state.baseURL, method));
    for (const [key, value] of Object.entries(ensureObject(params))) {
        if (Array.isArray(value)) {
            for (const item of value) {
                endpoint.searchParams.append(key, String(item));
            }
        } else {
            endpoint.searchParams.set(key, String(value));
        }
    }

    const response = await fetchWithTimeout(
        state.fetchImpl,
        endpoint.toString(),
        {
            method: 'GET',
            headers: {
                ...state.headers,
                Accept: 'text/event-stream',
            },
        },
        timeout,
    );

    const text = await response.text();
    return readSSEEvents(response, text);
}

async function fetchWithTimeout(fetchImpl, url, init, timeout) {
    const controller = new AbortController();
    const timer = setTimeout(() => {
        controller.abort();
    }, timeout);
    timer.unref?.();

    try {
        return await fetchImpl(url, {
            ...init,
            signal: controller.signal,
        });
    } catch (err) {
        if (err && err.name === 'AbortError') {
            throw new HolonRPCError(4, `timeout after ${timeout}ms`);
        }
        throw new HolonRPCError(14, err?.message || 'connection closed');
    } finally {
        clearTimeout(timer);
    }
}

function methodURL(baseURL, method) {
    return `${trimTrailingSlash(baseURL)}/${String(method || '').trim().replace(/^\/+/, '')}`;
}

function decodeHTTPRPCResponse(response, text) {
    let payload = null;
    try {
        payload = JSON.parse(text);
    } catch {}

    if (payload && payload.error) {
        throw new HolonRPCError(payload.error.code, payload.error.message || 'rpc error', payload.error.data);
    }
    if (payload && payload.jsonrpc === '2.0' && Object.prototype.hasOwnProperty.call(payload, 'result')) {
        return normalizeResult(payload.result);
    }
    if (!response.ok) {
        throw new HolonRPCError(response.status === 404 ? 5 : 13, `http status ${response.status}`);
    }
    if (payload && !Array.isArray(payload) && typeof payload === 'object') {
        return normalizeResult(payload);
    }
    return {};
}

function readSSEEvents(response, text) {
    if (!response.ok) {
        decodeHTTPRPCResponse(response, text);
    }

    const events = [];
    let current = {
        event: '',
        id: '',
        data: '',
    };

    const flush = () => {
        if (!current.event && !current.id && !current.data) {
            return;
        }

        const event = {
            event: current.event || 'message',
            id: current.id,
        };

        if ((event.event === 'message' || event.event === 'error') && current.data) {
            const payload = JSON.parse(current.data);
            if (payload.error) {
                event.error = payload.error;
            } else {
                event.result = normalizeResult(payload.result);
            }
        }

        events.push(event);
        current = { event: '', id: '', data: '' };
    };

    for (const line of String(text || '').split(/\r?\n/)) {
        if (line === '') {
            flush();
            continue;
        }
        if (line.startsWith('event:')) {
            current.event = line.slice('event:'.length).trim();
            continue;
        }
        if (line.startsWith('id:')) {
            current.id = line.slice('id:'.length).trim();
            continue;
        }
        if (line.startsWith('data:')) {
            const value = line.slice('data:'.length).trim();
            current.data = current.data ? `${current.data}\n${value}` : value;
        }
    }
    flush();

    return events;
}

function reconnectDelayMs(attempt) {
    const base = Math.min(
        DEFAULT_RECONNECT_MIN_DELAY_MS * (DEFAULT_RECONNECT_FACTOR ** attempt),
        DEFAULT_RECONNECT_MAX_DELAY_MS
    );
    const jitter = 1 + (Math.random() * DEFAULT_RECONNECT_JITTER);
    return Math.max(DEFAULT_RECONNECT_MIN_DELAY_MS, Math.floor(base * jitter));
}

function waitForSocketOpen(ws) {
    return new Promise((resolve, reject) => {
        let settled = false;

        const cleanup = () => {
            ws.off('open', onOpen);
            ws.off('error', onError);
            ws.off('close', onClose);
        };

        const onOpen = () => {
            if (settled) return;
            settled = true;
            cleanup();
            resolve();
        };

        const onError = (err) => {
            if (settled) return;
            settled = true;
            cleanup();
            reject(err || new Error('holon-rpc connection failed'));
        };

        const onClose = () => {
            if (settled) return;
            settled = true;
            cleanup();
            reject(new Error('holon-rpc connection closed before open'));
        };

        ws.once('open', onOpen);
        ws.once('error', onError);
        ws.once('close', onClose);
    });
}

function closeSocket(ws, code = 1000, reason = 'client close') {
    return new Promise((resolve) => {
        if (ws.readyState === WebSocket.CLOSED) {
            resolve();
            return;
        }

        const done = () => {
            ws.off('close', done);
            resolve();
        };

        ws.once('close', done);

        try {
            if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
                ws.close(code, reason);
            }
        } catch {
            resolve();
        }
    });
}

module.exports = {
    HolonRPCClient,
    HolonRPCError,
    DEFAULT_CONNECT_TIMEOUT_MS,
    DEFAULT_INVOKE_TIMEOUT_MS,
};
