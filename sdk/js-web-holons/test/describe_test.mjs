import { describe, it } from "node:test";
import assert from "node:assert/strict";
import WebSocket from "ws";

import {
    HolonClient,
    HolonHTTPClient,
    HOLON_META_METHOD,
    describe as describeHolon,
} from "../src/index.mjs";
import { HolonServer } from "../src/server.mjs";
import { startHTTPHolonRPCServer } from "./support/http_harness.mjs";

const DESCRIBE_RESPONSE = {
    manifest: {
        identity: {
            uuid: "gabriel-greeting-node",
            given_name: "Gabriel",
            family_name: "Greeting",
            motto: "Answer kindly.",
            composer: "js-web-holons-test",
            status: "draft",
            born: "2026-03-23",
            lang: "js-web",
        },
    },
    services: [
        {
            name: "greeting.v1.GreetingService",
            description: "Simple greeting service",
            methods: [
                {
                    name: "SayHello",
                    description: "Returns a greeting",
                    input_type: "greeting.v1.GreetingRequest",
                    output_type: "greeting.v1.GreetingResponse",
                    input_fields: [],
                    output_fields: [],
                    client_streaming: false,
                    server_streaming: false,
                    example_input: "{\"name\":\"Ada\"}",
                },
            ],
        },
    ],
};

describe("describe", () => {
    it("calls HolonMeta/Describe over WebSocket JSON-RPC", async () => {
        const server = new HolonServer("ws://127.0.0.1:0/api/v1/rpc", { maxConnections: 1 });
        server.register(HOLON_META_METHOD, async () => DESCRIBE_RESPONSE);

        const address = await server.start();
        try {
            const response = await describeHolon(address, {}, {
                connectOptions: {
                    WebSocket,
                    reconnect: false,
                    heartbeat: false,
                },
            });

            assert.deepEqual(response, DESCRIBE_RESPONSE);
        } finally {
            await server.close();
        }
    });

    it("calls HolonMeta/Describe over HTTP POST", async () => {
        const server = await startHTTPHolonRPCServer({
            handlers: {
                [HOLON_META_METHOD]: async () => DESCRIBE_RESPONSE,
            },
        });

        try {
            const response = await describeHolon(server.baseUrl);
            assert.deepEqual(response, DESCRIBE_RESPONSE);
        } finally {
            await server.close();
        }
    });

    it("accepts an existing client instance", async () => {
        const server = await startHTTPHolonRPCServer({
            handlers: {
                [HOLON_META_METHOD]: async () => DESCRIBE_RESPONSE,
            },
        });
        const client = new HolonHTTPClient(server.baseUrl);

        try {
            const response = await describeHolon(client);
            assert.deepEqual(response, DESCRIBE_RESPONSE);
        } finally {
            client.close();
            await server.close();
        }
    });

    it("works with an existing WebSocket client instance", async () => {
        const server = new HolonServer("ws://127.0.0.1:0/api/v1/rpc", { maxConnections: 1 });
        server.register(HOLON_META_METHOD, async () => DESCRIBE_RESPONSE);

        const address = await server.start();
        const client = new HolonClient(address, {
            WebSocket,
            reconnect: false,
            heartbeat: false,
        });

        try {
            const response = await describeHolon(client);
            assert.deepEqual(response, DESCRIBE_RESPONSE);
        } finally {
            client.close();
            await server.close();
        }
    });
});
