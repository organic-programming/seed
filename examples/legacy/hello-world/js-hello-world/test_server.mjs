import { describe, it } from "node:test";
import assert from "node:assert/strict";

// Direct unit test — call the greet handler without starting a server.
import { greet, register } from "./server.mjs";

describe("HelloService", () => {
    it("greets with a name", (_, done) => {
        const call = { request: { name: "Bob" } };
        greet(call, (err, resp) => {
            assert.equal(err, null);
            assert.equal(resp.message, "Hello, Bob!");
            done();
        });
    });

    it("defaults to World", (_, done) => {
        const call = { request: { name: "" } };
        greet(call, (err, resp) => {
            assert.equal(err, null);
            assert.equal(resp.message, "Hello, World!");
            done();
        });
    });

    it("register() adds HelloService", () => {
        let capturedService = null;
        let capturedImpl = null;
        const fakeServer = {
            addService(service, impl) {
                capturedService = service;
                capturedImpl = impl;
            },
        };

        register(fakeServer);
        assert.ok(capturedService);
        assert.equal(typeof capturedImpl.Greet, "function");
    });
});
