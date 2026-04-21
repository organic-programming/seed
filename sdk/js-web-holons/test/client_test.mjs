import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { createServer as createNetServer } from "node:net";
import WebSocket, { WebSocketServer } from "ws";
import { HolonClient, HolonError } from "../src/index.mjs";

let loopbackProbe = null;

function isSocketPermissionError(err) {
    if (!err) return false;
    if (err.code === "EPERM" || err.code === "EACCES") return true;
    return /listen\s+(eperm|eacces)/i.test(String(err.message || err));
}

function canListenOnLoopback() {
    if (loopbackProbe) {
        return loopbackProbe;
    }

    loopbackProbe = new Promise((resolve) => {
        const probe = createNetServer();

        probe.once("error", (err) => {
            resolve(!isSocketPermissionError(err));
        });

        probe.listen(0, "127.0.0.1", () => {
            probe.close(() => resolve(true));
        });
    });

    return loopbackProbe;
}

function itRequiresLoopback(name, fn) {
    it(name, async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip("socket bind not permitted in this environment");
            return;
        }
        await fn();
    });
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

function withTimeout(promise, timeoutMs, message) {
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error(message)), timeoutMs);
        promise.then(
            (value) => {
                clearTimeout(timer);
                resolve(value);
            },
            (err) => {
                clearTimeout(timer);
                reject(err);
            },
        );
    });
}

function sendJSON(ws, payload) {
    if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(payload));
    }
}

function startMockServer(options = {}) {
    const slowDelayMs = options.slowDelayMs ?? 90;
    const dropMethods = new Set(options.dropMethods ?? []);

    return new Promise((resolve, reject) => {
        const http = createServer();
        const wss = new WebSocketServer({ server: http, path: "/" });

        const connections = [];
        const requests = [];
        const connectionWaiters = [];
        const requestWaiters = [];

        function fulfillConnectionWaiters() {
            while (connectionWaiters.length > 0 && connections.length >= connectionWaiters[0].count) {
                const waiter = connectionWaiters.shift();
                waiter.resolve(connections[waiter.count - 1]);
            }
        }

        function fulfillRequestWaiters(msg) {
            for (let i = requestWaiters.length - 1; i >= 0; i -= 1) {
                const waiter = requestWaiters[i];
                if (waiter.method === msg.method) {
                    requestWaiters.splice(i, 1);
                    waiter.resolve(msg);
                }
            }
        }

        wss.on("connection", (ws) => {
            connections.push(ws);
            fulfillConnectionWaiters();

            ws.on("message", (data) => {
                let msg;
                try {
                    msg = JSON.parse(data.toString());
                } catch {
                    return;
                }

                if (!msg.method) {
                    return;
                }

                requests.push(msg);
                fulfillRequestWaiters(msg);

                if (dropMethods.has(msg.method)) {
                    return;
                }

                if (msg.method === "hello.v1.HelloService/Greet") {
                    const name = msg.params?.name || "World";
                    sendJSON(ws, {
                        jsonrpc: "2.0",
                        id: msg.id,
                        result: { message: `Hello, ${name}!` },
                    });
                    return;
                }

                if (msg.method === "test.v1.Service/Dupe") {
                    sendJSON(ws, { jsonrpc: "2.0", id: msg.id, result: { ok: true } });
                    setTimeout(() => {
                        sendJSON(ws, { jsonrpc: "2.0", id: msg.id, result: { ok: true, duplicate: true } });
                    }, 10);
                    return;
                }

                if (msg.method === "test.v1.Service/Slow") {
                    setTimeout(() => {
                        sendJSON(ws, { jsonrpc: "2.0", id: msg.id, result: { ok: true } });
                    }, slowDelayMs);
                    return;
                }

                if (msg.method === "test.v1.Service/Hang") {
                    return;
                }

                sendJSON(ws, {
                    jsonrpc: "2.0",
                    id: msg.id,
                    error: { code: 12, message: `method "${msg.method}" not registered` },
                });
            });
        });

        http.on("error", reject);
        http.listen(0, "127.0.0.1", () => {
            const address = http.address();
            const port = address && typeof address === "object" ? address.port : 0;

            resolve({
                server: http,
                wss,
                port,
                connections,
                requests,
                waitForConnectionCount: (count, timeout = 1000) => {
                    if (connections.length >= count) {
                        return Promise.resolve(connections[count - 1]);
                    }
                    return withTimeout(
                        new Promise((res) => connectionWaiters.push({ count, resolve: res })),
                        timeout,
                        `timed out waiting for ${count} websocket connection(s)`,
                    );
                },
                waitForRequest: (method, timeout = 1000) => {
                    const existing = requests.find((r) => r.method === method);
                    if (existing) {
                        return Promise.resolve(existing);
                    }
                    return withTimeout(
                        new Promise((res) => requestWaiters.push({ method, resolve: res })),
                        timeout,
                        `timed out waiting for request method ${method}`,
                    );
                },
            });
        });
    });
}

async function stopMockServer(ctx) {
    for (const ws of ctx.connections) {
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
            ws.terminate();
        }
    }

    await new Promise((resolve) => ctx.wss.close(() => resolve()));
    await new Promise((resolve) => ctx.server.close(() => resolve()));
}

function serverInvoke(ws, method, payload, timeout = 5000) {
    return new Promise((resolve, reject) => {
        const id = `s${Math.random().toString(36).slice(2)}`;
        const timer = setTimeout(() => {
            ws.off("message", handler);
            reject(new Error("server invoke timeout"));
        }, timeout);

        const handler = (data) => {
            const msg = JSON.parse(data.toString());
            if (msg.id !== id) return;
            clearTimeout(timer);
            ws.off("message", handler);
            if (msg.error) reject(new Error(msg.error.message));
            else resolve(msg.result);
        };

        ws.on("message", handler);
        ws.send(JSON.stringify({ jsonrpc: "2.0", id, method, params: payload }));
    });
}

async function withHarness(options, fn) {
    const serverCtx = await startMockServer(options?.server);
    const warnings = [];
    const client = new HolonClient(`ws://127.0.0.1:${serverCtx.port}/`, {
        WebSocket,
        reconnect: false,
        heartbeat: false,
        ...options?.client,
        onProtocolWarning: (warning) => {
            warnings.push(warning);
            options?.client?.onProtocolWarning?.(warning);
        },
    });

    try {
        await fn({ ...serverCtx, client, warnings });
    } finally {
        client.close();
        await stopMockServer(serverCtx);
    }
}

async function waitForWarning(warnings, type, timeout = 1000) {
    await withTimeout(
        (async () => {
            for (;;) {
                if (warnings.some((w) => w.type === type)) {
                    return;
                }
                await wait(10);
            }
        })(),
        timeout,
        `timed out waiting for warning ${type}`,
    );
}

describe("HolonClient", () => {
    // --- Browser → Go ---

    itRequiresLoopback("browser→go: invokes a method and receives a result", async () => {
        await withHarness({}, async ({ client }) => {
            const result = await client.invoke("hello.v1.HelloService/Greet", { name: "Bob" });
            assert.equal(result.message, "Hello, Bob!");
        });
    });

    itRequiresLoopback("browser→go: uses default name when payload is empty", async () => {
        await withHarness({}, async ({ client }) => {
            const result = await client.invoke("hello.v1.HelloService/Greet", {});
            assert.equal(result.message, "Hello, World!");
        });
    });

    itRequiresLoopback("browser→go: returns HolonError for unknown methods", async () => {
        await withHarness({}, async ({ client }) => {
            await assert.rejects(
                () => client.invoke("no.Such/Method"),
                (err) => {
                    assert(err instanceof HolonError);
                    assert.equal(err.code, 12);
                    return true;
                },
            );
        });
    });

    // --- Go → Browser ---

    itRequiresLoopback("go→browser: server calls registered browser handler", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount }) => {
            client.register("ui.v1.UIService/GetViewport", (payload) => ({
                width: 1920,
                height: 1080,
                devicePixelRatio: payload.dpr ?? 1,
            }));

            await client.connect();
            const ws = await waitForConnectionCount(1);
            const result = await serverInvoke(ws, "ui.v1.UIService/GetViewport", { dpr: 2 });

            assert.equal(result.width, 1920);
            assert.equal(result.height, 1080);
            assert.equal(result.devicePixelRatio, 2);
        });
    });

    itRequiresLoopback("go→browser: server gets error for unregistered method", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount }) => {
            await client.connect();
            const ws = await waitForConnectionCount(1);

            await assert.rejects(
                () => serverInvoke(ws, "no.Such/Method", {}),
                (err) => {
                    assert(err.message.includes("not registered"));
                    return true;
                },
            );
        });
    });

    itRequiresLoopback("go→browser: handler errors propagate to server", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount }) => {
            client.register("test.v1.Service/Fail", () => {
                throw new HolonError(3, "bad request from handler");
            });

            await client.connect();
            const ws = await waitForConnectionCount(1);

            await assert.rejects(
                () => serverInvoke(ws, "test.v1.Service/Fail", {}),
                (err) => {
                    assert(err.message.includes("bad request from handler"));
                    return true;
                },
            );
        });
    });

    // --- Protocol hardening ---

    itRequiresLoopback("protocol: malformed JSON is detected", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount, warnings }) => {
            await client.connect();
            const ws = await waitForConnectionCount(1);

            ws.send("{bad-json");
            await waitForWarning(warnings, "malformed_json");
        });
    });

    itRequiresLoopback("protocol: missing required fields are rejected", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount, warnings }) => {
            await client.connect();
            const ws = await waitForConnectionCount(1);

            ws.send(JSON.stringify({ jsonrpc: "2.0", method: "x.Y/Z", params: {} })); // missing id
            await waitForWarning(warnings, "invalid_envelope");
        });
    });

    itRequiresLoopback("protocol: wrong field types are rejected", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount, warnings }) => {
            await client.connect();
            const ws = await waitForConnectionCount(1);

            ws.send(JSON.stringify({ jsonrpc: "2.0", id: 1, result: {} }));
            await waitForWarning(warnings, "invalid_envelope");
        });
    });

    itRequiresLoopback("protocol: unknown response IDs are classified", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount, warnings }) => {
            await client.connect();
            const ws = await waitForConnectionCount(1);

            ws.send(JSON.stringify({ jsonrpc: "2.0", id: "unknown-1", result: { ok: true } }));
            await waitForWarning(warnings, "unknown_response_id");
        });
    });

    itRequiresLoopback("protocol: duplicate response IDs are classified", async () => {
        await withHarness({}, async ({ client, warnings }) => {
            const result = await client.invoke("test.v1.Service/Dupe", {});
            assert.equal(result.ok, true);

            await waitForWarning(warnings, "duplicate_response_id");
        });
    });

    itRequiresLoopback("protocol: stale response IDs are classified after timeout", async () => {
        await withHarness({ server: { slowDelayMs: 120 } }, async ({ client, warnings }) => {
            await assert.rejects(
                () => client.invoke("test.v1.Service/Slow", {}, { timeout: 20 }),
                (err) => {
                    assert(err instanceof HolonError);
                    assert.equal(err.code, 4);
                    return true;
                },
            );

            await waitForWarning(warnings, "stale_response_id");
        });
    });

    // --- Connection resilience ---

    itRequiresLoopback("resilience: enforces max pending request limit", async () => {
        await withHarness(
            { client: { maxPendingRequests: 1 } },
            async ({ client }) => {
                const first = client.invoke("test.v1.Service/Hang", {}, { timeout: 1000 });
                await assert.rejects(
                    () => client.invoke("test.v1.Service/Hang", {}, { timeout: 1000 }),
                    (err) => {
                        assert(err instanceof HolonError);
                        assert.equal(err.code, 8);
                        return true;
                    },
                );
                first.catch(() => {});
            },
        );
    });

    itRequiresLoopback("resilience: uses configurable default timeout", async () => {
        await withHarness(
            { server: { slowDelayMs: 90 }, client: { defaultTimeout: 25 } },
            async ({ client }) => {
                await assert.rejects(
                    () => client.invoke("test.v1.Service/Slow", {}),
                    (err) => {
                        assert(err instanceof HolonError);
                        assert.equal(err.code, 4);
                        return true;
                    },
                );
            },
        );
    });

    itRequiresLoopback("resilience: reconnects with backoff after disconnect", async () => {
        await withHarness(
            {
                client: {
                    reconnect: {
                        enabled: true,
                        minDelay: 10,
                        maxDelay: 50,
                        factor: 1.5,
                        jitter: 0,
                    },
                },
            },
            async ({ client, waitForConnectionCount }) => {
                const first = await client.invoke("hello.v1.HelloService/Greet", { name: "A" });
                assert.equal(first.message, "Hello, A!");

                const ws1 = await waitForConnectionCount(1);
                ws1.close(1001, "drop");

                await waitForConnectionCount(2, 2000);
                const second = await client.invoke("hello.v1.HelloService/Greet", { name: "B" });
                assert.equal(second.message, "Hello, B!");
            },
        );
    });

    itRequiresLoopback("resilience: heartbeat timeout detects stale connection", async () => {
        await withHarness(
            {
                server: { dropMethods: ["test.v1.Service/Heartbeat"] },
                client: {
                    reconnect: false,
                    heartbeat: {
                        enabled: true,
                        interval: 20,
                        timeout: 20,
                        method: "test.v1.Service/Heartbeat",
                        params: {},
                    },
                },
            },
            async ({ client, warnings }) => {
                await client.connect();
                await waitForWarning(warnings, "heartbeat_timeout", 2000);
            },
        );
    });

    itRequiresLoopback("resilience: in-flight invokes reject when disconnected", async () => {
        await withHarness({}, async ({ client, waitForConnectionCount, waitForRequest }) => {
            const pending = client.invoke("test.v1.Service/Hang", {}, { timeout: 2000 });
            await waitForRequest("test.v1.Service/Hang");

            const ws = await waitForConnectionCount(1);
            ws.close(1001, "disconnect");

            await assert.rejects(
                () => pending,
                (err) => {
                    assert(err instanceof HolonError);
                    assert.equal(err.code, 14);
                    return true;
                },
            );
        });
    });
});
