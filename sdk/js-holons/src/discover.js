'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const { fileURLToPath, pathToFileURL } = require('node:url');

const grpc = require('@grpc/grpc-js');

const grpcclient = require('./grpcclient');
const { slugForIdentity } = require('./identity');
const describeWire = require('./gen/holons/v1/describe');
const {
    LOCAL,
    PROXY,
    DELEGATED,
    SIBLINGS,
    CWD,
    SOURCE,
    BUILT,
    INSTALLED,
    CACHED,
    ALL,
    NO_LIMIT,
    NO_TIMEOUT,
} = require('./discovery_types');

const execFileAsync = promisify(execFile);
const HolonMetaClient = grpc.makeGenericClientConstructor(
    describeWire.HOLON_META_SERVICE_DEF,
    'HolonMeta',
    {},
);

/**
 * @typedef {import('./discovery_types').IdentityInfo} IdentityInfo
 * @typedef {import('./discovery_types').HolonInfo} HolonInfo
 * @typedef {import('./discovery_types').HolonRef} HolonRef
 * @typedef {import('./discovery_types').DiscoverResult} DiscoverResult
 * @typedef {import('./discovery_types').ResolveResult} ResolveResult
 */

let sourceDiscoverer = defaultSourceDiscoverer;
let packageProber = nativePackageProber;
let executablePathResolver = () => process.execPath;

async function Discover(scope, expression, root, specifiers, limit, timeout) {
    const normalizedSpecifiers = normalizeSpecifiers(specifiers);
    const normalizedLimit = normalizeInt(limit, NO_LIMIT);
    const normalizedTimeout = normalizeInt(timeout, NO_TIMEOUT);

    if (scope !== LOCAL) {
        return emptyDiscoverResult(`scope ${scope} not supported`);
    }
    if (normalizedSpecifiers.invalid) {
        return emptyDiscoverResult(normalizedSpecifiers.error);
    }
    if (normalizedLimit < 0) {
        return { found: [], error: null };
    }

    const normalizedExpression = normalizeExpression(expression);
    if (hasUnsupportedDirectURL(normalizedExpression)) {
        return emptyDiscoverResult('direct URL expressions are not supported');
    }

    let searchRoot;
    try {
        searchRoot = await resolveDiscoverRoot(root);
    } catch (err) {
        return emptyDiscoverResult(messageOf(err));
    }

    try {
        const pathCandidate = pathExpressionCandidate(normalizedExpression, searchRoot);
        if (pathCandidate) {
            const direct = await discoverPackageRefAtPath(pathCandidate, normalizedTimeout);
            if (direct.handled) {
                return {
                    found: applyLimit(direct.ref ? [direct.ref] : [], normalizedLimit),
                    error: direct.error,
                };
            }
        }

        const refs = await discoverRefs(searchRoot, normalizedSpecifiers.value, normalizedExpression, normalizedTimeout);
        const found = [];
        for (const ref of refs) {
            if (!matchesExpression(ref, normalizedExpression, searchRoot)) {
                continue;
            }
            found.push(cloneRef(ref));
            if (normalizedLimit > 0 && found.length >= normalizedLimit) {
                break;
            }
        }

        return { found, error: null };
    } catch (err) {
        return emptyDiscoverResult(messageOf(err));
    }
}

async function resolve(scope, expression, root, specifiers, timeout) {
    const result = await Discover(scope, expression, root, specifiers, 1, timeout);
    if (result.error) {
        return { ref: null, error: result.error };
    }
    if (result.found.length === 0) {
        return {
            ref: null,
            error: normalizeExpressionLabel(expression),
        };
    }

    const ref = cloneRef(result.found[0]);
    if (ref.error) {
        return { ref, error: ref.error };
    }
    return { ref, error: null };
}

async function discoverRefs(root, specifiers, expression, timeout) {
    const layers = [
        {
            flag: SIBLINGS,
            name: 'siblings',
            scan: async () => {
                const bundleRoot = await bundleHolonsRoot();
                if (!bundleRoot) {
                    return [];
                }
                return discoverPackagesDirect(bundleRoot, timeout);
            },
        },
        {
            flag: CWD,
            name: 'cwd',
            scan: () => discoverPackagesRecursive(root, timeout),
        },
        {
            flag: SOURCE,
            name: 'source',
            scan: () => discoverSourceRefs(root, expression, timeout),
        },
        {
            flag: BUILT,
            name: 'built',
            scan: () => discoverPackagesDirect(path.join(root, '.op', 'build'), timeout),
        },
        {
            flag: INSTALLED,
            name: 'installed',
            scan: () => discoverPackagesDirect(opbin(), timeout),
        },
        {
            flag: CACHED,
            name: 'cached',
            scan: () => discoverPackagesRecursive(cacheDir(), timeout),
        },
    ];

    const seen = new Set();
    const found = [];

    for (const layer of layers) {
        if ((specifiers & layer.flag) === 0) {
            continue;
        }

        const refs = await layer.scan();
        for (const ref of refs) {
            const key = refKey(ref);
            if (seen.has(key)) {
                continue;
            }
            seen.add(key);
            found.push(ref);
        }
    }

    return found;
}

async function discoverSourceRefs(root, expression, timeout) {
    const result = await sourceDiscoverer({
        scope: LOCAL,
        expression,
        root,
        specifiers: SOURCE,
        limit: NO_LIMIT,
        timeout,
    });

    const normalized = normalizeDiscoverResult(result);
    if (normalized.error) {
        throw new Error(normalized.error);
    }

    return normalized.found;
}

async function discoverPackagesDirect(root, timeout) {
    const dirs = await packageDirsDirect(root);
    return discoverPackagesFromDirs(root, dirs, timeout);
}

async function discoverPackagesRecursive(root, timeout) {
    const dirs = await packageDirsRecursive(root);
    return discoverPackagesFromDirs(root, dirs, timeout);
}

async function discoverPackagesFromDirs(root, dirs, timeout) {
    const absRoot = path.resolve(String(root || currentRoot()));
    const entriesByKey = new Map();
    const keys = [];

    for (const dir of dirs) {
        const ref = await loadOrProbePackageRef(dir, timeout);
        const record = {
            ref,
            relativePath: relativePath(absRoot, dir),
        };
        const key = refKey(ref);

        if (entriesByKey.has(key)) {
            const current = entriesByKey.get(key);
            if (shouldReplaceRecord(current, record)) {
                entriesByKey.set(key, record);
            }
            continue;
        }

        entriesByKey.set(key, record);
        keys.push(key);
    }

    const refs = keys
        .filter((key) => entriesByKey.has(key))
        .map((key) => entriesByKey.get(key))
        .sort(comparePackageRecords)
        .map((record) => record.ref);

    return refs;
}

async function loadOrProbePackageRef(dir, timeout) {
    try {
        return await loadPackageRef(dir);
    } catch (loadErr) {
        try {
            const info = await packageProber(dir, timeout);
            return buildRefForPath(dir, info, null);
        } catch (probeErr) {
            const message = `${messageOf(loadErr)}; ${messageOf(probeErr)}`;
            return buildRefForPath(dir, null, message);
        }
    }
}

async function loadPackageRef(dir) {
    const manifestPath = path.join(dir, '.holon.json');
    const data = await fs.promises.readFile(manifestPath, 'utf8');
    const payload = JSON.parse(data);
    if (payload && payload.schema && payload.schema !== 'holon-package/v1') {
        throw new Error(`${manifestPath}: unsupported schema ${JSON.stringify(payload.schema)}`);
    }

    /** @type {HolonInfo} */
    const info = normalizeHolonInfo({
        slug: payload?.slug,
        uuid: payload?.uuid,
        identity: {
            given_name: payload?.identity?.given_name,
            family_name: payload?.identity?.family_name,
            motto: payload?.identity?.motto,
            aliases: payload?.identity?.aliases,
        },
        lang: payload?.lang,
        runner: payload?.runner,
        status: payload?.status,
        kind: payload?.kind,
        transport: payload?.transport,
        entrypoint: payload?.entrypoint,
        architectures: payload?.architectures,
        has_dist: payload?.has_dist,
        has_source: payload?.has_source,
    });

    return buildRefForPath(dir, info, null);
}

async function nativePackageProber(dir, timeout) {
    const binaryPath = await packageBinaryPath(dir);
    const info = await describeBinaryTarget(binaryPath, timeout);
    return normalizeHolonInfo({
        ...info,
        entrypoint: info.entrypoint || path.basename(binaryPath),
        has_dist: await dirExists(path.join(dir, 'dist')),
        has_source: await dirExists(path.join(dir, 'git')),
    });
}

async function describeBinaryTarget(binaryPath, timeout) {
    const session = await grpcclient.dialStdio(binaryPath, HolonMetaClient, {
        env: process.env,
    });

    try {
        await waitForReady(session.client, timeout);
        const response = await describeClient(session.client, timeout);
        return holonInfoFromDescribeResponse(response);
    } finally {
        await session.close().catch(() => {});
    }
}

function describeClient(client, timeout) {
    return new Promise((resolve, reject) => {
        const callback = (err, response) => {
            if (err) {
                reject(err);
                return;
            }
            resolve(response);
        };

        if (timeout > 0) {
            client.Describe({}, { deadline: Date.now() + timeout }, callback);
            return;
        }

        client.Describe({}, callback);
    });
}

function holonInfoFromDescribeResponse(response) {
    const manifest = response?.manifest;
    const identity = manifest?.identity;
    if (!manifest) {
        throw new Error('Describe returned no manifest');
    }
    if (!identity) {
        throw new Error('Describe returned no manifest identity');
    }

    return normalizeHolonInfo({
        slug: slugForIdentity({
            given_name: identity.given_name,
            family_name: identity.family_name,
        }),
        uuid: identity.uuid,
        identity: {
            given_name: identity.given_name,
            family_name: identity.family_name,
            motto: identity.motto,
            aliases: identity.aliases,
        },
        lang: manifest.lang,
        runner: manifest.build?.runner,
        status: identity.status,
        kind: manifest.kind,
        transport: manifest.transport,
        entrypoint: manifest.artifacts?.binary,
        architectures: manifest.platforms,
        has_dist: false,
        has_source: false,
    });
}

function normalizeHolonInfo(input) {
    const identityAliases = normalizeStringArray(input?.identity?.aliases);
    const identity = {
        given_name: normalizeString(input?.identity?.given_name),
        family_name: normalizeString(input?.identity?.family_name),
    };
    const motto = normalizeString(input?.identity?.motto);
    if (motto) {
        identity.motto = motto;
    }
    if (identityAliases.length > 0) {
        identity.aliases = identityAliases;
    }

    const slug = normalizeString(input?.slug) || slugForIdentity(identity);

    return {
        slug,
        uuid: normalizeString(input?.uuid),
        identity,
        lang: normalizeString(input?.lang),
        runner: normalizeString(input?.runner),
        status: normalizeString(input?.status),
        kind: normalizeString(input?.kind),
        transport: normalizeString(input?.transport),
        entrypoint: normalizeString(input?.entrypoint),
        architectures: normalizeStringArray(input?.architectures),
        has_dist: Boolean(input?.has_dist),
        has_source: Boolean(input?.has_source),
    };
}

function normalizeDiscoverResult(result) {
    if (!result || typeof result !== 'object') {
        return { found: [], error: null };
    }

    const found = Array.isArray(result.found)
        ? result.found.map((ref) => normalizeRef(ref)).filter(Boolean)
        : [];

    return {
        found,
        error: normalizeNullableString(result.error),
    };
}

function normalizeRef(ref) {
    if (!ref || typeof ref !== 'object') {
        return null;
    }

    const url = normalizeString(ref.url);
    if (!url) {
        return null;
    }

    return {
        url,
        info: ref.info ? normalizeHolonInfo(ref.info) : null,
        error: normalizeNullableString(ref.error),
    };
}

async function discoverPackageRefAtPath(candidatePath, timeout) {
    const absPath = path.resolve(candidatePath);
    let stat;
    try {
        stat = await fs.promises.stat(absPath);
    } catch (err) {
        if (err && err.code === 'ENOENT') {
            return { handled: true, ref: null, error: null };
        }
        throw err;
    }

    if (!stat.isDirectory()) {
        return { handled: false, ref: null, error: null };
    }

    if (!absPath.endsWith('.holon') && !await fileExists(path.join(absPath, '.holon.json'))) {
        return { handled: false, ref: null, error: null };
    }

    const ref = await loadOrProbePackageRef(absPath, timeout);
    return { handled: true, ref, error: null };
}

async function packageDirsDirect(root) {
    const absRoot = path.resolve(String(root || currentRoot()));
    let stat;
    try {
        stat = await fs.promises.stat(absRoot);
    } catch (err) {
        if (err && err.code === 'ENOENT') {
            return [];
        }
        throw err;
    }
    if (!stat.isDirectory()) {
        return [];
    }

    const entries = await fs.promises.readdir(absRoot, { withFileTypes: true });
    const dirs = entries
        .filter((entry) => entry.isDirectory() && entry.name.endsWith('.holon'))
        .map((entry) => path.join(absRoot, entry.name))
        .sort();

    return dirs;
}

async function packageDirsRecursive(root) {
    const absRoot = path.resolve(String(root || currentRoot()));
    let stat;
    try {
        stat = await fs.promises.stat(absRoot);
    } catch (err) {
        if (err && err.code === 'ENOENT') {
            return [];
        }
        throw err;
    }
    if (!stat.isDirectory()) {
        return [];
    }

    const dirs = [];
    await walkPackageDirs(absRoot, absRoot, dirs);
    dirs.sort();
    return dirs;
}

async function walkPackageDirs(root, current, dirs) {
    let children;
    try {
        children = await fs.promises.readdir(current, { withFileTypes: true });
    } catch {
        return;
    }

    for (const child of children) {
        if (!child.isDirectory()) {
            continue;
        }

        const childPath = path.join(current, child.name);
        if (shouldSkipDir(root, childPath, child.name)) {
            continue;
        }
        if (child.name.endsWith('.holon')) {
            dirs.push(childPath);
            continue;
        }

        await walkPackageDirs(root, childPath, dirs);
    }
}

function shouldSkipDir(root, dirPath, name) {
    if (path.resolve(dirPath) === path.resolve(root)) {
        return false;
    }
    if (name.endsWith('.holon')) {
        return false;
    }
    if (name === '.git' || name === '.op' || name === 'node_modules' || name === 'vendor' || name === 'build' || name === 'testdata') {
        return true;
    }
    return name.startsWith('.');
}

function matchesExpression(ref, expression, root) {
    if (expression === null) {
        return true;
    }
    if (expression === '') {
        return false;
    }

    if (ref.info) {
        if (ref.info.slug === expression) {
            return true;
        }
        if (ref.info.uuid && ref.info.uuid.startsWith(expression)) {
            return true;
        }
        const aliases = ref.info.identity?.aliases || [];
        if (aliases.includes(expression)) {
            return true;
        }
    }

    const refPath = pathFromRefURL(ref.url);
    if (!refPath) {
        return false;
    }

    const base = stripHolonSuffix(path.basename(refPath));
    if (base === expression) {
        return true;
    }

    const candidatePath = pathExpressionCandidate(expression, root);
    return Boolean(candidatePath && samePath(candidatePath, refPath));
}

function pathExpressionCandidate(expression, root) {
    if (expression === null || expression === '') {
        return '';
    }

    if (expression.toLowerCase().startsWith('file://')) {
        try {
            return path.resolve(fileURLToPath(expression));
        } catch {
            return '';
        }
    }

    if (path.isAbsolute(expression)) {
        return path.resolve(expression);
    }

    if (
        expression.startsWith('.')
        || expression.includes(path.sep)
        || expression.includes('/')
        || expression.includes('\\')
        || expression.toLowerCase().endsWith('.holon')
    ) {
        return path.resolve(root, expression);
    }

    return '';
}

function hasUnsupportedDirectURL(expression) {
    if (expression === null || expression === '') {
        return false;
    }

    const match = /^([A-Za-z][A-Za-z0-9+.-]*):\/\//.exec(expression);
    if (!match) {
        return false;
    }

    return match[1].toLowerCase() !== 'file';
}

async function resolveDiscoverRoot(root) {
    if (root == null) {
        return currentRoot();
    }

    const trimmed = normalizeString(root);
    if (!trimmed) {
        throw new Error('root cannot be empty');
    }

    const absRoot = path.resolve(trimmed);
    const stat = await fs.promises.stat(absRoot);
    if (!stat.isDirectory()) {
        throw new Error(`root ${JSON.stringify(trimmed)} is not a directory`);
    }
    return absRoot;
}

function currentRoot() {
    return path.resolve(process.cwd());
}

async function bundleHolonsRoot() {
    let executablePath = '';
    try {
        executablePath = await Promise.resolve(executablePathResolver());
    } catch {
        return '';
    }
    if (!executablePath) {
        return '';
    }

    let current = path.dirname(path.resolve(executablePath));
    for (;;) {
        if (current.toLowerCase().endsWith('.app')) {
            const candidate = path.join(current, 'Contents', 'Resources', 'Holons');
            if (await dirExists(candidate)) {
                return candidate;
            }
        }

        const parent = path.dirname(current);
        if (parent === current) {
            break;
        }
        current = parent;
    }

    return '';
}

function oppath() {
    const configured = normalizeString(process.env.OPPATH);
    if (configured) {
        return path.resolve(configured);
    }
    return path.join(os.homedir(), '.op');
}

function opbin() {
    const configured = normalizeString(process.env.OPBIN);
    if (configured) {
        return path.resolve(configured);
    }
    return path.join(oppath(), 'bin');
}

function cacheDir() {
    return path.join(oppath(), 'cache');
}

async function defaultSourceDiscoverer(request) {
    const cwd = normalizeString(request?.root) || currentRoot();
    const timeout = normalizeInt(request?.timeout, NO_TIMEOUT);

    let stdout;
    try {
        ({ stdout } = await execFileAsync('op', ['--format', 'json', 'discover'], {
            cwd,
            env: process.env,
            timeout: timeout > 0 ? timeout : undefined,
            maxBuffer: 8 * 1024 * 1024,
        }));
    } catch (err) {
        if (err && err.code === 'ENOENT') {
            return { found: [], error: null };
        }
        if (err && typeof err.stderr === 'string' && /no such file or directory/i.test(err.stderr)) {
            return { found: [], error: null };
        }
        return { found: [], error: `source discovery via op failed: ${messageOf(err)}` };
    }

    let payload;
    try {
        payload = JSON.parse(stdout);
    } catch (err) {
        return { found: [], error: `source discovery via op returned invalid JSON: ${messageOf(err)}` };
    }

    const expression = normalizeExpression(request?.expression);
    const found = [];
    for (const entry of Array.isArray(payload?.entries) ? payload.entries : []) {
        const origin = normalizeString(entry?.origin);
        if (origin !== 'source') {
            continue;
        }

        const absPath = path.resolve(cwd, normalizeString(entry?.relative_path));
        const ref = buildRefForPath(absPath, {
            slug: entry?.slug,
            uuid: entry?.uuid,
            identity: {
                given_name: entry?.given_name,
                family_name: entry?.family_name,
            },
            lang: entry?.lang,
            status: entry?.status,
            kind: '',
            runner: '',
            transport: '',
            entrypoint: '',
            architectures: [],
            has_dist: false,
            has_source: true,
        }, null);

        if (!matchesExpression(ref, expression, cwd)) {
            continue;
        }
        found.push(ref);
    }

    return { found, error: null };
}

async function packageBinaryPath(dir) {
    const archDir = path.join(dir, 'bin', packageArchDir());
    const entries = await fs.promises.readdir(archDir, { withFileTypes: true });
    const files = entries
        .filter((entry) => entry.isFile())
        .map((entry) => path.join(archDir, entry.name))
        .sort();

    if (files.length === 0) {
        throw new Error(`${archDir}: no runnable package binary found`);
    }

    return files[0];
}

function packageArchDir() {
    const platform = process.platform === 'win32' ? 'windows' : process.platform;
    const arch = ({
        x64: 'amd64',
        ia32: '386',
    })[process.arch] || process.arch;

    return `${platform}_${arch}`;
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

function buildRefForPath(dir, info, error) {
    return {
        url: pathToFileURL(path.resolve(dir)).toString(),
        info: info ? normalizeHolonInfo(info) : null,
        error: normalizeNullableString(error),
    };
}

function refKey(ref) {
    const uuid = normalizeString(ref?.info?.uuid);
    if (uuid) {
        return uuid;
    }
    return normalizeString(ref?.url);
}

function comparePackageRecords(left, right) {
    if (left.relativePath === right.relativePath) {
        return refKey(left.ref).localeCompare(refKey(right.ref));
    }
    return left.relativePath.localeCompare(right.relativePath);
}

function shouldReplaceRecord(current, next) {
    return pathDepth(next.relativePath) < pathDepth(current.relativePath);
}

function relativePath(root, dir) {
    const rel = path.relative(root, dir);
    return (rel || '.').split(path.sep).join('/');
}

function pathDepth(relative) {
    const trimmed = String(relative || '').trim().replace(/^\/+|\/+$/g, '');
    if (!trimmed || trimmed === '.') {
        return 0;
    }
    return trimmed.split('/').length;
}

function stripHolonSuffix(name) {
    return String(name || '').replace(/\.holon$/i, '');
}

function pathFromRefURL(value) {
    const trimmed = normalizeString(value);
    if (!trimmed) {
        return '';
    }
    if (trimmed.toLowerCase().startsWith('file://')) {
        try {
            return path.resolve(fileURLToPath(trimmed));
        } catch {
            return '';
        }
    }
    return '';
}

function samePath(left, right) {
    return path.resolve(left) === path.resolve(right);
}

function normalizeSpecifiers(specifiers) {
    const value = normalizeInt(specifiers, 0);
    if (value < 0 || (value & ~ALL) !== 0) {
        return {
            invalid: true,
            value,
            error: `invalid specifiers 0x${(value >>> 0).toString(16).toUpperCase()}: valid range is 0x00-0x3F`,
        };
    }
    return {
        invalid: false,
        value: value === 0 ? ALL : value,
        error: null,
    };
}

function normalizeExpression(expression) {
    if (expression == null) {
        return null;
    }
    return String(expression).trim();
}

function normalizeExpressionLabel(expression) {
    if (expression == null) {
        return 'holon not found';
    }
    return `holon ${JSON.stringify(String(expression))} not found`;
}

function normalizeNullableString(value) {
    const trimmed = normalizeString(value);
    return trimmed || null;
}

function normalizeString(value) {
    return String(value ?? '').trim();
}

function normalizeStringArray(value) {
    if (!Array.isArray(value)) {
        return [];
    }
    return value
        .map((item) => normalizeString(item))
        .filter(Boolean);
}

function normalizeInt(value, fallback) {
    if (Number.isFinite(Number(value))) {
        return Number(value);
    }
    return fallback;
}

function applyLimit(refs, limit) {
    if (limit <= 0 || refs.length <= limit) {
        return refs;
    }
    return refs.slice(0, limit);
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

function emptyDiscoverResult(error) {
    return { found: [], error: normalizeNullableString(error) };
}

async function fileExists(candidate) {
    try {
        const stat = await fs.promises.stat(candidate);
        return stat.isFile();
    } catch {
        return false;
    }
}

async function dirExists(candidate) {
    try {
        const stat = await fs.promises.stat(candidate);
        return stat.isDirectory();
    } catch {
        return false;
    }
}

function messageOf(err) {
    if (err && typeof err === 'object') {
        if (typeof err.stderr === 'string' && err.stderr.trim()) {
            return err.stderr.trim();
        }
        if (typeof err.message === 'string' && err.message.trim()) {
            return err.message.trim();
        }
    }
    return String(err);
}

module.exports = {
    Discover,
    resolve,
    _internal: {
        packageArchDir,
        setExecutablePathResolver(fn) {
            executablePathResolver = typeof fn === 'function' ? fn : (() => process.execPath);
        },
        setPackageProber(fn) {
            packageProber = typeof fn === 'function' ? fn : nativePackageProber;
        },
        setSourceDiscoverer(fn) {
            sourceDiscoverer = typeof fn === 'function' ? fn : defaultSourceDiscoverer;
        },
    },
};
