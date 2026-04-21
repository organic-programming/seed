import { createServer as createHTTPServer } from "node:http";
import { WebSocketServer } from "ws";
import { HolonError } from "./client.mjs";

const DEFAULT_URI = "ws://127.0.0.1:0/rpc";
const DEFAULT_TIMEOUT_MS = 5000;
const DEFAULT_MAX_PAYLOAD_BYTES = 1024 * 1024;
const DEFAULT_SHUTDOWN_GRACE_MS = 10_000;
const WS_OPEN = 1;

const ALLOWED_ENVELOPE_KEYS = new Set(["jsonrpc", "id", "method", "params", "result", "error"]);
const hasOwn = (obj, key) => Object.prototype.hasOwnProperty.call(obj, key);

export class HolonServer {
    #uri;
    #address;
    #maxConnections;
    #nextClientID = 1;
    #nextServerID = 1;
    #handlers = new Map();
    #clients = new Map();
    #activeRequests = 0;
    #drainWaiters = [];
    #waiters = [];
    #starting = null;
    #closing = null;
    #closed = false;
    #httpServer = null;
    #wss = null;
    #maxPayloadBytes;
    #shutdownGraceMs;

    constructor(uri = DEFAULT_URI, options = {}) {
        this.#uri = nonEmptyString(uri, "uri");
        this.#address = this.#uri;

        const maxConnections = options.maxConnections ?? 1;
        if (maxConnections !== Infinity) {
            positiveInt(maxConnections, "maxConnections");
        }
        this.#maxConnections = maxConnections;

        this.#maxPayloadBytes = positiveInt(
            options.maxPayloadBytes ?? DEFAULT_MAX_PAYLOAD_BYTES,
            "maxPayloadBytes",
        );
        this.#shutdownGraceMs = positiveInt(
            options.shutdownGraceMs ?? DEFAULT_SHUTDOWN_GRACE_MS,
            "shutdownGraceMs",
        );
    }

    get address() {
        return this.#address;
    }

    register(method, handler) {
        nonEmptyString(method, "method");
        if (typeof handler !== "function") {
            throw new TypeError("handler must be a function");
        }
        this.#handlers.set(method, handler);
    }

    unregister(method) {
        this.#handlers.delete(method);
    }

    listClients() {
        return Array.from(this.#clients.keys()).map((id) => ({ id }));
    }

    waitForClient(options = {}) {
        const existing = this.listClients().at(0);
        if (existing) {
            return Promise.resolve(existing);
        }

        const timeoutMs = positiveInt(options.timeout ?? DEFAULT_TIMEOUT_MS, "timeout");
        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                const index = this.#waiters.findIndex((waiter) => waiter.resolve === resolve);
                if (index >= 0) {
                    this.#waiters.splice(index, 1);
                }
                reject(new HolonError(4, `timeout waiting for client after ${timeoutMs}ms`));
            }, timeoutMs);

            this.#waiters.push({ resolve, reject, timer });
        });
    }

    async start() {
        if (this.#closed) {
            throw new HolonError(14, "server is closed");
        }
        if (this.#wss) {
            return this.#address;
        }
        if (this.#starting) {
            return this.#starting;
        }

        this.#starting = this.#startInternal();
        try {
            return await this.#starting;
        } finally {
            this.#starting = null;
        }
    }

    async #startInternal() {
        const listen = parseListenURI(this.#uri);

        this.#httpServer = createHTTPServer();
        this.#wss = new WebSocketServer({
            server: this.#httpServer,
            path: listen.path,
            maxPayload: this.#maxPayloadBytes,
            handleProtocols: (protocols) => {
                if (protocols.has("holon-rpc")) {
                    return "holon-rpc";
                }
                return false;
            },
        });

        this.#wss.on("connection", (ws) => {
            this.#handleConnection(ws);
        });

        await new Promise((resolve, reject) => {
            this.#httpServer.once("error", reject);
            this.#httpServer.listen(listen.port, listen.host, () => {
                this.#httpServer.off("error", reject);
                resolve();
            });
        });

        const addr = this.#httpServer.address();
        if (!addr || typeof addr === "string") {
            throw new Error("failed to determine server address");
        }

        const host = normalizePublicHost(listen.host);
        this.#address = `ws://${host}:${addr.port}${listen.path}`;

        return this.#address;
    }

    async close() {
        if (this.#closing) {
            return this.#closing;
        }
        if (this.#closed) {
            return;
        }
        this.#closed = true;
        this.#closing = this.#closeInternal();
        await this.#closing;
        this.#closing = null;
    }

    async #closeInternal() {
        const connectionClosedError = new HolonError(14, "connection closed");

        for (const waiter of this.#waiters) {
            clearTimeout(waiter.timer);
            waiter.reject(new HolonError(14, "server is closed"));
        }
        this.#waiters.length = 0;

        let wssClosed = null;
        if (this.#wss) {
            const wss = this.#wss;
            this.#wss = null;
            wssClosed = new Promise((resolve) => wss.close(() => resolve()));
        }

        await this.#waitForActiveRequests();

        for (const peer of this.#clients.values()) {
            this.#dropPeer(peer, connectionClosedError);
            try {
                peer.ws.close(1001, "server closed");
            } catch {
                // no-op
            }
        }
        this.#clients.clear();

        if (wssClosed) {
            await wssClosed;
        }

        if (this.#httpServer) {
            await new Promise((resolve) => this.#httpServer.close(() => resolve()));
            this.#httpServer = null;
        }
    }

    async #waitForActiveRequests() {
        if (this.#activeRequests === 0) {
            return;
        }

        await Promise.race([
            new Promise((resolve) => {
                this.#drainWaiters.push(resolve);
            }),
            new Promise((resolve) => {
                setTimeout(resolve, this.#shutdownGraceMs);
            }),
        ]);
    }

    invoke(clientOrID, method, params = {}, options = {}) {
        const peer = this.#resolvePeer(clientOrID);
        if (!peer) {
            return Promise.reject(new HolonError(5, `unknown client: ${String(clientOrID)}`));
        }
        if (peer.ws.readyState !== WS_OPEN) {
            return Promise.reject(new HolonError(14, "connection closed"));
        }

        nonEmptyString(method, "method");
        const timeoutMs = positiveInt(options.timeout ?? DEFAULT_TIMEOUT_MS, "timeout");

        const id = `s${this.#nextServerID++}`;
        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                peer.pending.delete(id);
                reject(new HolonError(4, `timeout after ${timeoutMs}ms`));
            }, timeoutMs);

            peer.pending.set(id, { resolve, reject, timer });

            this.#sendJSON(peer.ws, {
                jsonrpc: "2.0",
                id,
                method,
                params,
            }).catch(() => {
                const pending = peer.pending.get(id);
                if (!pending) return;
                clearTimeout(timer);
                peer.pending.delete(id);
                pending.reject(new HolonError(14, "connection closed"));
            });
        });
    }

    #resolvePeer(clientOrID) {
        if (!clientOrID) {
            return null;
        }
        if (typeof clientOrID === "string") {
            return this.#clients.get(clientOrID) ?? null;
        }
        if (typeof clientOrID === "object" && typeof clientOrID.id === "string") {
            return this.#clients.get(clientOrID.id) ?? null;
        }
        return null;
    }

    #handleConnection(ws) {
        if (this.#closed) {
            try {
                ws.close(1001, "server is closing");
            } catch {
                // no-op
            }
            return;
        }

        if (this.#clients.size >= this.#maxConnections) {
            try {
                ws.close(1008, "max connections exceeded");
            } catch {
                // no-op
            }
            return;
        }

        const peer = {
            id: `c${this.#nextClientID++}`,
            ws,
            pending: new Map(),
        };

        this.#clients.set(peer.id, peer);
        this.#resolveClientWaiter(peer);

        ws.on("message", (data, isBinary) => {
            if (isBinary) {
                this.#closeForProtocolViolation(ws, "binary frames are not supported");
                return;
            }
            this.#handleRawMessage(peer, decodeText(data));
        });

        ws.on("close", () => {
            this.#dropPeer(peer, new HolonError(14, "connection closed"));
        });

        ws.on("error", () => {
            this.#dropPeer(peer, new HolonError(14, "connection closed"));
        });
    }

    #resolveClientWaiter(peer) {
        if (this.#waiters.length === 0) {
            return;
        }
        const waiter = this.#waiters.shift();
        clearTimeout(waiter.timer);
        waiter.resolve({ id: peer.id });
    }

    async #handleRawMessage(peer, text) {
        if (text === null) {
            this.#closeForProtocolViolation(peer.ws, "unsupported frame type");
            return;
        }

        let msg;
        try {
            msg = JSON.parse(text);
        } catch {
            this.#closeForProtocolViolation(peer.ws, "malformed JSON");
            return;
        }

        const envelope = validateEnvelope(msg);
        if (!envelope.ok) {
            this.#closeForProtocolViolation(peer.ws, `invalid envelope: ${envelope.reason}`);
            return;
        }

        if (envelope.kind === "request") {
            await this.#handleRequest(peer, envelope.msg);
        } else {
            this.#handleResponse(peer, envelope.msg);
        }
    }

    async #handleRequest(peer, msg) {
        this.#activeRequests += 1;
        try {
            if (msg.method === "rpc.heartbeat") {
                await this.#safeSendJSON(peer.ws, {
                    jsonrpc: "2.0",
                    id: msg.id,
                    result: {},
                });
                return;
            }

            const handler = this.#handlers.get(msg.method);
            if (!handler) {
                await this.#safeSendJSON(peer.ws, {
                    jsonrpc: "2.0",
                    id: msg.id,
                    error: {
                        code: -32601,
                        message: `method \"${msg.method}\" not found`,
                    },
                });
                return;
            }

            try {
                const result = await handler(msg.params ?? {}, { id: peer.id });
                await this.#safeSendJSON(peer.ws, {
                    jsonrpc: "2.0",
                    id: msg.id,
                    result: normalizeResult(result),
                });
            } catch (err) {
                const code = Number.isInteger(err?.code)
                    ? Number(err.code)
                    : err instanceof HolonError
                        ? err.code
                        : 13;
                const message = typeof err?.message === "string" && err.message.trim() !== ""
                    ? err.message
                    : "internal error";

                await this.#safeSendJSON(peer.ws, {
                    jsonrpc: "2.0",
                    id: msg.id,
                    error: { code, message },
                });
            }
        } finally {
            this.#activeRequests = Math.max(0, this.#activeRequests - 1);
            if (this.#activeRequests === 0 && this.#drainWaiters.length > 0) {
                const waiters = this.#drainWaiters.splice(0, this.#drainWaiters.length);
                for (const waiter of waiters) {
                    waiter();
                }
            }
        }
    }

    #handleResponse(peer, msg) {
        const pending = peer.pending.get(msg.id);
        if (!pending) {
            return;
        }

        clearTimeout(pending.timer);
        peer.pending.delete(msg.id);

        if (msg.error) {
            pending.reject(new HolonError(msg.error.code, msg.error.message));
            return;
        }

        pending.resolve(msg.result ?? {});
    }

    #dropPeer(peer, err) {
        if (!this.#clients.has(peer.id)) {
            return;
        }

        this.#clients.delete(peer.id);
        for (const [id, pending] of peer.pending.entries()) {
            clearTimeout(pending.timer);
            pending.reject(err);
            peer.pending.delete(id);
        }
    }

    async #sendJSON(ws, payload) {
        if (ws.readyState !== WS_OPEN) {
            throw new HolonError(14, "connection closed");
        }

        await new Promise((resolve, reject) => {
            ws.send(JSON.stringify(payload), (err) => {
                if (err) {
                    reject(err);
                    return;
                }
                resolve();
            });
        });
    }

    async #safeSendJSON(ws, payload) {
        try {
            await this.#sendJSON(ws, payload);
            return true;
        } catch {
            return false;
        }
    }

    #closeForProtocolViolation(ws, reason) {
        try {
            ws.close(1002, reason.slice(0, 123));
        } catch {
            try {
                ws.close();
            } catch {
                // no-op
            }
        }
    }
}

function parseListenURI(uri) {
    let parsed;
    try {
        parsed = new URL(uri);
    } catch {
        throw new TypeError(`invalid listen URI: ${uri}`);
    }

    if (parsed.protocol !== "ws:") {
        throw new TypeError(`unsupported listen scheme: ${parsed.protocol}`);
    }

    const host = parsed.hostname || "127.0.0.1";
    const port = parsed.port === "" ? 0 : Number(parsed.port);
    if (!Number.isInteger(port) || port < 0 || port > 65535) {
        throw new TypeError("listen URI port must be between 0 and 65535");
    }

    let path = parsed.pathname;
    if (!path || path === "/") {
        path = "/rpc";
    }

    return {
        host,
        port,
        path,
    };
}

function normalizePublicHost(host) {
    if (host === "0.0.0.0" || host === "::") {
        return "127.0.0.1";
    }
    return host;
}

function validateEnvelope(msg) {
    if (!isPlainObject(msg)) {
        return { ok: false, reason: "message must be an object" };
    }

    for (const key of Object.keys(msg)) {
        if (!ALLOWED_ENVELOPE_KEYS.has(key)) {
            return { ok: false, reason: `unknown field: ${key}` };
        }
    }

    if (msg.jsonrpc !== "2.0") {
        return { ok: false, reason: "jsonrpc must be \"2.0\"" };
    }

    if (!hasOwn(msg, "id") || typeof msg.id !== "string" || msg.id.trim() === "") {
        return { ok: false, reason: "id must be a non-empty string" };
    }

    const hasMethod = hasOwn(msg, "method");
    const hasResult = hasOwn(msg, "result");
    const hasError = hasOwn(msg, "error");

    if (hasMethod) {
        if (typeof msg.method !== "string" || msg.method.trim() === "") {
            return { ok: false, reason: "method must be a non-empty string" };
        }
        if (hasResult || hasError) {
            return { ok: false, reason: "request cannot include result or error" };
        }

        return {
            ok: true,
            kind: "request",
            msg: {
                id: msg.id,
                method: msg.method,
                params: hasOwn(msg, "params") ? msg.params : {},
            },
        };
    }

    if (hasOwn(msg, "params")) {
        return { ok: false, reason: "response cannot include params" };
    }

    if (hasResult === hasError) {
        return { ok: false, reason: "response must include exactly one of result or error" };
    }

    if (hasError) {
        if (!isPlainObject(msg.error)) {
            return { ok: false, reason: "error must be an object" };
        }
        if (!Number.isInteger(msg.error.code)) {
            return { ok: false, reason: "error.code must be an integer" };
        }
        if (typeof msg.error.message !== "string") {
            return { ok: false, reason: "error.message must be a string" };
        }
    }

    return {
        ok: true,
        kind: "response",
        msg,
    };
}

function decodeText(data) {
    if (typeof data === "string") {
        return data;
    }
    if (typeof Buffer !== "undefined" && Buffer.isBuffer(data)) {
        return data.toString("utf8");
    }
    if (data instanceof ArrayBuffer) {
        return new TextDecoder().decode(new Uint8Array(data));
    }
    if (ArrayBuffer.isView(data)) {
        return new TextDecoder().decode(new Uint8Array(data.buffer, data.byteOffset, data.byteLength));
    }
    return null;
}

function normalizeResult(result) {
    if (typeof result === "undefined") {
        return {};
    }
    return result;
}

function positiveInt(value, name) {
    if (!Number.isInteger(value) || value <= 0) {
        throw new TypeError(`${name} must be a positive integer`);
    }
    return value;
}

function nonEmptyString(value, name) {
    if (typeof value !== "string" || value.trim() === "") {
        throw new TypeError(`${name} must be a non-empty string`);
    }
    return value;
}

function isPlainObject(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
