/**
 * Browser environments cannot spawn processes or scan the filesystem.
 * connect() in js-web-holons only supports explicit dial URIs.
 *
 * Supported transports:
 *   - ws:// and wss:// for bidirectional WebSocket JSON-RPC
 *   - http:// and https:// for unary HTTP POST and SSE streaming
 */

import { HolonClient, HolonError } from "./client.mjs";

const DEFAULT_HTTP_TIMEOUT_MS = 30000;
const DEFAULT_RPC_PATH = "/api/v1/rpc";
const DEFAULT_ERROR_CODE = 13;

export class HolonHTTPClient {
    #baseUrl;
    #fetch;
    #closed = false;
    #controllers = new Set();
    #nextID = 1;
    #defaultTimeoutMs;

    constructor(baseUrl, options = {}) {
        this.#baseUrl = normalizeBaseUrl(baseUrl);
        this.#fetch = options.fetch ?? defaultFetch();
        if (typeof this.#fetch !== "function") {
            throw new Error("fetch implementation required");
        }
        this.#defaultTimeoutMs = positiveInt(
            options.defaultTimeout ?? DEFAULT_HTTP_TIMEOUT_MS,
            "defaultTimeout",
        );
    }

    get baseUrl() {
        return this.#baseUrl;
    }

    async invoke(method, params = {}, options = {}) {
        this.#assertOpen();
        validateMethod(method);

        const response = await this.#fetchWithTimeout(
            buildMethodURL(this.#baseUrl, method),
            {
                method: "POST",
                headers: buildHeaders(options.headers, {
                    accept: "application/json",
                    "content-type": "application/json",
                }),
                body: JSON.stringify(normalizeParams(params)),
            },
            options,
        );

        return decodeUnaryResponse(response);
    }

    async *stream(method, params = {}, options = {}) {
        this.#assertOpen();
        validateMethod(method);

        const response = await this.#fetchWithTimeout(
            buildMethodURL(this.#baseUrl, method),
            {
                method: "POST",
                headers: buildHeaders(options.headers, {
                    accept: "text/event-stream",
                    "content-type": "application/json",
                }),
                body: JSON.stringify(normalizeParams(params)),
            },
            options,
        );

        yield* decodeSSEStream(response);
    }

    async *streamQuery(method, params = {}, options = {}) {
        this.#assertOpen();
        validateMethod(method);

        const response = await this.#fetchWithTimeout(
            buildQueryMethodURL(this.#baseUrl, method, params),
            {
                method: "GET",
                headers: buildHeaders(options.headers, {
                    accept: "text/event-stream",
                }),
            },
            options,
        );

        yield* decodeSSEStream(response);
    }

    close() {
        if (this.#closed) {
            return;
        }
        this.#closed = true;
        for (const controller of this.#controllers) {
            controller.abort();
        }
        this.#controllers.clear();
    }

    #assertOpen() {
        if (this.#closed) {
            throw new HolonError(14, "client is closed");
        }
    }

    async #fetchWithTimeout(url, init, options = {}) {
        const timeoutMs = positiveInt(options.timeout ?? this.#defaultTimeoutMs, "timeout");
        const controller = new AbortController();
        const cleanup = linkAbortSignal(options.signal, controller);
        let timedOut = false;

        const timer = setTimeout(() => {
            timedOut = true;
            controller.abort();
        }, timeoutMs);

        this.#controllers.add(controller);

        try {
            return await this.#fetch(url, {
                ...init,
                signal: controller.signal,
            });
        } catch (err) {
            if (timedOut) {
                throw new HolonError(4, `timeout after ${timeoutMs}ms`);
            }
            if (controller.signal.aborted) {
                throw new HolonError(1, "request aborted");
            }
            throw err;
        } finally {
            clearTimeout(timer);
            cleanup();
            this.#controllers.delete(controller);
        }
    }
}

export function connect(target, options = {}) {
    const normalized = normalizeTarget(target);
    if (normalized.kind === "websocket") {
        return new HolonClient(normalized.url, options);
    }
    return new HolonHTTPClient(normalized.baseUrl, options);
}

export function disconnect(client) {
    if (!client) {
        return;
    }
    if (typeof client.close === "function") {
        client.close();
    }
}

function defaultFetch() {
    if (typeof globalThis.fetch !== "function") {
        return undefined;
    }
    return globalThis.fetch.bind(globalThis);
}

function normalizeTarget(target) {
    const value = String(target || "").trim();
    if (!value) {
        throw new Error("target is required");
    }

    if (/^wss?:\/\//i.test(value)) {
        return {
            kind: "websocket",
            url: normalizeSocketUrl(value),
        };
    }

    if (/^https?:\/\//i.test(value)) {
        return {
            kind: "http",
            baseUrl: normalizeBaseUrl(value),
        };
    }

    if (value.includes("://")) {
        throw new Error(`unsupported browser connect target: ${value}`);
    }

    if (looksLikeHostPort(value)) {
        throw new Error(
            `browser connect() requires an explicit ws://, wss://, http://, or https:// URI: ${JSON.stringify(value)}`,
        );
    }

    throw new Error(
        `browser connect() only supports direct transport URIs; slug resolution is unavailable in js-web-holons. Use js-holons in Node.js for slug-based resolution: ${JSON.stringify(value)}`,
    );
}

function normalizeBaseUrl(value) {
    const url = parseURL(value, "invalid Holon-RPC target");
    if (url.protocol !== "http:" && url.protocol !== "https:") {
        throw new Error(`Holon-RPC HTTP targets must use http:// or https://: ${value}`);
    }
    if (!url.hostname) {
        throw new Error(`invalid Holon-RPC target: ${value}`);
    }

    applyDefaultRPCPath(url);
    url.hash = "";
    return stripTrailingSlash(url.toString());
}

function normalizeSocketUrl(value) {
    const url = parseURL(value, "invalid Holon-RPC target");
    if (url.protocol !== "ws:" && url.protocol !== "wss:") {
        throw new Error(`Holon-RPC WebSocket targets must use ws:// or wss://: ${value}`);
    }
    if (!url.hostname) {
        throw new Error(`invalid Holon-RPC target: ${value}`);
    }

    applyDefaultRPCPath(url);
    url.hash = "";
    return url.toString();
}

function parseURL(value, label) {
    try {
        return new URL(String(value));
    } catch {
        throw new Error(`${label}: ${value}`);
    }
}

function applyDefaultRPCPath(url) {
    if (url.pathname === "/" || url.pathname === "") {
        url.pathname = DEFAULT_RPC_PATH;
    }
}

function stripTrailingSlash(value) {
    if (value.endsWith("/")) {
        return value.slice(0, -1);
    }
    return value;
}

function looksLikeHostPort(value) {
    return /^[^/\s:?#]+:\d+$/.test(value) || /^\[[^\]]+\]:\d+$/.test(value);
}

function validateMethod(method) {
    if (typeof method !== "string" || method.trim() === "") {
        throw new TypeError("method must be a non-empty string");
    }
}

function normalizeParams(params) {
    if (params == null) {
        return {};
    }
    if (!isPlainObject(params)) {
        throw new TypeError("params must be an object");
    }
    return params;
}

function buildMethodURL(baseUrl, method) {
    const normalizedMethod = String(method).trim().replace(/^\/+/, "");
    return `${stripTrailingSlash(baseUrl)}/${normalizedMethod}`;
}

function buildQueryMethodURL(baseUrl, method, params) {
    const url = new URL(buildMethodURL(baseUrl, method));
    const normalizedParams = normalizeParams(params);
    for (const [key, value] of Object.entries(normalizedParams)) {
        appendQueryValues(url.searchParams, key, value);
    }
    return url.toString();
}

function appendQueryValues(searchParams, key, value) {
    if (Array.isArray(value)) {
        for (const item of value) {
            searchParams.append(key, stringifyQueryValue(item));
        }
        return;
    }
    searchParams.append(key, stringifyQueryValue(value));
}

function stringifyQueryValue(value) {
    if (value == null) {
        return "";
    }
    if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
        return String(value);
    }
    return JSON.stringify(value);
}

function buildHeaders(extraHeaders, baseHeaders) {
    const headers = new Headers();
    for (const [key, value] of Object.entries(baseHeaders)) {
        headers.set(key, value);
    }
    if (extraHeaders == null) {
        return headers;
    }
    for (const [key, value] of headerEntries(extraHeaders)) {
        headers.set(key, value);
    }
    return headers;
}

function headerEntries(headers) {
    if (typeof headers.entries === "function") {
        return Array.from(headers.entries(), ([key, value]) => [String(key), String(value)]);
    }
    return Object.entries(headers).map(([key, value]) => [String(key), String(value)]);
}

function linkAbortSignal(sourceSignal, controller) {
    if (!sourceSignal) {
        return () => {};
    }
    const abort = () => controller.abort();
    if (sourceSignal.aborted) {
        abort();
        return () => {};
    }
    sourceSignal.addEventListener("abort", abort, { once: true });
    return () => {
        sourceSignal.removeEventListener("abort", abort);
    };
}

async function decodeUnaryResponse(response) {
    const payload = await readJSONBody(response);
    if (response.status >= 400) {
        throw responseErrorFromHTTP(payload, response.status, response.statusText);
    }

    if (isRPCMessage(payload)) {
        if (payload.error) {
            throw rpcErrorFromEnvelope(payload.error);
        }
        return payload.result ?? {};
    }

    return payload ?? {};
}

async function *decodeSSEStream(response) {
    if (response.status >= 400) {
        const payload = await readJSONBody(response);
        throw responseErrorFromHTTP(payload, response.status, response.statusText);
    }

    if (!response.body || typeof response.body.getReader !== "function") {
        throw new Error("SSE response body is not readable");
    }

    for await (const rawEvent of readSSEBlocks(response.body)) {
        const event = decodeSSEEvent(rawEvent);
        yield event;
        if (event.event === "done" || event.error) {
            return;
        }
    }
}

async function *readSSEBlocks(body) {
    const reader = body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    try {
        for (;;) {
            const { value, done } = await reader.read();
            if (done) {
                break;
            }
            buffer += decoder.decode(value, { stream: true });
            buffer = buffer.replace(/\r\n/g, "\n");

            let separator = buffer.indexOf("\n\n");
            while (separator >= 0) {
                const block = buffer.slice(0, separator);
                buffer = buffer.slice(separator + 2);
                if (block.trim() !== "") {
                    yield parseSSEBlock(block);
                }
                separator = buffer.indexOf("\n\n");
            }
        }

        buffer += decoder.decode();
        buffer = buffer.replace(/\r\n/g, "\n");
        if (buffer.trim() !== "") {
            yield parseSSEBlock(buffer);
        }
    } finally {
        reader.releaseLock();
    }
}

function parseSSEBlock(block) {
    const event = {
        event: "",
        id: "",
        data: [],
    };

    for (const line of String(block).split("\n")) {
        if (!line || line.startsWith(":")) {
            continue;
        }

        if (line.startsWith("event:")) {
            event.event = line.slice("event:".length).trim();
            continue;
        }
        if (line.startsWith("id:")) {
            event.id = line.slice("id:".length).trim();
            continue;
        }
        if (line.startsWith("data:")) {
            event.data.push(line.slice("data:".length).trimStart());
        }
    }

    return {
        event: event.event,
        id: event.id,
        data: event.data.join("\n"),
    };
}

function decodeSSEEvent(rawEvent) {
    const event = {
        event: rawEvent.event,
        id: rawEvent.id,
        result: null,
        error: null,
    };

    if (rawEvent.event === "done") {
        return event;
    }

    if (!rawEvent.data) {
        return event;
    }

    let payload;
    try {
        payload = JSON.parse(rawEvent.data);
    } catch (err) {
        throw new Error(`invalid SSE payload: ${err.message}`);
    }

    if (!isRPCMessage(payload)) {
        event.result = payload;
        return event;
    }

    if (payload.error) {
        event.error = rpcErrorFromEnvelope(payload.error);
        return event;
    }

    event.result = payload.result ?? {};
    return event;
}

async function readJSONBody(response) {
    const text = await response.text();
    if (text.trim() === "") {
        return null;
    }
    try {
        return JSON.parse(text);
    } catch (err) {
        throw new Error(`invalid JSON response: ${err.message}`);
    }
}

function isRPCMessage(value) {
    return isPlainObject(value)
        && (value.jsonrpc === "2.0" || Object.prototype.hasOwnProperty.call(value, "result") || Object.prototype.hasOwnProperty.call(value, "error"));
}

function responseErrorFromHTTP(payload, status, statusText) {
    if (isRPCMessage(payload) && payload.error) {
        return rpcErrorFromEnvelope(payload.error);
    }
    return new HolonError(statusToErrorCode(status), `HTTP ${status}: ${statusText || "request failed"}`);
}

function rpcErrorFromEnvelope(error) {
    const code = Number.isInteger(error?.code) ? error.code : DEFAULT_ERROR_CODE;
    const message = typeof error?.message === "string" && error.message.trim() !== ""
        ? error.message
        : "request failed";
    return new HolonError(code, message, error?.data);
}

function statusToErrorCode(status) {
    if (status === 400) {
        return -32600;
    }
    if (status === 404) {
        return 5;
    }
    return DEFAULT_ERROR_CODE;
}

function positiveInt(value, name) {
    if (!Number.isInteger(value) || value <= 0) {
        throw new TypeError(`${name} must be a positive integer`);
    }
    return value;
}

function isPlainObject(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
