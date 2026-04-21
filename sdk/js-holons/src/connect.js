'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { fileURLToPath } = require('node:url');

const grpc = require('@grpc/grpc-js');

const discover = require('./discover');
const grpcclient = require('./grpcclient');
const {
    LOCAL,
} = require('./discovery_types');

/**
 * @typedef {import('./discovery_types').HolonInfo} HolonInfo
 * @typedef {import('./discovery_types').HolonRef} HolonRef
 * @typedef {import('./discovery_types').ConnectResult} ConnectResult
 */

const started = new WeakMap();

async function connect(scope, expression, root, specifiers, timeout) {
    if (scope !== LOCAL) {
        return {
            channel: null,
            uid: '',
            origin: null,
            error: `scope ${scope} not supported`,
        };
    }

    const target = normalizeString(expression);
    if (!target) {
        return {
            channel: null,
            uid: '',
            origin: null,
            error: 'expression is required',
        };
    }

    try {
        const resolved = await discover.resolve(scope, target, root, specifiers, timeout);
        if (resolved.error) {
            return {
                channel: null,
                uid: '',
                origin: resolved.ref ? cloneRef(resolved.ref) : null,
                error: resolved.error,
            };
        }
        if (!resolved.ref) {
            return {
                channel: null,
                uid: '',
                origin: null,
                error: `holon ${JSON.stringify(target)} not found`,
            };
        }
        if (resolved.ref.error) {
            return {
                channel: null,
                uid: '',
                origin: cloneRef(resolved.ref),
                error: resolved.ref.error,
            };
        }

        return await connectResolved(resolved.ref, timeout);
    } catch (err) {
        return {
            channel: null,
            uid: '',
            origin: null,
            error: messageOf(err),
        };
    }
}

async function connectResolved(ref, timeout) {
    const origin = cloneRef(ref);

    try {
        const session = await dialRef(ref, timeout);
        if (session) {
            remember(session);
            return {
                channel: session.client,
                uid: '',
                origin,
                error: null,
            };
        }
    } catch (err) {
        const launched = await launchRef(ref, timeout).catch(() => null);
        if (launched) {
            remember(launched);
            return {
                channel: launched.client,
                uid: '',
                origin,
                error: null,
            };
        }

        return {
            channel: null,
            uid: '',
            origin,
            error: messageOf(err) || 'target unreachable',
        };
    }

    const launched = await launchRef(ref, timeout).catch((err) => ({
        error: messageOf(err),
    }));
    if (launched && launched.client) {
        remember(launched);
        return {
            channel: launched.client,
            uid: '',
            origin,
            error: null,
        };
    }

    return {
        channel: null,
        uid: '',
        origin,
        error: launched?.error || 'target unreachable',
    };
}

async function dialRef(ref, timeout) {
    const scheme = uriScheme(ref.url);
    if (!scheme || scheme === 'file') {
        return null;
    }
    if (scheme !== 'tcp' && scheme !== 'unix' && scheme !== 'ws' && scheme !== 'wss') {
        throw new Error(`unsupported transport ${JSON.stringify(scheme)}`);
    }

    const session = await grpcclient.dialURI(ref.url, grpc.Client, {
        credentials: grpc.credentials.createInsecure(),
    });
    await waitForReady(session.client, timeout);
    return session;
}

async function launchRef(ref, timeout) {
    const fsPath = pathFromFileURL(ref.url);
    if (!fsPath) {
        throw new Error('target unreachable');
    }

    const target = await resolveLaunchTarget(fsPath, ref.info);
    const args = [...target.args, 'serve', '--listen', 'stdio://'];
    const session = await grpcclient.dialStdio(target.commandPath, grpc.Client, {
        args,
        cwd: target.workingDirectory,
        env: process.env,
        credentials: grpc.credentials.createInsecure(),
    });
    await waitForReady(session.client, timeout);
    return session;
}

async function resolveLaunchTarget(fsPath, info) {
    if (isPackagePath(fsPath)) {
        return resolvePackageLaunchTarget(fsPath, info);
    }
    return resolveSourceLaunchTarget(fsPath, info);
}

async function resolvePackageLaunchTarget(dir, info) {
    const entrypoint = normalizeString(info?.entrypoint);
    if (entrypoint) {
        const packageBinary = path.join(dir, 'bin', packageArchDir(), path.basename(entrypoint));
        if (await fileExists(packageBinary)) {
            return {
                commandPath: packageBinary,
                args: [],
                workingDirectory: dir,
            };
        }

        const distEntrypoint = path.join(dir, 'dist', fromPosix(entrypoint));
        if (await fileExists(distEntrypoint)) {
            const runnerTarget = launchTargetForRunner(info?.runner, distEntrypoint, dir);
            if (runnerTarget) {
                return runnerTarget;
            }
        }
    }

    const fallbackBinary = await firstPackageBinary(dir);
    if (fallbackBinary) {
        return {
            commandPath: fallbackBinary,
            args: [],
            workingDirectory: dir,
        };
    }

    throw new Error(`holon ${JSON.stringify(info?.slug || path.basename(dir))} package is not runnable`);
}

async function resolveSourceLaunchTarget(dir, info) {
    const entrypoint = normalizeString(info?.entrypoint);
    if (!entrypoint) {
        throw new Error(`holon ${JSON.stringify(info?.slug || path.basename(dir))} has no entrypoint`);
    }

    if (path.isAbsolute(entrypoint) && await fileExists(entrypoint)) {
        return {
            commandPath: entrypoint,
            args: [],
            workingDirectory: dir,
        };
    }

    const sourceBuiltBinary = path.join(dir, '.op', 'build', 'bin', path.basename(entrypoint));
    if (await fileExists(sourceBuiltBinary)) {
        return {
            commandPath: sourceBuiltBinary,
            args: [],
            workingDirectory: dir,
        };
    }

    const slugBuiltBinary = path.join(
        dir,
        '.op',
        'build',
        `${normalizeString(info?.slug)}.holon`,
        'bin',
        packageArchDir(),
        path.basename(entrypoint),
    );
    if (await fileExists(slugBuiltBinary)) {
        return {
            commandPath: slugBuiltBinary,
            args: [],
            workingDirectory: dir,
        };
    }

    throw new Error(`holon ${JSON.stringify(info?.slug || path.basename(dir))} is not runnable`);
}

function launchTargetForRunner(runner, entrypoint, workingDirectory) {
    const normalizedRunner = normalizeString(runner).toLowerCase();
    if (!normalizedRunner || !entrypoint) {
        return null;
    }

    switch (normalizedRunner) {
    case 'go':
    case 'go-module':
        return {
            commandPath: process.env.GO_BIN || 'go',
            args: ['run', entrypoint],
            workingDirectory,
        };
    case 'node':
    case 'typescript':
    case 'npm':
        return {
            commandPath: 'node',
            args: [entrypoint],
            workingDirectory,
        };
    case 'python':
        return {
            commandPath: 'python3',
            args: [entrypoint],
            workingDirectory,
        };
    case 'ruby':
        return {
            commandPath: 'ruby',
            args: [entrypoint],
            workingDirectory,
        };
    case 'dart':
    case 'flutter':
        return {
            commandPath: 'dart',
            args: ['run', entrypoint],
            workingDirectory,
        };
    default:
        return null;
    }
}

function disconnect(result) {
    const channel = result && result.channel;
    if (!channel) {
        return;
    }

    const handle = started.get(channel);
    started.delete(channel);

    if (handle && typeof handle.close === 'function') {
        try {
            const promise = handle.close();
            if (promise && typeof promise.catch === 'function') {
                promise.catch(() => {});
            }
            return;
        } catch {
            // Fall through to direct close below.
        }
    }

    if (typeof channel.close === 'function') {
        try {
            channel.close();
        } catch {
            // Best-effort cleanup only.
        }
    }
}

function remember(session) {
    started.set(session.client, {
        close: session.close,
    });
}

function waitForReady(client, timeout) {
    return new Promise((resolve, reject) => {
        const deadline = timeout > 0
            ? Date.now() + timeout
            : Date.now() + (365 * 24 * 60 * 60 * 1000);

        client.waitForReady(deadline, (err) => {
            if (err) {
                reject(err);
                return;
            }
            resolve();
        });
    });
}

async function firstPackageBinary(dir) {
    const archRoot = path.join(dir, 'bin', packageArchDir());
    let entries;
    try {
        entries = await fs.promises.readdir(archRoot, { withFileTypes: true });
    } catch {
        return '';
    }

    const files = entries
        .filter((entry) => entry.isFile())
        .map((entry) => path.join(archRoot, entry.name))
        .sort();

    return files[0] || '';
}

function packageArchDir() {
    const platform = process.platform === 'win32' ? 'windows' : process.platform;
    const arch = ({
        x64: 'amd64',
        ia32: '386',
    })[process.arch] || process.arch;

    return `${platform}_${arch}`;
}

function pathFromFileURL(value) {
    const trimmed = normalizeString(value);
    if (!trimmed || !trimmed.toLowerCase().startsWith('file://')) {
        return '';
    }
    try {
        return path.resolve(fileURLToPath(trimmed));
    } catch {
        return '';
    }
}

function isPackagePath(value) {
    return path.basename(value).toLowerCase().endsWith('.holon');
}

function uriScheme(value) {
    const match = /^([A-Za-z][A-Za-z0-9+.-]*):\/\//.exec(normalizeString(value));
    return match ? match[1].toLowerCase() : '';
}

function normalizeString(value) {
    return String(value ?? '').trim();
}

function cloneRef(ref) {
    return {
        url: ref.url,
        info: ref.info ? {
            ...ref.info,
            identity: {
                ...ref.info.identity,
                aliases: ref.info.identity?.aliases ? [...ref.info.identity.aliases] : undefined,
            },
            architectures: [...ref.info.architectures],
        } : null,
        error: ref.error,
    };
}

async function fileExists(candidate) {
    try {
        const stat = await fs.promises.stat(candidate);
        return stat.isFile();
    } catch {
        return false;
    }
}

function messageOf(err) {
    if (err && typeof err === 'object' && typeof err.message === 'string' && err.message.trim()) {
        return err.message.trim();
    }
    return String(err);
}

function fromPosix(value) {
    return String(value || '').split('/').join(path.sep);
}

module.exports = {
    connect,
    disconnect,
    _internal: {
        packageArchDir,
        started,
    },
};
