import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { HolonError, HolonHTTPClient } from "../src/index.mjs";
import { startHTTPHolonRPCServer } from "./support/http_harness.mjs";

async function collect(asyncIterable) {
    const items = [];
    for await (const item of asyncIterable) {
        items.push(item);
    }
    return items;
}

describe("HolonHTTPClient", () => {
    it("invokes unary HTTP JSON-RPC methods with POST", async () => {
        const server = await startHTTPHolonRPCServer({
            handlers: {
                "echo.v1.Echo/Ping": async (params) => ({
                    message: String(params.message || ""),
                    sdk: "go-holons",
                }),
            },
        });
        const client = new HolonHTTPClient(server.baseUrl);

        try {
            const response = await client.invoke("echo.v1.Echo/Ping", { message: "hello" });

            assert.deepEqual(response, {
                message: "hello",
                sdk: "go-holons",
            });
            assert.deepEqual(server.requests[0], {
                transport: "http",
                httpMethod: "POST",
                method: "echo.v1.Echo/Ping",
                params: { message: "hello" },
            });
        } finally {
            client.close();
            await server.close();
        }
    });

    it("surfaces JSON-RPC errors from unary HTTP calls", async () => {
        const server = await startHTTPHolonRPCServer();
        const client = new HolonHTTPClient(server.baseUrl);

        try {
            await assert.rejects(
                () => client.invoke("does.not.Exist/Nope", {}),
                (err) => {
                    assert(err instanceof HolonError);
                    assert.equal(err.code, 5);
                    return true;
                },
            );
        } finally {
            client.close();
            await server.close();
        }
    });

    it("streams SSE events over POST", async () => {
        const server = await startHTTPHolonRPCServer({
            streamHandlers: {
                "build.v1.BuildService/WatchBuild": async function* (params) {
                    yield { status: "building", project: params.project, progress: 42 };
                    yield { status: "done", project: params.project, progress: 100 };
                },
            },
        });
        const client = new HolonHTTPClient(server.baseUrl);

        try {
            const events = await collect(
                client.stream("build.v1.BuildService/WatchBuild", { project: "hello-world" }),
            );

            assert.deepEqual(events, [
                {
                    event: "message",
                    id: "1",
                    result: { status: "building", project: "hello-world", progress: 42 },
                    error: null,
                },
                {
                    event: "message",
                    id: "2",
                    result: { status: "done", project: "hello-world", progress: 100 },
                    error: null,
                },
                {
                    event: "done",
                    id: "",
                    result: null,
                    error: null,
                },
            ]);

            assert.deepEqual(server.requests[0], {
                transport: "sse",
                httpMethod: "POST",
                method: "build.v1.BuildService/WatchBuild",
                params: { project: "hello-world" },
            });
        } finally {
            client.close();
            await server.close();
        }
    });

    it("streams SSE events over GET with query params", async () => {
        const server = await startHTTPHolonRPCServer({
            streamHandlers: {
                "build.v1.BuildService/WatchBuild": async function* (params) {
                    yield { status: "watching", project: params.project };
                },
            },
        });
        const client = new HolonHTTPClient(server.baseUrl);

        try {
            const events = await collect(
                client.streamQuery("build.v1.BuildService/WatchBuild", { project: "gabriel" }),
            );

            assert.equal(events[0].result.project, "gabriel");
            assert.deepEqual(server.requests[0], {
                transport: "sse",
                httpMethod: "GET",
                method: "build.v1.BuildService/WatchBuild",
                params: { project: "gabriel" },
            });
        } finally {
            client.close();
            await server.close();
        }
    });

    it("emits SSE error events without crashing the parser", async () => {
        const server = await startHTTPHolonRPCServer({
            streamHandlers: {
                "build.v1.BuildService/WatchBuild": async function* () {
                    yield { status: "building", progress: 42 };
                    yield {
                        error: {
                            code: 13,
                            message: "build failed",
                        },
                    };
                },
            },
        });
        const client = new HolonHTTPClient(server.baseUrl);

        try {
            const events = await collect(client.stream("build.v1.BuildService/WatchBuild", {}));

            assert.equal(events.length, 2);
            assert.equal(events[0].event, "message");
            assert.equal(events[1].event, "error");
            assert(events[1].error instanceof HolonError);
            assert.equal(events[1].error.code, 13);
        } finally {
            client.close();
            await server.close();
        }
    });
});
