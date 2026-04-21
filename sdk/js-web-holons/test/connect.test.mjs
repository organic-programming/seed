import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { ALL, LOCAL, connect, disconnect } from "../src/index.mjs";

describe("connect", () => {
    it("unresolvable target returns error", () => {
        const result = connect(LOCAL, "gabriel-greeting-go", null, ALL, 1000);
        assert.equal(result.channel, null);
        assert.equal(result.uid, "");
        assert.equal(result.origin, null);
        assert.equal(result.error, 'holon "gabriel-greeting-go" not found');
    });

    it("returns ConnectResult", () => {
        const result = connect(LOCAL, "missing", null, ALL, 1000);
        assert.deepEqual(result, {
            channel: null,
            uid: "",
            origin: null,
            error: 'holon "missing" not found',
        });
    });

    it("disconnect accepts ConnectResult", () => {
        const result = connect(LOCAL, "missing", null, ALL, 1000);
        assert.doesNotThrow(() => disconnect(result));
    });
});
