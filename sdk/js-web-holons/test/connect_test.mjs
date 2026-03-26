import { describe, it } from "node:test";
import assert from "node:assert/strict";
import WebSocket from "ws";
import { connect, disconnect, HolonClient, HolonHTTPClient } from "../src/index.mjs";

describe("connect", () => {
    it("returns an HTTP Holon-RPC client for http:// targets", () => {
        const client = connect("http://localhost:9090");

        assert.ok(client);
        assert.ok(client instanceof HolonHTTPClient);
        assert.equal(client.baseUrl, "http://localhost:9090/api/v1/rpc");
    });

    it("returns a WebSocket Holon-RPC client for ws:// targets", () => {
        const client = connect("ws://localhost:9090", {
            WebSocket,
            reconnect: false,
            heartbeat: false,
        });

        assert.ok(client instanceof HolonClient);
        assert.doesNotThrow(() => disconnect(client));
    });

    it("rejects bare host:port targets because the transport is ambiguous", () => {
        assert.throws(() => connect("localhost:9090"), /requires an explicit ws:\/\/, wss:\/\/, http:\/\/, or https:\/\/ URI/i);
    });

    it("throws for bare slug targets", () => {
        assert.throws(() => connect("my-holon"), /slug resolution is unavailable in js-web-holons/i);
    });

    it("rejects non-browser transports", () => {
        assert.throws(() => connect("tcp://127.0.0.1:9090"), /unsupported browser connect target/i);
        assert.throws(() => connect("unix:///tmp/holon.sock"), /unsupported browser connect target/i);
        assert.throws(() => connect("stdio://"), /unsupported browser connect target/i);
    });

    it("disconnects a valid client without throwing", () => {
        const client = connect("https://example.com");
        assert.doesNotThrow(() => disconnect(client));
    });
});
