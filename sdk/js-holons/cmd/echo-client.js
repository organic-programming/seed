#!/usr/bin/env node
'use strict';

const path = require('node:path');
const grpc = require('@grpc/grpc-js');

const grpcclient = require('../src/grpcclient');

const DEFAULT_SDK = 'js-holons';
const DEFAULT_SERVER_SDK = 'go-holons';
const DEFAULT_URI = 'stdio://';
const DEFAULT_MESSAGE = 'hello';
const DEFAULT_GO_BINARY = process.env.GO_BIN || 'go';
const DEFAULT_TIMEOUT_MS = 5000;

const ECHO_SERVICE_DEF = {
    Ping: {
        path: '/echo.v1.Echo/Ping',
        requestStream: false,
        responseStream: false,
        requestSerialize: serialize,
        requestDeserialize: deserialize,
        responseSerialize: serialize,
        responseDeserialize: deserialize,
        originalName: 'ping',
    },
};

function parseArgs(argv) {
    const out = {
        uri: DEFAULT_URI,
        sdk: DEFAULT_SDK,
        serverSDK: DEFAULT_SERVER_SDK,
        message: DEFAULT_MESSAGE,
        goBinary: DEFAULT_GO_BINARY,
        timeoutMs: DEFAULT_TIMEOUT_MS,
    };

    for (let i = 2; i < argv.length; i += 1) {
        const token = argv[i];
        if (token === '--sdk' && i + 1 < argv.length) {
            out.sdk = argv[i + 1];
            i += 1;
            continue;
        }
        if (token === '--server-sdk' && i + 1 < argv.length) {
            out.serverSDK = argv[i + 1];
            i += 1;
            continue;
        }
        if (token === '--message' && i + 1 < argv.length) {
            out.message = argv[i + 1];
            i += 1;
            continue;
        }
        if (token === '--go' && i + 1 < argv.length) {
            out.goBinary = argv[i + 1];
            i += 1;
            continue;
        }
        if (token === '--timeout-ms' && i + 1 < argv.length) {
            const parsed = Number(argv[i + 1]);
            if (Number.isFinite(parsed) && parsed > 0) {
                out.timeoutMs = Math.trunc(parsed);
            }
            i += 1;
            continue;
        }
        if (!token.startsWith('--') && !out._uriWasSet) {
            out.uri = token;
            out._uriWasSet = true;
        }
    }

    delete out._uriWasSet;
    return out;
}

function buildGoEchoServerArgs(options = {}) {
    const args = [
        'run',
        './cmd/echo-server',
        '--listen',
        'stdio://',
    ];

    if (options.serverSDK) {
        args.push('--sdk', options.serverSDK);
    }

    return args;
}

async function run(argv = process.argv, deps = {}) {
    const args = parseArgs(argv);
    const now = typeof deps.now === 'function' ? deps.now : Date.now;
    const grpcclientModule = deps.grpcclientModule || grpcclient;

    const EchoClient = grpc.makeGenericClientConstructor(ECHO_SERVICE_DEF, 'Echo', {});
    const dialOptions = {};
    if (args.uri === 'stdio://') {
        dialOptions.command = args.goBinary;
        dialOptions.args = buildGoEchoServerArgs(args);
        dialOptions.cwd = path.resolve(__dirname, '..', '..', 'go-holons');
        dialOptions.env = process.env;
    }

    const startedAt = now();
    const session = await grpcclientModule.dialURI(args.uri, EchoClient, dialOptions);

    try {
        const out = await invokePing(session.client, args.message, args.timeoutMs);
        if (!out || out.message !== args.message) {
            throw new Error(`unexpected echo response: ${JSON.stringify(out || {})}`);
        }

        return {
            status: 'pass',
            sdk: args.sdk,
            server_sdk: args.serverSDK,
            latency_ms: Math.max(0, now() - startedAt),
        };
    } finally {
        await session.close();
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

function serialize(value) {
    return Buffer.from(JSON.stringify(value || {}));
}

function deserialize(buffer) {
    try {
        return JSON.parse(Buffer.from(buffer).toString('utf8'));
    } catch {
        return {};
    }
}

function invokePing(client, message, timeoutMs) {
    return new Promise((resolve, reject) => {
        let completed = false;
        let call = null;
        let timer = null;

        const onDone = (err, value) => {
            if (completed) return;
            completed = true;
            clearTimeout(timer);
            if (err) {
                reject(err);
                return;
            }
            resolve(value || {});
        };

        call = client.Ping({ message }, onDone);

        timer = setTimeout(() => {
            if (call && typeof call.cancel === 'function') {
                call.cancel();
            }
            onDone(new Error(`ping timeout after ${timeoutMs}ms`));
        }, timeoutMs);

        timer.unref?.();
    });
}

module.exports = {
    ECHO_SERVICE_DEF,
    buildGoEchoServerArgs,
    parseArgs,
    run,
};

if (require.main === module) {
    main();
}
