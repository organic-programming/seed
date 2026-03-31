import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { ALL, LOCAL, NO_TIMEOUT, resolve } from "../src/index.mjs";

describe("resolve", () => {
    it("known slug returns not-found error", () => {
        const result = resolve(LOCAL, "gabriel-greeting-go", null, ALL, NO_TIMEOUT);
        assert.equal(result.ref, null);
        assert.equal(result.error, 'holon "gabriel-greeting-go" not found');
    });

    it("missing target returns not-found error", () => {
        const result = resolve(LOCAL, "", null, ALL, NO_TIMEOUT);
        assert.equal(result.ref, null);
        assert.equal(result.error, 'holon "" not found');
    });

    it("invalid specifiers returns error", () => {
        const result = resolve(LOCAL, "gabriel-greeting-go", null, 0xFF, NO_TIMEOUT);
        assert.equal(result.ref, null);
        assert.match(result.error || "", /invalid specifiers/i);
    });
});
