'use strict';

const fs = require('node:fs');
const path = require('node:path');
const protobuf = require('protobufjs');

const PROTO_ROOT = findExistingRoot('holons/v1/manifest.proto', [
    process.env.HOLONS_PROTO_ROOT,
    ...ancestorCandidates(process.cwd(), ['.op', 'protos']),
    ...ancestorCandidates(__dirname, ['.op', 'protos']),
    ...ancestorCandidates(__dirname, ['holons', 'grace-op', '_protos']),
    ...ancestorCandidates(__dirname, ['holons', 'grace-op', '.op', 'protos']),
    ...ancestorCandidates(__dirname, ['_protos']),
]);

const GOOGLE_PROTO_ROOT = findExistingRoot('google/protobuf/descriptor.proto', [
    process.env.HOLONS_PROTOBUF_INCLUDE,
    ...ancestorCandidates(__dirname, ['node_modules', 'protobufjs']),
    '/opt/homebrew/include',
    '/usr/local/include',
    '/usr/include',
]);

if (!PROTO_ROOT) {
    throw new Error('unable to locate holons system protos (expected holons/v1/manifest.proto)');
}

const PROTO_FILES = [
    path.join(PROTO_ROOT, 'holons/v1/manifest.proto'),
    path.join(PROTO_ROOT, 'holons/v1/session.proto'),
    path.join(PROTO_ROOT, 'holons/v1/describe.proto'),
    path.join(PROTO_ROOT, 'holons/v1/instance.proto'),
    path.join(PROTO_ROOT, 'holons/v1/observability.proto'),
    path.join(PROTO_ROOT, 'holons/v1/coax.proto'),
];

const root = new protobuf.Root();
const defaultResolvePath = protobuf.Root.prototype.resolvePath;

root.resolvePath = function resolvePath(origin, target) {
    const candidates = [];
    if (origin) {
        candidates.push(path.resolve(path.dirname(origin), target));
    }
    candidates.push(path.resolve(PROTO_ROOT, target));
    if (GOOGLE_PROTO_ROOT) {
        candidates.push(path.resolve(GOOGLE_PROTO_ROOT, target));
    }

    for (const candidate of candidates) {
        if (fs.existsSync(candidate)) {
            return candidate;
        }
    }
    return defaultResolvePath.call(this, origin, target);
};

root.loadSync(PROTO_FILES, { keepCase: true });
root.resolveAll();

function ancestorCandidates(start, suffixParts) {
    const candidates = [];
    const seen = new Set();
    let current = path.resolve(String(start || '.'));

    for (;;) {
        const candidate = path.join(current, ...suffixParts);
        if (!seen.has(candidate)) {
            seen.add(candidate);
            candidates.push(candidate);
        }

        const parent = path.dirname(current);
        if (parent === current) {
            break;
        }
        current = parent;
    }

    return candidates;
}

function findExistingRoot(relativeFile, candidates) {
    const seen = new Set();

    for (const candidate of candidates) {
        const trimmed = String(candidate || '').trim();
        if (!trimmed) {
            continue;
        }

        const resolved = path.resolve(trimmed);
        if (seen.has(resolved)) {
            continue;
        }
        seen.add(resolved);

        if (fs.existsSync(path.join(resolved, relativeFile))) {
            return resolved;
        }
    }

    return '';
}

module.exports = { root };
