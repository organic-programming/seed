'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const ECHO_SERVER = path.join(__dirname, '..', '..', 'cmd', 'echo-server.js');

function useRuntimeFixture(t) {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'js-holons-discovery-'));
    const opHome = path.join(root, '.op-home');
    const opBin = path.join(root, '.op-bin');

    const previousOpPath = process.env.OPPATH;
    const previousOpBin = process.env.OPBIN;
    process.env.OPPATH = opHome;
    process.env.OPBIN = opBin;
    fs.mkdirSync(path.join(opHome, 'cache'), { recursive: true });
    fs.mkdirSync(opBin, { recursive: true });

    t.after(() => {
        fs.rmSync(root, { recursive: true, force: true });
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

    return { root, opHome, opBin };
}

function useCwd(t, cwd) {
    const previous = process.cwd();
    process.chdir(cwd);
    t.after(() => {
        process.chdir(previous);
    });
}

function writePackageHolon(dir, seed = {}) {
    fs.mkdirSync(dir, { recursive: true });

    const slug = seed.slug || slugFromNames(seed.givenName, seed.familyName);
    const payload = {
        schema: 'holon-package/v1',
        slug,
        uuid: seed.uuid || `${slug}-uuid`,
        identity: {
            given_name: seed.givenName || 'Test',
            family_name: seed.familyName || 'Holon',
        },
        lang: seed.lang || 'js',
        runner: seed.runner || 'node',
        status: seed.status || 'draft',
        kind: seed.kind || 'service',
        transport: seed.transport || '',
        entrypoint: seed.entrypoint || 'echo-wrapper',
        architectures: seed.architectures || [],
        has_dist: Boolean(seed.hasDist),
        has_source: Boolean(seed.hasSource),
    };

    if (seed.motto) {
        payload.identity.motto = seed.motto;
    }
    if (seed.aliases) {
        payload.identity.aliases = seed.aliases;
    }

    fs.writeFileSync(
        path.join(dir, '.holon.json'),
        `${JSON.stringify(payload, null, 2)}\n`,
        'utf8',
    );

    return payload;
}

function writeProbeablePackage(dir, seed = {}) {
    const slug = seed.slug || slugFromNames(seed.givenName, seed.familyName);
    const entrypoint = seed.entrypoint || 'echo-wrapper';
    const binaryDir = path.join(dir, 'bin', packageArchDir());
    const binaryPath = path.join(binaryDir, path.basename(entrypoint));

    fs.mkdirSync(binaryDir, { recursive: true });
    fs.writeFileSync(binaryPath, wrapperScript(seed.pidFile, seed.argsFile), { mode: 0o755 });

    if (seed.includeJSON !== false) {
        writePackageHolon(dir, {
            ...seed,
            slug,
            entrypoint,
        });
    }

    return { slug, binaryPath };
}

function packageArchDir() {
    const platform = process.platform === 'win32' ? 'windows' : process.platform;
    const arch = ({
        x64: 'amd64',
        ia32: '386',
    })[process.arch] || process.arch;

    return `${platform}_${arch}`;
}

function wrapperScript(pidFile, argsFile) {
    const lines = ['#!/bin/sh'];
    if (pidFile) {
        lines.push(`printf '%s\n' "$$" > ${shellQuote(pidFile)}`);
    }
    if (argsFile) {
        lines.push(`: > ${shellQuote(argsFile)}`);
        lines.push(`for arg in "$@"; do printf '%s\n' "$arg" >> ${shellQuote(argsFile)}; done`);
    }
    lines.push(`exec ${shellQuote(process.execPath)} ${shellQuote(ECHO_SERVER)} "$@"`);
    lines.push('');
    return lines.join('\n');
}

function shellQuote(value) {
    return `'${String(value).replace(/'/g, `'\"'\"'`)}'`;
}

function slugFromNames(givenName, familyName) {
    return `${String(givenName || 'Test')}-${String(familyName || 'Holon')}`
        .trim()
        .toLowerCase()
        .replace(/\s+/g, '-')
        .replace(/^-+|-+$/g, '');
}

async function waitForPidFile(pidFile, timeoutMs = 5000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
        try {
            const raw = fs.readFileSync(pidFile, 'utf8').trim();
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
        return err && err.code === 'EPERM';
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

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

module.exports = {
    ECHO_SERVER,
    invokePing,
    packageArchDir,
    useCwd,
    useRuntimeFixture,
    waitForPidExit,
    waitForPidFile,
    writePackageHolon,
    writeProbeablePackage,
};
