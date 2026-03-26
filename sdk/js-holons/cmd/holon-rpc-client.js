#!/usr/bin/env node
'use strict';

const {
    HolonRPCClient,
    HolonRPCError,
} = require('../src/holonrpc_client');

const DEFAULT_SDK = 'js-holons';
const DEFAULT_SERVER_SDK = 'go-holons';
const DEFAULT_METHOD = 'echo.v1.Echo/Ping';
const DEFAULT_MESSAGE = 'cert';
const DEFAULT_TIMEOUT_MS = 5000;

function parseArgs(argv = process.argv) {
    const out = {
        uri: '',
        sdk: DEFAULT_SDK,
        serverSDK: DEFAULT_SERVER_SDK,
        method: DEFAULT_METHOD,
        message: DEFAULT_MESSAGE,
        timeoutMs: DEFAULT_TIMEOUT_MS,
        params: null,
        connectOnly: false,
        expectErrorCodes: [],
    };

    for (let i = 2; i < argv.length; i += 1) {
        const token = argv[i];

        if (token === '--sdk') {
            out.sdk = requireValue(argv, i, '--sdk');
            i += 1;
            continue;
        }
        if (token === '--server-sdk') {
            out.serverSDK = requireValue(argv, i, '--server-sdk');
            i += 1;
            continue;
        }
        if (token === '--method') {
            out.method = requireValue(argv, i, '--method');
            i += 1;
            continue;
        }
        if (token === '--message') {
            out.message = requireValue(argv, i, '--message');
            i += 1;
            continue;
        }
        if (token === '--params-json') {
            out.params = parseParamsJSON(requireValue(argv, i, '--params-json'));
            i += 1;
            continue;
        }
        if (token === '--expect-error') {
            out.expectErrorCodes = parseExpectedErrorCodes(requireValue(argv, i, '--expect-error'));
            i += 1;
            continue;
        }
        if (token === '--timeout-ms') {
            const raw = requireValue(argv, i, '--timeout-ms');
            const parsed = Number(raw);
            if (!Number.isFinite(parsed) || parsed <= 0) {
                throw new Error('--timeout-ms must be a positive number');
            }
            out.timeoutMs = Math.trunc(parsed);
            i += 1;
            continue;
        }
        if (token === '--connect-only') {
            out.connectOnly = true;
            continue;
        }
        if (token.startsWith('--')) {
            throw new Error(`unknown flag: ${token}`);
        }
        if (out.uri !== '') {
            throw new Error(`unexpected argument: ${token}`);
        }

        out.uri = token;
    }

    if (!out.uri) {
        throw new Error('usage: node ./cmd/holon-rpc-client.js <ws://|wss://|http://|https://|rest+sse://...> [flags]');
    }

    if (out.params === null) {
        if (out.method === DEFAULT_METHOD) {
            out.params = { message: out.message };
        } else {
            out.params = {};
        }
    }

    return out;
}

async function run(argv = process.argv, deps = {}) {
    const args = parseArgs(argv);
    const now = typeof deps.now === 'function' ? deps.now : Date.now;

    const createClient = deps.createClient || (() => new HolonRPCClient());
    const client = createClient(args);
    if (!client || typeof client.connect !== 'function' || typeof client.invoke !== 'function') {
        throw new Error('createClient() must return a HolonRPCClient-compatible instance');
    }

    const startedAt = now();

    try {
        await client.connect(args.uri, { timeout: args.timeoutMs });

        if (args.connectOnly) {
            return {
                status: 'pass',
                sdk: args.sdk,
                server_sdk: args.serverSDK,
                latency_ms: Math.max(0, now() - startedAt),
                check: 'connect',
            };
        }

        try {
            const result = await client.invoke(args.method, args.params, { timeout: args.timeoutMs });

            if (args.expectErrorCodes.length > 0) {
                throw new Error(`expected error code ${args.expectErrorCodes.join('|')}, but call succeeded`);
            }

            if (args.method === DEFAULT_METHOD) {
                const expected = String(args.params.message || '');
                const actual = String((result || {}).message || '');
                if (actual !== expected) {
                    throw new Error(`unexpected echo response: ${JSON.stringify(result || {})}`);
                }
            }

            return {
                status: 'pass',
                sdk: args.sdk,
                server_sdk: args.serverSDK,
                latency_ms: Math.max(0, now() - startedAt),
                method: args.method,
            };
        } catch (err) {
            const code = extractErrorCode(err);
            if (args.expectErrorCodes.length > 0 && args.expectErrorCodes.includes(code)) {
                return {
                    status: 'pass',
                    sdk: args.sdk,
                    server_sdk: args.serverSDK,
                    latency_ms: Math.max(0, now() - startedAt),
                    method: args.method,
                    error_code: code,
                };
            }

            throw err;
        }
    } finally {
        if (typeof client.close === 'function') {
            await client.close();
        }
    }
}

async function main() {
    try {
        const result = await run(process.argv);
        process.stdout.write(`${JSON.stringify(result)}\n`);
    } catch (err) {
        process.stderr.write(`${err.message}\n`);
        process.exit(1);
    }
}

function requireValue(argv, index, flag) {
    const value = argv[index + 1];
    if (typeof value !== 'string' || value === '') {
        throw new Error(`missing value for ${flag}`);
    }
    return value;
}

function parseParamsJSON(raw) {
    let decoded;
    try {
        decoded = JSON.parse(raw);
    } catch {
        throw new Error('--params-json must be valid JSON');
    }

    if (!decoded || Array.isArray(decoded) || typeof decoded !== 'object') {
        throw new Error('--params-json must decode to a JSON object');
    }

    return decoded;
}

function parseExpectedErrorCodes(raw) {
    const tokens = raw.split(',').map((part) => part.trim()).filter(Boolean);
    if (tokens.length === 0) {
        throw new Error('--expect-error requires at least one numeric code');
    }

    const codes = [];
    for (const token of tokens) {
        const code = Number(token);
        if (!Number.isFinite(code)) {
            throw new Error(`invalid error code in --expect-error: ${token}`);
        }
        codes.push(Math.trunc(code));
    }

    return codes;
}

function extractErrorCode(err) {
    if (err instanceof HolonRPCError && Number.isFinite(err.code)) {
        return Number(err.code);
    }

    const maybeCode = Number(err?.code);
    if (Number.isFinite(maybeCode)) {
        return maybeCode;
    }

    return NaN;
}

module.exports = {
    parseArgs,
    run,
};

if (require.main === module) {
    main();
}
