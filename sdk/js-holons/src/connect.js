'use strict';

const fs = require('node:fs');
const path = require('node:path');
const readline = require('node:readline');
const { spawn } = require('node:child_process');

const grpc = require('@grpc/grpc-js');

const discover = require('./discover');
const transport = require('./transport');
const grpcclient = require('./grpcclient');

const DEFAULT_TIMEOUT_MS = 5000;
const started = new WeakMap();

async function connect(target, opts) {
    const trimmed = String(target || '').trim();

    if (!trimmed) {
        throw new Error('target is required');
    }

    const options = normalizeOptions(opts);
    const ephemeral = opts == null || options.transport === 'stdio';

    if (isDirectTarget(trimmed)) {
        return dialReady(normalizeDialTarget(trimmed), options.timeout);
    }

    const entry = await discover.findBySlug(trimmed);
    if (!entry) {
        throw new Error(`holon "${trimmed}" not found`);
    }

    const portFile = options.port_file || defaultPortFilePath(entry.slug);
    const reusable = await usablePortFile(portFile, options.timeout);
    if (reusable) {
        return reusable;
    }
    if (!options.start) {
        throw new Error(`holon "${trimmed}" is not running`);
    }

    const binaryPath = await resolveBinaryPath(entry);
    if (options.transport === 'stdio') {
        const session = await grpcclient.dialStdio(binaryPath, grpc.Client, {
            credentials: grpc.credentials.createInsecure(),
        });

        await waitForReady(session.client, options.timeout);
        started.set(session.client, {
            ephemeral: true,
            close: session.close,
        });
        return session.client;
    }

    const startup = options.transport === 'unix'
        ? await startUnixHolon(binaryPath, entry.slug, portFile, options.timeout)
        : await startTCPHolon(binaryPath, options.timeout);
    const { client, child, target: advertisedTarget } = startup;

    if (!ephemeral) {
        try {
            await writePortFile(portFile, advertisedTarget);
        } catch (err) {
            await stopChild(child);
            client.close();
            throw err;
        }
    }

    started.set(client, {
        ephemeral,
        close: async () => {
            client.close();
            if (ephemeral) {
                await stopChild(child);
            }
        },
    });
    return client;
}

async function disconnect(client) {
    if (!client) return;

    const handle = started.get(client);
    started.delete(client);

    if (handle && typeof handle.close === 'function') {
        await handle.close();
        return;
    }

    if (typeof client.close === 'function') {
        client.close();
    }
}

function normalizeOptions(opts = {}) {
    const timeout = Number.isFinite(opts.timeout) && opts.timeout > 0 ? opts.timeout : DEFAULT_TIMEOUT_MS;
    const transportName = String(opts.transport || 'stdio').trim().toLowerCase();
    if (transportName !== 'tcp' && transportName !== 'stdio' && transportName !== 'unix') {
        throw new Error(`unsupported transport ${JSON.stringify(opts.transport)}`);
    }

    return {
        timeout,
        transport: transportName,
        start: opts.start !== false,
        port_file: typeof opts.port_file === 'string' ? opts.port_file.trim() : '',
    };
}

async function dialReady(target, timeoutMs) {
    const client = new grpc.Client(target, grpc.credentials.createInsecure());
    try {
        await waitForReady(client, timeoutMs);
        return client;
    } catch (err) {
        client.close();
        throw err;
    }
}

function waitForReady(client, timeoutMs) {
    return new Promise((resolve, reject) => {
        client.waitForReady(Date.now() + timeoutMs, (err) => {
            if (err) {
                reject(err);
                return;
            }
            resolve();
        });
    });
}

async function usablePortFile(portFile, timeoutMs) {
    let data;
    try {
        data = await fs.promises.readFile(portFile, 'utf8');
    } catch {
        return null;
    }

    const rawTarget = String(data || '').trim();
    if (!rawTarget) {
        await fs.promises.rm(portFile, { force: true });
        return null;
    }

    try {
        const client = await dialReady(normalizeDialTarget(rawTarget), Math.max(250, Math.min(timeoutMs / 4, 1000)));
        return client;
    } catch {
        await fs.promises.rm(portFile, { force: true });
        return null;
    }
}

async function startTCPHolon(binaryPath, timeoutMs) {
    const child = spawn(binaryPath, ['serve', '--listen', 'tcp://127.0.0.1:0'], {
        stdio: ['ignore', 'pipe', 'pipe'],
    });

    try {
        const advertised = await waitForAdvertisedURI(child, timeoutMs);
        const client = await dialReady(normalizeDialTarget(advertised), timeoutMs);
        return {
            client,
            child,
            target: advertised,
        };
    } catch (err) {
        await stopChild(child);
        throw err;
    }
}

async function startUnixHolon(binaryPath, slug, portFile, timeoutMs) {
    const target = defaultUnixSocketURI(slug, portFile);
    const socketPath = target.slice('unix://'.length);
    const child = spawn(binaryPath, ['serve', '--listen', target], {
        stdio: ['ignore', 'ignore', 'pipe'],
    });

    try {
        await waitForUnixSocket(child, socketPath, timeoutMs);
        const client = await dialReady(normalizeDialTarget(target), timeoutMs);
        return { client, child, target };
    } catch (err) {
        await stopChild(child);
        throw err;
    }
}

async function waitForUnixSocket(child, socketPath, timeoutMs) {
    const deadline = Date.now() + timeoutMs;
    const stderrChunks = [];

    child.stderr.on('data', (chunk) => {
        stderrChunks.push(Buffer.from(chunk));
    });

    while (Date.now() < deadline) {
        if (child.exitCode !== null) {
            const stderrText = Buffer.concat(stderrChunks).toString('utf8').trim();
            const details = stderrText ? `: ${stderrText}` : '';
            throw new Error(`holon exited before binding unix socket (${child.exitCode})${details}`);
        }

        try {
            const stat = await fs.promises.stat(socketPath);
            if (stat.isSocket?.() ?? true) {
                return;
            }
        } catch {}

        await sleep(20);
    }

    const stderrText = Buffer.concat(stderrChunks).toString('utf8').trim();
    const details = stderrText ? `: ${stderrText}` : '';
    throw new Error(`timed out waiting for unix holon startup${details}`);
}

function waitForAdvertisedURI(child, timeoutMs) {
    return new Promise((resolve, reject) => {
        const stderrChunks = [];
        let settled = false;

        const finish = (err, uri) => {
            if (settled) return;
            settled = true;
            cleanup();
            if (err) {
                reject(err);
                return;
            }
            resolve(uri);
        };

        const timer = setTimeout(() => {
            finish(new Error('timed out waiting for holon startup'));
        }, timeoutMs);
        timer.unref?.();

        const onLine = (line) => {
            const uri = firstURI(line);
            if (uri) {
                finish(null, uri);
            }
        };

        const stdoutRL = readline.createInterface({ input: child.stdout });
        const stderrRL = readline.createInterface({ input: child.stderr });
        stdoutRL.on('line', onLine);
        stderrRL.on('line', onLine);
        child.stderr.on('data', (chunk) => {
            stderrChunks.push(Buffer.from(chunk));
        });

        child.once('error', (err) => {
            finish(err);
        });
        child.once('exit', (code, signal) => {
            const stderrText = Buffer.concat(stderrChunks).toString('utf8').trim();
            const details = stderrText ? `: ${stderrText}` : '';
            finish(new Error(`holon exited before advertising an address (${signal || code || 'unknown'})${details}`));
        });

        function cleanup() {
            clearTimeout(timer);
            stdoutRL.close();
            stderrRL.close();
            child.removeAllListeners('error');
            child.removeAllListeners('exit');
        }
    });
}

async function resolveBinaryPath(entry) {
    if (!entry.manifest) {
        throw new Error(`holon "${entry.slug}" has no manifest`);
    }

    const binary = String(entry.manifest.artifacts?.binary || '').trim();
    if (!binary) {
        throw new Error(`holon "${entry.slug}" has no artifacts.binary`);
    }

    if (path.isAbsolute(binary) && await fileExists(binary)) {
        return binary;
    }

    const candidate = path.join(entry.dir, '.op', 'build', 'bin', path.basename(binary));
    if (await fileExists(candidate)) {
        return candidate;
    }

    return binary;
}

function defaultPortFilePath(slug) {
    return path.join(process.cwd(), '.op', 'run', `${slug}.port`);
}

function defaultUnixSocketURI(slug, portFile) {
    const label = socketLabel(slug);
    const hash = fnv1a64(String(portFile));
    return `unix:///tmp/holons-${label}-${hash.toString(16).padStart(12, '0').slice(-12)}.sock`;
}

function socketLabel(slug) {
    let label = '';
    let lastDash = false;

    for (const ch of String(slug || '').trim().toLowerCase()) {
        const code = ch.charCodeAt(0);
        if ((code >= 97 && code <= 122) || (code >= 48 && code <= 57)) {
            label += ch;
            lastDash = false;
        } else if ((ch === '-' || ch === '_') && label && !lastDash) {
            label += '-';
            lastDash = true;
        }

        if (label.length >= 24) {
            break;
        }
    }

    label = label.replace(/^-+|-+$/g, '');
    return label || 'socket';
}

function fnv1a64(text) {
    let hash = 0xcbf29ce484222325n;
    for (const byte of Buffer.from(String(text), 'utf8')) {
        hash ^= BigInt(byte);
        hash = (hash * 0x100000001b3n) & 0xffffffffffffffffn;
    }
    return hash & 0xffffffffffffn;
}

async function writePortFile(portFile, uri) {
    await fs.promises.mkdir(path.dirname(portFile), { recursive: true });
    await fs.promises.writeFile(portFile, `${String(uri).trim()}\n`, 'utf8');
}

async function stopChild(child) {
    if (!child || child.exitCode !== null) {
        return;
    }

    child.kill('SIGTERM');
    const exited = await Promise.race([
        onceExit(child),
        sleep(2000).then(() => false),
    ]);

    if (exited) {
        return;
    }

    if (child.exitCode === null) {
        child.kill('SIGKILL');
    }
    await onceExit(child);
}

function onceExit(child) {
    return new Promise((resolve) => {
        if (!child || child.exitCode !== null) {
            resolve(true);
            return;
        }
        child.once('exit', () => resolve(true));
    });
}

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

async function fileExists(candidate) {
    try {
        const stat = await fs.promises.stat(candidate);
        return stat.isFile();
    } catch {
        return false;
    }
}

function isDirectTarget(target) {
    return target.includes('://') || target.includes(':');
}

function normalizeDialTarget(target) {
    if (!target.includes('://')) {
        return target;
    }

    const parsed = transport.parseURI(target);
    if (parsed.scheme === 'tcp') {
        const host = !parsed.host || parsed.host === '0.0.0.0' ? '127.0.0.1' : parsed.host;
        return `${host}:${parsed.port}`;
    }
    if (parsed.scheme === 'unix') {
        return `unix://${parsed.path}`;
    }
    return target;
}

function firstURI(line) {
    for (const field of String(line || '').split(/\s+/)) {
        const trimmed = field.trim().replace(/^["'([{]+|["')\]}.,]+$/g, '');
        if (
            trimmed.startsWith('tcp://')
            || trimmed.startsWith('unix://')
            || trimmed.startsWith('stdio://')
            || trimmed.startsWith('ws://')
            || trimmed.startsWith('wss://')
        ) {
            return trimmed;
        }
    }
    return '';
}

module.exports = {
    connect,
    disconnect,
    _internal: {
        started,
        stopChild,
    },
};
