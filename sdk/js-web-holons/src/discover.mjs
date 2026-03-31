import {
    ALL,
    LOCAL,
} from "./discovery_types.mjs";

export {
    ALL,
    BUILT,
    CACHED,
    CWD,
    DELEGATED,
    INSTALLED,
    LOCAL,
    NO_LIMIT,
    NO_TIMEOUT,
    PROXY,
    SIBLINGS,
    SOURCE,
} from "./discovery_types.mjs";

/**
 * Browser Phase 1 discovery is validation-only. All filesystem-backed layers
 * are unavailable in js-web-holons, so successful calls return an empty set.
 *
 * @param {number} scope
 * @param {string|null|undefined} expression
 * @param {string|null|undefined} root
 * @param {number} specifiers
 * @param {number} limit
 * @param {number} timeout
 * @returns {import("./discovery_types.mjs").DiscoverResult}
 */
export function Discover(scope, expression, root, specifiers, limit, timeout) {
    void expression;
    void timeout;

    if (scope !== LOCAL) {
        return { found: [], error: `scope ${scope} not supported` };
    }

    const normalizedSpecifiers = normalizeSpecifiers(specifiers);
    if (normalizedSpecifiers.error !== null) {
        return { found: [], error: normalizedSpecifiers.error };
    }

    if (normalizeLimit(limit) < 0) {
        return { found: [], error: null };
    }

    const rootError = validateRoot(root);
    if (rootError !== null) {
        return { found: [], error: rootError };
    }

    return { found: [], error: null };
}

/**
 * Resolve the first browser-discoverable holon. Phase 1 browser discovery has
 * no discoverable layers, so resolution returns not found after validation.
 *
 * @param {number} scope
 * @param {string|null|undefined} expression
 * @param {string|null|undefined} root
 * @param {number} specifiers
 * @param {number} timeout
 * @returns {import("./discovery_types.mjs").ResolveResult}
 */
export function resolve(scope, expression, root, specifiers, timeout) {
    const result = Discover(scope, expression, root, specifiers, 1, timeout);
    if (result.error !== null) {
        return { ref: null, error: result.error };
    }

    const target = normalizeExpression(expression);
    return { ref: null, error: `holon "${target}" not found` };
}

function normalizeSpecifiers(specifiers) {
    const bits = specifiers == null ? 0 : Number(specifiers);
    if (!Number.isInteger(bits) || bits < 0 || (bits & ~ALL) !== 0) {
        return {
            value: null,
            error: `invalid specifiers 0x${formatHex(specifiers)}: valid range is 0x00-0x3F`,
        };
    }
    return {
        value: bits === 0 ? ALL : bits,
        error: null,
    };
}

function normalizeLimit(limit) {
    if (limit == null) {
        return 0;
    }
    const value = Number(limit);
    if (Number.isNaN(value)) {
        return 0;
    }
    return value;
}

function validateRoot(root) {
    if (root == null) {
        return null;
    }

    if (String(root).trim() === "") {
        return "root cannot be empty";
    }

    return null;
}

function normalizeExpression(expression) {
    if (expression == null) {
        return "";
    }
    return String(expression).trim();
}

function formatHex(value) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) {
        return "00";
    }
    return (Math.trunc(numeric) >>> 0).toString(16).toUpperCase().padStart(2, "0");
}
