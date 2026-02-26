/**
 * js-web-holons — Bidirectional browser SDK for Organic Programming.
 *
 * Both the browser and the Go server can initiate RPC calls:
 *   - Browser → Go:  client.invoke("Service/Method", payload)
 *   - Go → Browser:  client.register("Service/Method", handler)
 *
 * Wire protocol (symmetric — either direction):
 *   Request:  { "jsonrpc":"2.0", "id":"1", "method":"Service/Method", "params": {...} }
 *   Response: { "jsonrpc":"2.0", "id":"1", "result": {...} }
 *   Error:    { "jsonrpc":"2.0", "id":"1", "error": { "code": 5, "message": "..." } }
 *
 * A message is a request if it has "method".
 * A message is a response if it has "result" or "error".
 *
 * @module js-web-holons
 */

const DEFAULTS = Object.freeze({
    defaultTimeoutMs: 30000,
    connectTimeoutMs: 10000,
    maxPendingRequests: 256,
    maxTrackedResponseIds: 1024,
    reconnect: {
        enabled: true,
        minDelayMs: 200,
        maxDelayMs: 5000,
        factor: 2,
        jitter: 0.2,
    },
    heartbeat: {
        enabled: true,
        intervalMs: 15000,
        timeoutMs: 5000,
        method: "rpc.heartbeat",
        params: {},
    },
});

const ALLOWED_ENVELOPE_KEYS = new Set(["jsonrpc", "id", "method", "params", "result", "error"]);
const hasOwn = (obj, key) => Object.prototype.hasOwnProperty.call(obj, key);

/**
 * Error returned when the remote side responds with an error envelope.
 */
export class HolonError extends Error {
    /**
     * @param {number} code - gRPC-style status code
     * @param {string} message - human-readable message
     */
    constructor(code, message) {
        super(message);
        this.name = "HolonError";
        this.code = code;
    }
}

/**
 * Bidirectional WebSocket client for holon RPC.
 *
 * The client manages a single WebSocket connection, multiplexing
 * concurrent requests via unique message IDs. Both sides can
 * initiate calls.
 */
export class HolonClient {
    #ws = null;
    #url;
    #WS;
    #nextId = 1;
    #nextHeartbeatId = 1;
    #pending = new Map();  // id → { kind, resolve, reject, timer }
    #pendingInvokeCount = 0;
    #pendingInvokeReservations = 0;
    #handlers = new Map(); // method → async handler(params) → result
    #connected = null;
    #closed = false;
    #config;
    #onProtocolWarning;

    // Track response IDs to classify unknown vs duplicate vs stale replies.
    #settledResponseIds = new Set();
    #settledResponseOrder = [];
    #timedOutResponseIds = new Set();
    #timedOutResponseOrder = [];

    #reconnectAttempt = 0;
    #reconnectTimer = null;
    #heartbeatTimer = null;
    #heartbeatInFlightId = null;

    /**
     * @param {string} url - WebSocket URL, e.g. "ws://localhost:8080/ws"
     * @param {Object} [options]
     * @param {Function} [options.WebSocket] - WebSocket constructor override (for Node.js / testing)
     * @param {number} [options.defaultTimeout=30000] - default invoke timeout in ms
     * @param {number} [options.connectTimeout=10000] - connection timeout in ms
     * @param {number} [options.maxPendingRequests=256] - max in-flight invoke() calls
     * @param {number} [options.maxTrackedResponseIds=1024] - bounded cache for duplicate/stale detection
     * @param {boolean|Object} [options.reconnect] - reconnect config
     * @param {boolean|Object} [options.heartbeat] - heartbeat config
     * @param {Function} [options.onProtocolWarning] - callback for protocol anomalies
     */
    constructor(url, options = {}) {
        this.#url = url;
        this.#WS = options.WebSocket ?? globalThis.WebSocket;
        if (!this.#WS) {
            throw new Error("WebSocket implementation required");
        }

        const normalized = normalizeOptions(options);
        this.#config = normalized.config;
        this.#onProtocolWarning = normalized.onProtocolWarning;
    }

    /**
     * Register a browser-side handler for Go→Browser calls.
     * When the server invokes this method, the handler is called and
     * the result is sent back automatically.
     *
     * @param {string} method - full method path, e.g. "ui.v1.UIService/GetViewport"
     * @param {Function} handler - async (payload) => result
     */
    register(method, handler) {
        this.#handlers.set(method, handler);
    }

    /**
     * Establishes the WebSocket connection. Called automatically on
     * first invoke(), but can be called explicitly for eager connection.
     * @returns {Promise<void>}
     */
    connect() {
        if (this.#closed) {
            return Promise.reject(new HolonError(14, "client is closed"));
        }
        if (this.#isSocketOpen()) {
            return Promise.resolve();
        }
        if (this.#connected) {
            return this.#connected;
        }

        this.#connected = this.#openSocket();
        return this.#connected;
    }

    /**
     * Invoke a method on the Go server.
     *
     * @param {string} method - full method path, e.g. "hello.v1.HelloService/Greet"
     * @param {Object} [payload={}] - JSON-serializable request payload
     * @param {Object} [options] - invocation options
     * @param {number} [options.timeout] - timeout in ms (defaults to constructor setting)
     * @returns {Promise<Object>} the result payload
     * @throws {HolonError} on server errors, timeout, or transport issues
     */
    async invoke(method, payload = {}, options = {}) {
        if (this.#closed) throw new HolonError(14, "client is closed");
        if (typeof method !== "string" || method.trim() === "") {
            throw new HolonError(3, "method must be a non-empty string");
        }

        const timeout = positiveInt(options.timeout ?? this.#config.defaultTimeoutMs, "timeout");
        if ((this.#pendingInvokeCount + this.#pendingInvokeReservations) >= this.#config.maxPendingRequests) {
            throw new HolonError(8, `max pending requests exceeded (${this.#config.maxPendingRequests})`);
        }
        this.#pendingInvokeReservations += 1;

        let reserved = true;
        const releaseReservation = () => {
            if (!reserved) return;
            reserved = false;
            this.#pendingInvokeReservations = Math.max(0, this.#pendingInvokeReservations - 1);
        };

        try {
            await this.connect();
        } catch (err) {
            releaseReservation();
            throw err;
        }

        if (!this.#isSocketOpen()) {
            releaseReservation();
            throw new HolonError(14, "connection closed");
        }

        const ws = this.#ws;
        const id = String(this.#nextId++);

        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                const pending = this.#deletePending(id, "timeout");
                if (!pending) return;
                pending.reject(new HolonError(4, `timeout after ${timeout}ms`));
            }, timeout);

            this.#setPending(id, {
                kind: "invoke",
                resolve,
                reject,
                timer,
            });
            releaseReservation();

            try {
                ws.send(JSON.stringify({
                    jsonrpc: "2.0",
                    id,
                    method,
                    params: payload,
                }));
            } catch {
                const pending = this.#deletePending(id, "send_error");
                if (!pending) return;
                pending.reject(new HolonError(14, "connection closed"));
            }
        });
    }

    /**
     * Close the WebSocket connection gracefully.
     */
    close() {
        if (this.#closed) return;
        this.#closed = true;

        this.#clearReconnectTimer();
        this.#stopHeartbeat();
        this.#failAllPending(new HolonError(14, "client is closed"));

        this.#connected = null;
        if (this.#ws) {
            try {
                this.#ws.close(1000, "client close");
            } catch {
                // no-op
            }
            this.#ws = null;
        }
    }

    /**
     * @returns {Promise<void>}
     */
    #openSocket() {
        return new Promise((resolve, reject) => {
            const ws = new this.#WS(this.#url, "holon-rpc");
            let opened = false;
            let settled = false;

            const connectTimer = setTimeout(() => {
                if (settled) return;
                settled = true;
                try {
                    ws.close(1000, "connect timeout");
                } catch {
                    // no-op
                }
                this.#connected = null;
                if (!this.#closed && this.#config.reconnect.enabled) {
                    this.#scheduleReconnect();
                }
                reject(new Error(`WebSocket connection timeout: ${this.#url}`));
            }, this.#config.connectTimeoutMs);

            ws.onopen = () => {
                if (settled) return;
                opened = true;
                settled = true;
                clearTimeout(connectTimer);

                this.#ws = ws;
                this.#clearReconnectTimer();
                this.#reconnectAttempt = 0;
                this.#startHeartbeat();
                resolve();
            };

            ws.onerror = () => {
                if (!opened && !settled) {
                    settled = true;
                    clearTimeout(connectTimer);
                    this.#connected = null;
                    if (!this.#closed && this.#config.reconnect.enabled) {
                        this.#scheduleReconnect();
                    }
                    reject(new Error(`WebSocket connection failed: ${this.#url}`));
                }
            };

            ws.onclose = () => {
                if (this.#ws === ws) {
                    this.#ws = null;
                }

                this.#stopHeartbeat();
                this.#connected = null;
                this.#failAllPending(new HolonError(14, "connection closed"));

                if (!opened && !settled) {
                    settled = true;
                    clearTimeout(connectTimer);
                    reject(new Error(`WebSocket connection closed before open: ${this.#url}`));
                }

                if (!this.#closed && this.#config.reconnect.enabled) {
                    this.#scheduleReconnect();
                }
            };

            ws.onmessage = (event) => {
                this.#handleMessage(event.data);
            };
        });
    }

    /**
     * @param {string|Buffer|ArrayBuffer|Uint8Array} data - raw message from WebSocket
     */
    #handleMessage(data) {
        const text = decodeMessageData(data);
        if (text === null) {
            this.#emitProtocolWarning("unsupported_frame_type", {
                frameType: typeof data,
            });
            this.#closeForProtocolViolation("unsupported frame type");
            return;
        }

        let msg;
        try {
            msg = JSON.parse(text);
        } catch {
            this.#emitProtocolWarning("malformed_json", {
                raw: text.slice(0, 256),
            });
            this.#closeForProtocolViolation("malformed JSON");
            return;
        }

        const envelope = validateEnvelope(msg);
        if (!envelope.ok) {
            this.#emitProtocolWarning("invalid_envelope", {
                reason: envelope.reason,
            });
            this.#closeForProtocolViolation(`invalid envelope: ${envelope.reason}`);
            return;
        }

        if (envelope.kind === "request") {
            this.#handleRequest(envelope.msg);
        } else {
            this.#handleResponse(envelope.msg);
        }
    }

    /**
     * Dispatch a server-initiated request to a registered handler.
     */
    async #handleRequest(msg) {
        const ws = this.#ws;
        if (!ws || !this.#isSocketOpen()) return;

        const handler = this.#handlers.get(msg.method);

        if (!handler) {
            ws.send(JSON.stringify({
                jsonrpc: "2.0",
                id: msg.id,
                error: { code: 12, message: `method "${msg.method}" not registered` },
            }));
            return;
        }

        try {
            const result = await handler(msg.params ?? {});
            ws.send(JSON.stringify({ jsonrpc: "2.0", id: msg.id, result }));
        } catch (err) {
            const code = err instanceof HolonError ? err.code : 13;
            const message = typeof err?.message === "string" ? err.message : "internal error";
            ws.send(JSON.stringify({
                jsonrpc: "2.0",
                id: msg.id,
                error: { code, message },
            }));
        }
    }

    /**
     * Route a response to the pending invoke() promise.
     */
    #handleResponse(msg) {
        const pending = this.#deletePending(msg.id, "settled");
        if (!pending) {
            if (this.#timedOutResponseIds.has(msg.id)) {
                this.#emitProtocolWarning("stale_response_id", { id: msg.id });
            } else if (this.#settledResponseIds.has(msg.id)) {
                this.#emitProtocolWarning("duplicate_response_id", { id: msg.id });
            } else {
                this.#emitProtocolWarning("unknown_response_id", { id: msg.id });
            }
            return;
        }

        this.#rememberSettledResponseId(msg.id);

        if (pending.kind === "heartbeat") {
            this.#heartbeatInFlightId = null;
            pending.resolve(null);
            return;
        }

        if (msg.error) {
            pending.reject(new HolonError(msg.error.code, msg.error.message));
            return;
        }

        pending.resolve(msg.result);
    }

    #isSocketOpen() {
        return this.#ws && this.#ws.readyState === this.#WS.OPEN;
    }

    #setPending(id, pending) {
        this.#pending.set(id, pending);
        if (pending.kind === "invoke") {
            this.#pendingInvokeCount += 1;
        }
    }

    #deletePending(id, reason) {
        const pending = this.#pending.get(id);
        if (!pending) return null;

        this.#pending.delete(id);
        clearTimeout(pending.timer);

        if (pending.kind === "invoke") {
            this.#pendingInvokeCount = Math.max(0, this.#pendingInvokeCount - 1);
        }
        if (pending.kind === "heartbeat" && this.#heartbeatInFlightId === id) {
            this.#heartbeatInFlightId = null;
        }
        if (reason === "timeout") {
            this.#rememberTimedOutResponseId(id);
        }

        return pending;
    }

    #failAllPending(err) {
        for (const [id, pending] of this.#pending.entries()) {
            clearTimeout(pending.timer);
            this.#pending.delete(id);
            if (pending.kind === "invoke") {
                this.#pendingInvokeCount = Math.max(0, this.#pendingInvokeCount - 1);
            }
            if (pending.kind === "heartbeat" && this.#heartbeatInFlightId === id) {
                this.#heartbeatInFlightId = null;
            }
            pending.reject(err);
        }
    }

    #rememberSettledResponseId(id) {
        rememberBoundedId(
            this.#settledResponseIds,
            this.#settledResponseOrder,
            id,
            this.#config.maxTrackedResponseIds,
        );
    }

    #rememberTimedOutResponseId(id) {
        rememberBoundedId(
            this.#timedOutResponseIds,
            this.#timedOutResponseOrder,
            id,
            this.#config.maxTrackedResponseIds,
        );
    }

    #scheduleReconnect() {
        if (this.#reconnectTimer || this.#closed || !this.#config.reconnect.enabled) {
            return;
        }

        this.#reconnectAttempt += 1;
        const delayMs = computeReconnectDelay(this.#config.reconnect, this.#reconnectAttempt);

        this.#emitProtocolWarning("reconnect_scheduled", {
            attempt: this.#reconnectAttempt,
            delayMs,
        });

        this.#reconnectTimer = setTimeout(() => {
            this.#reconnectTimer = null;
            if (this.#closed || this.#isSocketOpen()) return;

            this.connect().catch(() => {
                if (!this.#closed) {
                    this.#scheduleReconnect();
                }
            });
        }, delayMs);
    }

    #clearReconnectTimer() {
        if (this.#reconnectTimer) {
            clearTimeout(this.#reconnectTimer);
            this.#reconnectTimer = null;
        }
    }

    #startHeartbeat() {
        this.#stopHeartbeat();

        if (!this.#config.heartbeat.enabled || this.#config.heartbeat.intervalMs <= 0) {
            return;
        }

        this.#heartbeatTimer = setInterval(() => {
            this.#heartbeatTick();
        }, this.#config.heartbeat.intervalMs);

        if (typeof this.#heartbeatTimer.unref === "function") {
            this.#heartbeatTimer.unref();
        }
    }

    #stopHeartbeat() {
        if (this.#heartbeatTimer) {
            clearInterval(this.#heartbeatTimer);
            this.#heartbeatTimer = null;
        }
        this.#heartbeatInFlightId = null;
    }

    #heartbeatTick() {
        if (this.#closed || !this.#isSocketOpen()) {
            return;
        }
        if (this.#heartbeatInFlightId !== null) {
            return;
        }

        const ws = this.#ws;
        const id = `h${this.#nextHeartbeatId++}`;
        const timeout = this.#config.heartbeat.timeoutMs;

        const timer = setTimeout(() => {
            const pending = this.#deletePending(id, "timeout");
            if (!pending) return;

            this.#emitProtocolWarning("heartbeat_timeout", {
                id,
                timeoutMs: timeout,
            });
            pending.reject(new HolonError(4, `heartbeat timeout after ${timeout}ms`));
            this.#closeSocket(4000, "heartbeat timeout");
        }, timeout);

        this.#heartbeatInFlightId = id;

        this.#setPending(id, {
            kind: "heartbeat",
            resolve: () => { clearTimeout(timer); },
            reject: () => { clearTimeout(timer); },
            timer,
        });

        try {
            ws.send(JSON.stringify({
                jsonrpc: "2.0",
                id,
                method: this.#config.heartbeat.method,
                params: this.#config.heartbeat.params,
            }));
        } catch {
            const pending = this.#deletePending(id, "send_error");
            if (pending) {
                pending.reject(new HolonError(14, "connection closed"));
            }
        }
    }

    #closeForProtocolViolation(reason) {
        this.#closeSocket(1002, reason);
    }

    #closeSocket(code, reason) {
        if (!this.#ws) return;
        try {
            this.#ws.close(code, reason.slice(0, 123));
        } catch {
            try {
                this.#ws.close();
            } catch {
                // no-op
            }
        }
    }

    #emitProtocolWarning(type, detail = {}) {
        if (typeof this.#onProtocolWarning !== "function") {
            return;
        }
        try {
            this.#onProtocolWarning({ type, ...detail });
        } catch {
            // no-op
        }
    }
}

function normalizeOptions(options) {
    const reconnectInput = options.reconnect;
    const reconnectEnabled = typeof reconnectInput === "boolean"
        ? reconnectInput
        : reconnectInput?.enabled ?? DEFAULTS.reconnect.enabled;

    const reconnectObj = typeof reconnectInput === "object" && reconnectInput !== null
        ? reconnectInput
        : {};

    const heartbeatInput = options.heartbeat;
    const heartbeatEnabled = typeof heartbeatInput === "boolean"
        ? heartbeatInput
        : heartbeatInput?.enabled ?? DEFAULTS.heartbeat.enabled;

    const heartbeatObj = typeof heartbeatInput === "object" && heartbeatInput !== null
        ? heartbeatInput
        : {};

    return {
        config: {
            defaultTimeoutMs: positiveInt(options.defaultTimeout ?? DEFAULTS.defaultTimeoutMs, "defaultTimeout"),
            connectTimeoutMs: positiveInt(options.connectTimeout ?? DEFAULTS.connectTimeoutMs, "connectTimeout"),
            maxPendingRequests: positiveInt(
                options.maxPendingRequests ?? DEFAULTS.maxPendingRequests,
                "maxPendingRequests",
            ),
            maxTrackedResponseIds: positiveInt(
                options.maxTrackedResponseIds ?? DEFAULTS.maxTrackedResponseIds,
                "maxTrackedResponseIds",
            ),
            reconnect: {
                enabled: Boolean(reconnectEnabled),
                minDelayMs: positiveInt(
                    reconnectObj.minDelay ?? DEFAULTS.reconnect.minDelayMs,
                    "reconnect.minDelay",
                ),
                maxDelayMs: positiveInt(
                    reconnectObj.maxDelay ?? DEFAULTS.reconnect.maxDelayMs,
                    "reconnect.maxDelay",
                ),
                factor: positiveNumber(
                    reconnectObj.factor ?? DEFAULTS.reconnect.factor,
                    "reconnect.factor",
                ),
                jitter: boundedNumber(
                    reconnectObj.jitter ?? DEFAULTS.reconnect.jitter,
                    "reconnect.jitter",
                    0,
                    1,
                ),
            },
            heartbeat: {
                enabled: Boolean(heartbeatEnabled),
                intervalMs: positiveInt(
                    heartbeatObj.interval ?? DEFAULTS.heartbeat.intervalMs,
                    "heartbeat.interval",
                ),
                timeoutMs: positiveInt(
                    heartbeatObj.timeout ?? DEFAULTS.heartbeat.timeoutMs,
                    "heartbeat.timeout",
                ),
                method: nonEmptyString(
                    heartbeatObj.method ?? DEFAULTS.heartbeat.method,
                    "heartbeat.method",
                ),
                params: heartbeatObj.params ?? DEFAULTS.heartbeat.params,
            },
        },
        onProtocolWarning: typeof options.onProtocolWarning === "function"
            ? options.onProtocolWarning
            : null,
    };
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

function decodeMessageData(data) {
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

function rememberBoundedId(set, order, id, maxSize) {
    if (set.has(id)) {
        return;
    }
    set.add(id);
    order.push(id);

    while (order.length > maxSize) {
        const oldest = order.shift();
        set.delete(oldest);
    }
}

function computeReconnectDelay(reconnect, attempt) {
    const expDelay = reconnect.minDelayMs * (reconnect.factor ** Math.max(0, attempt - 1));
    const capped = Math.min(expDelay, reconnect.maxDelayMs);

    if (reconnect.jitter === 0) {
        return Math.round(capped);
    }

    const random = (Math.random() * 2) - 1; // [-1, +1]
    const jittered = capped * (1 + (random * reconnect.jitter));
    return Math.max(1, Math.round(jittered));
}

function isPlainObject(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}

function positiveInt(value, name) {
    if (!Number.isInteger(value) || value <= 0) {
        throw new TypeError(`${name} must be a positive integer`);
    }
    return value;
}

function positiveNumber(value, name) {
    if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
        throw new TypeError(`${name} must be a positive number`);
    }
    return value;
}

function boundedNumber(value, name, min, max) {
    if (typeof value !== "number" || !Number.isFinite(value) || value < min || value > max) {
        throw new TypeError(`${name} must be between ${min} and ${max}`);
    }
    return value;
}

function nonEmptyString(value, name) {
    if (typeof value !== "string" || value.trim() === "") {
        throw new TypeError(`${name} must be a non-empty string`);
    }
    return value;
}
