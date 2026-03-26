'use strict';

const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');

const {
    PROTO_MANIFEST_FILE_NAME,
    resolveProtoFile,
    slugForIdentity,
} = require('./identity');

/**
 * @typedef {Object} HolonBuild
 * @property {string} runner
 * @property {string} main
 */

/**
 * @typedef {Object} HolonArtifacts
 * @property {string} binary
 * @property {string} primary
 */

/**
 * @typedef {Object} HolonManifest
 * @property {string} kind
 * @property {HolonBuild} build
 * @property {HolonArtifacts} artifacts
 */

/**
 * @typedef {Object} HolonEntry
 * @property {string} slug
 * @property {string} uuid
 * @property {string} dir
 * @property {string} relative_path
 * @property {string} origin
 * @property {import('./identity').HolonIdentity} identity
 * @property {HolonManifest|null} manifest
 */

async function discover(root) {
    return discoverInRoot(root, 'local');
}

async function discoverLocal() {
    return discover(process.cwd());
}

async function discoverAll() {
    const roots = [
        { root: process.cwd(), origin: 'local' },
        { root: opbin(), origin: '$OPBIN' },
        { root: cacheDir(), origin: 'cache' },
    ];

    const seen = new Set();
    const entries = [];
    for (const spec of roots) {
        const discovered = await discoverInRoot(spec.root, spec.origin);
        for (const entry of discovered) {
            const key = entry.uuid.trim() || entry.dir;
            if (seen.has(key)) {
                continue;
            }
            seen.add(key);
            entries.push(entry);
        }
    }

    return entries;
}

async function findBySlug(slug) {
    const needle = String(slug || '').trim();
    if (!needle) {
        return null;
    }

    let match = null;
    for (const entry of await discoverAll()) {
        if (entry.slug !== needle) {
            continue;
        }
        if (match && match.uuid !== entry.uuid) {
            throw new Error(`ambiguous holon "${needle}"`);
        }
        match = entry;
    }
    return match;
}

async function findByUUID(prefix) {
    const needle = String(prefix || '').trim();
    if (!needle) {
        return null;
    }

    let match = null;
    for (const entry of await discoverAll()) {
        if (!entry.uuid.startsWith(needle)) {
            continue;
        }
        if (match && match.uuid !== entry.uuid) {
            throw new Error(`ambiguous UUID prefix "${needle}"`);
        }
        match = entry;
    }
    return match;
}

async function discoverInRoot(root, origin) {
    const resolvedRoot = path.resolve(String(root || '').trim() || process.cwd());
    let stat;
    try {
        stat = await fs.promises.stat(resolvedRoot);
    } catch (err) {
        if (err && err.code === 'ENOENT') {
            return [];
        }
        throw err;
    }
    if (!stat.isDirectory()) {
        return [];
    }

    const entriesByKey = new Map();
    const orderedKeys = [];
    await scanDir(resolvedRoot, resolvedRoot, origin, entriesByKey, orderedKeys);

    const entries = orderedKeys
        .filter((key) => entriesByKey.has(key))
        .map((key) => entriesByKey.get(key));

    entries.sort((left, right) => {
        if (left.relative_path === right.relative_path) {
            return left.uuid.localeCompare(right.uuid);
        }
        return left.relative_path.localeCompare(right.relative_path);
    });

    return entries;
}

async function scanDir(root, dir, origin, entriesByKey, orderedKeys) {
    let children;
    try {
        children = await fs.promises.readdir(dir, { withFileTypes: true });
    } catch {
        return;
    }

    for (const child of children) {
        const childPath = path.join(dir, child.name);
        if (child.isDirectory()) {
            if (shouldSkipDir(root, childPath, child.name)) {
                continue;
            }
            await scanDir(root, childPath, origin, entriesByKey, orderedKeys);
            continue;
        }
        if (!child.isFile() || child.name !== PROTO_MANIFEST_FILE_NAME) {
            continue;
        }

        try {
            const resolved = resolveProtoFile(childPath);
            const absDir = path.resolve(manifestRoot(childPath));
            const entry = {
                slug: slugForIdentity(resolved.identity),
                uuid: resolved.identity.uuid || '',
                dir: absDir,
                relative_path: relativePath(root, absDir),
                origin,
                identity: resolved.identity,
                manifest: {
                    kind: resolved.kind,
                    build: {
                        runner: resolved.build_runner,
                        main: resolved.build_main,
                    },
                    artifacts: {
                        binary: resolved.artifact_binary,
                        primary: resolved.artifact_primary,
                    },
                },
            };

            const key = entry.uuid.trim() || entry.dir;
            if (entriesByKey.has(key)) {
                const existing = entriesByKey.get(key);
                if (pathDepth(entry.relative_path) < pathDepth(existing.relative_path)) {
                    entriesByKey.set(key, entry);
                }
                continue;
            }

            entriesByKey.set(key, entry);
            orderedKeys.push(key);
        } catch {
            // Skip invalid holon manifests to match op discover behavior.
        }
    }
}

function shouldSkipDir(root, dirPath, name) {
    if (path.resolve(dirPath) === path.resolve(root)) {
        return false;
    }
    return name === '.git'
        || name === '.op'
        || name === 'node_modules'
        || name === 'vendor'
        || name === 'build'
        || name.startsWith('.');
}

function relativePath(root, dirPath) {
    const rel = path.relative(root, dirPath);
    return (rel || '.').split(path.sep).join('/');
}

function manifestRoot(filePath) {
    const manifestDir = path.dirname(filePath);
    const versionDir = path.basename(manifestDir);
    const apiDir = path.basename(path.dirname(manifestDir));
    if (/^v[0-9]+(?:[A-Za-z0-9._-]*)?$/.test(versionDir) && apiDir === 'api') {
        return path.dirname(path.dirname(manifestDir));
    }
    return manifestDir;
}

function pathDepth(relative) {
    const trimmed = String(relative || '').trim().replace(/^\/+|\/+$/g, '');
    if (!trimmed || trimmed === '.') {
        return 0;
    }
    return trimmed.split('/').length;
}

function opPath() {
    const configured = String(process.env.OPPATH || '').trim();
    if (configured) {
        return path.resolve(configured);
    }
    return path.join(os.homedir(), '.op');
}

function opbin() {
    const configured = String(process.env.OPBIN || '').trim();
    if (configured) {
        return path.resolve(configured);
    }
    return path.join(opPath(), 'bin');
}

function cacheDir() {
    return path.join(opPath(), 'cache');
}

module.exports = {
    discover,
    discoverLocal,
    discoverAll,
    findBySlug,
    findByUUID,
};
