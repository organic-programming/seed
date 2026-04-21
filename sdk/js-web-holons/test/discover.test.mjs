import { describe, it } from "node:test";
import assert from "node:assert/strict";

import {
    ALL,
    BUILT,
    CACHED,
    CWD,
    Discover,
    LOCAL,
    NO_LIMIT,
    NO_TIMEOUT,
    PROXY,
    SIBLINGS,
    SOURCE,
    INSTALLED,
} from "../src/index.mjs";

function assertEmptyResult(result) {
    assert.deepEqual(result, { found: [], error: null });
}

describe("Discover", () => {
    it("discover all layers returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("filter by specifiers returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, CWD | BUILT, NO_LIMIT, NO_TIMEOUT));
    });

    it("slug expression returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, "gabriel-greeting-go", null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("alias expression returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, "op", null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("UUID prefix returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, "3f08b5c3", null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("path expression returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, "./holons/foo", null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("limit one still returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, ALL, 1, NO_TIMEOUT));
    });

    it("limit zero means unlimited but empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, ALL, 0, NO_TIMEOUT));
    });

    it("negative limit returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, ALL, -1, NO_TIMEOUT));
    });

    it("invalid specifiers returns error", () => {
        const result = Discover(LOCAL, null, null, 0xFF, NO_LIMIT, NO_TIMEOUT);
        assert.deepEqual(result.found, []);
        assert.match(result.error || "", /invalid specifiers/i);
    });

    it("specifiers zero treated as all", () => {
        const allResult = Discover(LOCAL, null, null, ALL, NO_LIMIT, NO_TIMEOUT);
        const zeroResult = Discover(LOCAL, null, null, 0, NO_LIMIT, NO_TIMEOUT);
        assert.deepEqual(zeroResult, allResult);
    });

    it("null expression returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("missing expression returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, undefined, null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("siblings layer returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, SIBLINGS, NO_LIMIT, NO_TIMEOUT));
    });

    it("source layer returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, SOURCE, NO_LIMIT, NO_TIMEOUT));
    });

    it("built layer returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, BUILT, NO_LIMIT, NO_TIMEOUT));
    });

    it("installed layer returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, INSTALLED, NO_LIMIT, NO_TIMEOUT));
    });

    it("cached layer returns empty found", () => {
        assertEmptyResult(Discover(LOCAL, null, null, CACHED, NO_LIMIT, NO_TIMEOUT));
    });

    it("nil root accepted", () => {
        assertEmptyResult(Discover(LOCAL, null, null, ALL, NO_LIMIT, NO_TIMEOUT));
    });

    it("empty root returns error", () => {
        const result = Discover(LOCAL, null, "", ALL, NO_LIMIT, NO_TIMEOUT);
        assert.deepEqual(result.found, []);
        assert.equal(result.error, "root cannot be empty");
    });

    it("unsupported scope returns error", () => {
        const result = Discover(PROXY, null, null, ALL, NO_LIMIT, NO_TIMEOUT);
        assert.deepEqual(result.found, []);
        assert.match(result.error || "", /scope 1 not supported/i);
    });
});
