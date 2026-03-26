'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const readline = require('node:readline');
const { spawn } = require('node:child_process');

const connectModule = require('../src/connect');
const { useStaticDescribeEnv } = require('./helpers/static_describe');

const ECHO_SERVER = path.join(__dirname, '..', 'cmd', 'echo-server.js');

describe('connect', { concurrency: 1 }, () => {
    it('dials direct tcp targets', async (t) => {
        useStaticDescribeEnv(t);
        const child = spawn(process.execPath, [ECHO_SERVER, '--listen', 'tcp://127.0.0.1:0'], {
            stdio: ['ignore', 'pipe', 'pipe'],
        });
        t.after(async () => terminateChildProcess(child));

        const uri = await waitForAdvertisedURI(child);
        const client = await connectModule.connect(uri);
        t.after(async () => connectModule.disconnect(client));

        const out = await invokePing(client, 'direct-js');
        assert.equal(out.message, 'direct-js');
        assert.equal(out.sdk, 'js-holons');
    });

    it('starts slug targets ephemerally and stops them on disconnect', async (t) => {
        useStaticDescribeEnv(t);
        const fixture = await createHolonFixture(t, 'Connect', 'Ephemeral');
        useHolonRoot(t, fixture.root);

        const client = await connectModule.connect(fixture.slug);
        const pid = await waitForPidFile(fixture.pidFile);
        const args = await waitForArgLines(fixture.argsFile);

        const out = await invokePing(client, 'ephemeral-js');
        assert.equal(out.message, 'ephemeral-js');
        assert.deepEqual(args, ['serve', '--listen', 'stdio://']);

        await connectModule.disconnect(client);
        await waitForPidExit(pid);

        await assert.rejects(fs.promises.stat(fixture.portFile), /ENOENT/);
    });

    it('keeps stdio connections alive long enough for the first delayed RPC', async (t) => {
        useStaticDescribeEnv(t);
        const fixture = await createHolonFixture(t, 'Connect', 'Delayed');
        useHolonRoot(t, fixture.root);

        const client = await connectModule.connect(fixture.slug);
        const pid = await waitForPidFile(fixture.pidFile);

        await sleep(600);

        const out = await invokePing(client, 'delayed-js');
        assert.equal(out.message, 'delayed-js');

        await connectModule.disconnect(client);
        await waitForPidExit(pid);
    });

    it('writes a port file in persistent mode and reuses it', async (t) => {
        useStaticDescribeEnv(t);
        const fixture = await createHolonFixture(t, 'Connect', 'Persistent');
        useHolonRoot(t, fixture.root);

        const client = await connectModule.connect(fixture.slug, { timeout: 5000, start: true, transport: 'tcp' });
        const pid = await waitForPidFile(fixture.pidFile);

        const portTarget = (await fs.promises.readFile(fixture.portFile, 'utf8')).trim();
        assert.match(portTarget, /^tcp:\/\/127\.0\.0\.1:\d+$/);

        await connectModule.disconnect(client);
        assert.equal(pidExists(pid), true);

        const reused = await connectModule.connect(fixture.slug);
        t.after(async () => connectModule.disconnect(reused));
        assert.equal(connectModule._internal.started.has(reused), false);

        const out = await invokePing(reused, 'persistent-js');
        assert.equal(out.message, 'persistent-js');

        await terminatePid(pid);
        await waitForPidExit(pid);
    });

    it('writes a unix port file in persistent mode and reuses it', async (t) => {
        useStaticDescribeEnv(t);
        const fixture = await createHolonFixture(t, 'Connect', 'Unix');
        useHolonRoot(t, fixture.root);

        const client = await connectModule.connect(fixture.slug, { timeout: 5000, start: true, transport: 'unix' });
        const pid = await waitForPidFile(fixture.pidFile);

        const portTarget = (await fs.promises.readFile(fixture.portFile, 'utf8')).trim();
        assert.match(portTarget, /^unix:\/\/\/tmp\/holons-/);

        await connectModule.disconnect(client);
        assert.equal(pidExists(pid), true);

        const reused = await connectModule.connect(fixture.slug);
        t.after(async () => connectModule.disconnect(reused));
        assert.equal(connectModule._internal.started.has(reused), false);

        const out = await invokePing(reused, 'unix-js');
        assert.equal(out.message, 'unix-js');

        await terminatePid(pid);
        await waitForPidExit(pid);
    });

    it('reuses an existing port file without starting a new process', async (t) => {
        useStaticDescribeEnv(t);
        const fixture = await createHolonFixture(t, 'Connect', 'Reuse');
        useHolonRoot(t, fixture.root);

        const child = spawn(fixture.binaryPath, ['serve', '--listen', 'tcp://127.0.0.1:0'], {
            stdio: ['ignore', 'pipe', 'pipe'],
        });
        t.after(async () => terminateChildProcess(child));

        const uri = await waitForAdvertisedURI(child);
        const pid = await waitForPidFile(fixture.pidFile);
        await fs.promises.mkdir(path.dirname(fixture.portFile), { recursive: true });
        await fs.promises.writeFile(fixture.portFile, `${uri}\n`, 'utf8');

        const client = await connectModule.connect(fixture.slug);
        t.after(async () => connectModule.disconnect(client));

        assert.equal(connectModule._internal.started.has(client), false);
        const out = await invokePing(client, 'reuse-js');
        assert.equal(out.message, 'reuse-js');
        assert.equal(pidExists(pid), true);
    });

    it('removes stale port files and starts fresh', async (t) => {
        useStaticDescribeEnv(t);
        const fixture = await createHolonFixture(t, 'Connect', 'Stale');
        useHolonRoot(t, fixture.root);

        const stalePort = await reserveLoopbackPort();
        await fs.promises.mkdir(path.dirname(fixture.portFile), { recursive: true });
        await fs.promises.writeFile(fixture.portFile, `tcp://127.0.0.1:${stalePort}\n`, 'utf8');

        const client = await connectModule.connect(fixture.slug);
        const pid = await waitForPidFile(fixture.pidFile);

        const out = await invokePing(client, 'stale-js');
        assert.equal(out.message, 'stale-js');

        await assert.rejects(fs.promises.stat(fixture.portFile), /ENOENT/);

        await connectModule.disconnect(client);
        await waitForPidExit(pid);
    });
});

function invokePing(client, message) {
    return new Promise((resolve, reject) => {
        client.makeUnaryRequest(
            '/echo.v1.Echo/Ping',
            serialize,
            deserialize,
            { message },
            (err, response) => {
                if (err) {
                    reject(err);
                    return;
                }
                resolve(response);
            },
        );
    });
}

function serialize(value) {
    return Buffer.from(JSON.stringify(value || {}));
}

function deserialize(buffer) {
    return JSON.parse(Buffer.from(buffer).toString('utf8'));
}

async function createHolonFixture(t, givenName, familyName) {
    const root = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'js-holons-connect-'));
    const slug = `${givenName}-${familyName}`.toLowerCase();
    const holonDir = path.join(root, 'holons', slug);
    const binaryDir = path.join(holonDir, '.op', 'build', 'bin');
    const binaryPath = path.join(binaryDir, 'echo-wrapper');
    const pidFile = path.join(root, `${slug}.pid`);
    const argsFile = path.join(root, `${slug}.args`);
    const portFile = path.join(root, '.op', 'run', `${slug}.port`);

    await fs.promises.mkdir(binaryDir, { recursive: true });
    await fs.promises.writeFile(binaryPath, wrapperScript(pidFile, argsFile), { mode: 0o755 });
    await fs.promises.writeFile(path.join(holonDir, 'holon.proto'), [
        'syntax = "proto3";',
        '',
        'package test.v1;',
        '',
        'option (holons.v1.manifest) = {',
        '  identity: {',
        `    uuid: "${slug}-uuid"`,
        `    given_name: "${givenName}"`,
        `    family_name: "${familyName}"`,
        '    composer: "connect-test"',
        '  }',
        '  kind: "service"',
        '  build: {',
        '    runner: "node"',
        '    main: "cmd/echo-server.js"',
        '  }',
        '  artifacts: {',
        '    binary: "echo-wrapper"',
        '  }',
        '};',
        '',
    ].join('\n'));

    t.after(async () => {
        await fs.promises.rm(root, { recursive: true, force: true });
    });

    return { root, slug, binaryPath, pidFile, argsFile, portFile };
}

function wrapperScript(pidFile, argsFile) {
    return [
        '#!/bin/sh',
        `printf '%s\n' "$$" > ${shellQuote(pidFile)}`,
        `: > ${shellQuote(argsFile)}`,
        `for arg in "$@"; do printf '%s\n' "$arg" >> ${shellQuote(argsFile)}; done`,
        `exec ${shellQuote(process.execPath)} ${shellQuote(ECHO_SERVER)} "$@"`,
        '',
    ].join('\n');
}

function shellQuote(value) {
    return `'${String(value).replace(/'/g, `'\"'\"'`)}'`;
}

function useHolonRoot(t, root) {
    const previousCwd = process.cwd();
    const previousOpPath = process.env.OPPATH;
    const previousOpBin = process.env.OPBIN;

    process.chdir(root);
    process.env.OPPATH = path.join(root, '.op-home');
    process.env.OPBIN = path.join(root, '.op-bin');

    t.after(() => {
        process.chdir(previousCwd);
        if (previousOpPath === undefined) {
            delete process.env.OPPATH;
        } else {
            process.env.OPPATH = previousOpPath;
        }
        if (previousOpBin === undefined) {
            delete process.env.OPBIN;
        } else {
            process.env.OPBIN = previousOpBin;
        }
    });
}

function waitForAdvertisedURI(child, timeoutMs = 5000) {
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
            finish(new Error('timed out waiting for advertised URI'));
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
        child.stderr.on('data', (chunk) => stderrChunks.push(Buffer.from(chunk)));

        child.once('error', (err) => finish(err));
        child.once('exit', (code, signal) => {
            const stderrText = Buffer.concat(stderrChunks).toString('utf8').trim();
            finish(new Error(`child exited early (${signal || code || 'unknown'}): ${stderrText}`));
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

async function waitForArgLines(argsFile, timeoutMs = 5000) {
    const deadline = Date.now() + timeoutMs;

    while (Date.now() < deadline) {
        try {
            const lines = (await fs.promises.readFile(argsFile, 'utf8'))
                .split(/\r?\n/)
                .filter(Boolean);
            if (lines.length > 0) {
                return lines;
            }
        } catch {
            // Ignore transient file creation/read races while the wrapper starts.
        }
        await new Promise((resolve) => setTimeout(resolve, 25));
    }

    throw new Error(`timed out waiting for wrapper args in ${argsFile}`);
}

function firstURI(line) {
    for (const field of String(line || '').split(/\s+/)) {
        const trimmed = field.trim().replace(/^["'([{]+|["')\]}.,]+$/g, '');
        if (trimmed.startsWith('tcp://') || trimmed.startsWith('unix://')) {
            return trimmed;
        }
    }
    return '';
}

async function waitForPidFile(pidFile, timeoutMs = 5000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
        try {
            const raw = (await fs.promises.readFile(pidFile, 'utf8')).trim();
            const pid = Number(raw);
            if (Number.isInteger(pid) && pid > 0) {
                return pid;
            }
        } catch {}
        await sleep(25);
    }
    throw new Error(`timed out waiting for pid file ${pidFile}`);
}

function pidExists(pid) {
    try {
        process.kill(pid, 0);
        return true;
    } catch (err) {
        if (err && err.code === 'EPERM') {
            return true;
        }
        return false;
    }
}

async function waitForPidExit(pid, timeoutMs = 2000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
        if (!pidExists(pid)) {
            return;
        }
        await sleep(25);
    }
    throw new Error(`process ${pid} did not exit`);
}

async function terminatePid(pid) {
    if (!pidExists(pid)) {
        return;
    }

    process.kill(pid, 'SIGTERM');
    try {
        await waitForPidExit(pid, 2000);
        return;
    } catch {}

    if (pidExists(pid)) {
        process.kill(pid, 'SIGKILL');
    }
}

async function terminateChildProcess(child) {
    if (!child || child.exitCode !== null) return;
    child.kill('SIGTERM');

    const exited = await Promise.race([
        onceExit(child),
        sleep(1000).then(() => false),
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

function reserveLoopbackPort() {
    return new Promise((resolve, reject) => {
        const server = net.createServer();
        server.once('error', reject);
        server.listen(0, '127.0.0.1', () => {
            const address = server.address();
            if (!address || typeof address === 'string') {
                server.close(() => reject(new Error('failed to reserve loopback port')));
                return;
            }
            server.close((err) => {
                if (err) {
                    reject(err);
                    return;
                }
                resolve(address.port);
            });
        });
    });
}

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
