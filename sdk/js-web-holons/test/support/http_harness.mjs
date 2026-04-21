import { createServer } from "node:http";

const DEFAULT_BASE_PATH = "/api/v1/rpc";

export async function startHTTPHolonRPCServer(options = {}) {
    const basePath = normalizeBasePath(options.basePath ?? DEFAULT_BASE_PATH);
    const handlers = options.handlers ?? {};
    const streamHandlers = options.streamHandlers ?? {};
    const requests = [];

    const server = createServer(async (req, res) => {
        try {
            const requestURL = new URL(req.url, "http://127.0.0.1");
            const method = extractMethod(basePath, requestURL.pathname);
            if (!method) {
                writeRPCError(res, 404, "h0", { code: 5, message: "method not found" });
                return;
            }

            if (acceptsSSE(req)) {
                if (req.method !== "POST" && req.method !== "GET") {
                    res.writeHead(405);
                    res.end();
                    return;
                }

                const params = req.method === "GET"
                    ? queryParams(requestURL.searchParams)
                    : await readJSONBody(req);
                requests.push({ transport: "sse", httpMethod: req.method, method, params });

                const handler = streamHandlers[method];
                if (typeof handler !== "function") {
                    writeRPCError(res, 404, "h0", { code: 5, message: "method not found" });
                    return;
                }

                res.writeHead(200, {
                    "content-type": "text/event-stream",
                    "cache-control": "no-cache",
                    connection: "keep-alive",
                });

                let eventID = 1;
                try {
                    for await (const item of iterateStreamItems(await handler(params, { req, res }))) {
                        if (item && item.error) {
                            writeSSEEvent(res, "error", String(eventID++), {
                                jsonrpc: "2.0",
                                id: "h0",
                                error: item.error,
                            });
                            writeSSEEvent(res, "done", "", null);
                            res.end();
                            return;
                        }

                        writeSSEEvent(res, "message", String(eventID++), {
                            jsonrpc: "2.0",
                            id: "h0",
                            result: item ?? {},
                        });
                    }

                    writeSSEEvent(res, "done", "", null);
                    res.end();
                } catch (err) {
                    writeSSEEvent(res, "error", String(eventID++), {
                        jsonrpc: "2.0",
                        id: "h0",
                        error: {
                            code: 13,
                            message: err instanceof Error ? err.message : "internal error",
                        },
                    });
                    writeSSEEvent(res, "done", "", null);
                    res.end();
                }
                return;
            }

            if (req.method !== "POST") {
                res.writeHead(405);
                res.end();
                return;
            }

            const params = await readJSONBody(req);
            requests.push({ transport: "http", httpMethod: req.method, method, params });

            const handler = handlers[method];
            if (typeof handler !== "function") {
                writeRPCError(res, 404, "h0", { code: 5, message: "method not found" });
                return;
            }

            try {
                const result = await handler(params, { req, res });
                writeRPCResult(res, "h0", result ?? {});
            } catch (err) {
                writeRPCError(res, 500, "h0", {
                    code: 13,
                    message: err instanceof Error ? err.message : "internal error",
                });
            }
        } catch (err) {
            writeRPCError(res, 400, "h0", {
                code: -32700,
                message: err instanceof Error ? err.message : "parse error",
            });
        }
    });

    await new Promise((resolve, reject) => {
        server.once("error", reject);
        server.listen(0, "127.0.0.1", () => {
            server.off("error", reject);
            resolve();
        });
    });

    const address = server.address();
    if (!address || typeof address === "string") {
        throw new Error("failed to determine server address");
    }

    return {
        baseUrl: `http://127.0.0.1:${address.port}${basePath}`,
        requests,
        async close() {
            await new Promise((resolve) => server.close(() => resolve()));
        },
    };
}

function normalizeBasePath(value) {
    const text = String(value || "").trim();
    if (!text) {
        return DEFAULT_BASE_PATH;
    }
    return text.startsWith("/") ? text.replace(/\/+$/, "") : `/${text.replace(/\/+$/, "")}`;
}

function extractMethod(basePath, pathname) {
    const normalizedPath = pathname.replace(/\/+$/, "");
    if (!normalizedPath.startsWith(`${basePath}/`)) {
        return "";
    }
    return decodeURIComponent(normalizedPath.slice(basePath.length + 1));
}

function acceptsSSE(req) {
    return String(req.headers.accept || "").toLowerCase().includes("text/event-stream");
}

async function readJSONBody(req) {
    const chunks = [];
    for await (const chunk of req) {
        chunks.push(Buffer.from(chunk));
    }
    const raw = Buffer.concat(chunks).toString("utf8").trim();
    if (!raw) {
        return {};
    }
    const parsed = JSON.parse(raw);
    if (parsed == null) {
        return {};
    }
    if (typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error("request body must be a JSON object");
    }
    return parsed;
}

function queryParams(searchParams) {
    const params = {};
    for (const [key, value] of searchParams.entries()) {
        if (Object.prototype.hasOwnProperty.call(params, key)) {
            const current = params[key];
            params[key] = Array.isArray(current) ? [...current, value] : [current, value];
        } else {
            params[key] = value;
        }
    }
    return params;
}

async function *iterateStreamItems(value) {
    if (value == null) {
        return;
    }
    if (typeof value[Symbol.asyncIterator] === "function") {
        yield* value;
        return;
    }
    if (typeof value[Symbol.iterator] === "function") {
        yield* value;
        return;
    }
    yield value;
}

function writeRPCResult(res, id, result) {
    const payload = JSON.stringify({
        jsonrpc: "2.0",
        id,
        result,
    });
    res.writeHead(200, {
        "content-type": "application/json",
    });
    res.end(payload);
}

function writeRPCError(res, status, id, error) {
    const payload = JSON.stringify({
        jsonrpc: "2.0",
        id,
        error,
    });
    res.writeHead(status, {
        "content-type": "application/json",
    });
    res.end(payload);
}

function writeSSEEvent(res, event, id, payload) {
    res.write(`event: ${event}\n`);
    if (id) {
        res.write(`id: ${id}\n`);
    }
    if (payload == null) {
        res.write("data:\n\n");
        return;
    }
    res.write(`data: ${JSON.stringify(payload)}\n\n`);
}
