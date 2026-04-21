#!/usr/bin/env node

import { spawn } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const DEFAULT_URI = "stdio://";
const DEFAULT_SDK = "js-web-holons";
const DEFAULT_SERVER_SDK = "go-holons";
const DEFAULT_MESSAGE = "hello";
const DEFAULT_TIMEOUT_MS = 5000;
const DEFAULT_GO_BINARY = process.env.GO_BIN || "go";
const DEFAULT_GOCACHE = "/tmp/go-cache";

const MODULE_DIR = path.dirname(fileURLToPath(import.meta.url));

export function parseArgs(argv = process.argv) {
    const out = {
        uri: DEFAULT_URI,
        sdk: DEFAULT_SDK,
        serverSDK: DEFAULT_SERVER_SDK,
        message: DEFAULT_MESSAGE,
        timeoutMs: DEFAULT_TIMEOUT_MS,
        goBinary: DEFAULT_GO_BINARY,
    };

    let uriSet = false;

    for (let i = 2; i < argv.length; i += 1) {
        const token = argv[i];

        if (token === "--sdk") {
            out.sdk = requireValue(argv, i, "--sdk");
            i += 1;
            continue;
        }
        if (token === "--server-sdk") {
            out.serverSDK = requireValue(argv, i, "--server-sdk");
            i += 1;
            continue;
        }
        if (token === "--message") {
            out.message = requireValue(argv, i, "--message");
            i += 1;
            continue;
        }
        if (token === "--go") {
            out.goBinary = requireValue(argv, i, "--go");
            i += 1;
            continue;
        }
        if (token === "--timeout-ms") {
            const raw = requireValue(argv, i, "--timeout-ms");
            const parsed = Number(raw);
            if (!Number.isInteger(parsed) || parsed <= 0) {
                throw new Error("--timeout-ms must be a positive integer");
            }
            out.timeoutMs = parsed;
            i += 1;
            continue;
        }
        if (token.startsWith("--")) {
            throw new Error(`unknown flag: ${token}`);
        }
        if (uriSet) {
            throw new Error(`unexpected argument: ${token}`);
        }

        out.uri = normalizeURI(token);
        uriSet = true;
    }

    out.uri = normalizeURI(out.uri);
    return out;
}

export function buildInvocation(args, options = {}) {
    const goHolonsDir = options.goHolonsDir || path.resolve(MODULE_DIR, "..", "..", "go-holons");
    const helperPath = options.helperPath || path.resolve(MODULE_DIR, "echo-client-go", "main.go");
    const env = {
        ...process.env,
        ...(options.env || {}),
    };

    if (!env.GOCACHE) {
        env.GOCACHE = DEFAULT_GOCACHE;
    }

    return {
        command: args.goBinary,
        args: [
            "run",
            helperPath,
            "--sdk",
            args.sdk,
            "--server-sdk",
            args.serverSDK,
            "--message",
            args.message,
            "--timeout-ms",
            String(args.timeoutMs),
            "--go",
            args.goBinary,
            args.uri,
        ],
        cwd: goHolonsDir,
        env,
    };
}

export async function run(argv = process.argv, options = {}) {
    const parsed = parseArgs(argv);
    const invocation = buildInvocation(parsed, options.paths || {});
    const result = await runInvocation(invocation, options.spawnFn || spawn);

    if (result.code !== 0) {
        const message = result.stderr.trim() || `echo-client failed with exit code ${result.code}`;
        throw new Error(message);
    }

    const payload = lastNonEmptyLine(result.stdout);
    if (!payload) {
        throw new Error("echo-client returned empty stdout");
    }

    let decoded;
    try {
        decoded = JSON.parse(payload);
    } catch {
        throw new Error(`echo-client returned non-JSON stdout: ${payload}`);
    }

    return decoded;
}

export function runInvocation(invocation, spawnFn = spawn) {
    return new Promise((resolve, reject) => {
        const child = spawnFn(invocation.command, invocation.args, {
            cwd: invocation.cwd,
            env: invocation.env,
            stdio: ["ignore", "pipe", "pipe"],
        });

        let stdout = "";
        let stderr = "";

        child.stdout.on("data", (chunk) => {
            stdout += chunk.toString();
        });
        child.stderr.on("data", (chunk) => {
            stderr += chunk.toString();
        });
        child.on("error", reject);
        child.on("close", (code) => {
            resolve({
                code: code ?? 1,
                stdout,
                stderr,
            });
        });
    });
}

function requireValue(argv, index, flag) {
    const value = argv[index + 1];
    if (typeof value !== "string" || value === "") {
        throw new Error(`missing value for ${flag}`);
    }
    return value;
}

function normalizeURI(value) {
    if (value === "stdio") {
        return "stdio://";
    }
    return value;
}

function lastNonEmptyLine(text) {
    const lines = String(text || "").trim().split(/\r?\n/).filter(Boolean);
    if (lines.length === 0) {
        return "";
    }
    return lines[lines.length - 1];
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

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
    main();
}
