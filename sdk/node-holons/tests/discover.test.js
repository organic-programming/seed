'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const discoverModule = require('../src/discover');
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
} = require('../src/discovery_types');
const { useStaticDescribeEnv } = require('./helpers/static_describe');
const {
    useCwd,
    useRuntimeFixture,
    writePackageHolon,
    writeProbeablePackage,
} = require('./helpers/discovery_fixtures');

describe('Discover', () => {
    it('discover_all_layers', async (t) => {
        const { root, opHome, opBin } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'cwd-alpha.holon'), { slug: 'cwd-alpha', uuid: 'uuid-cwd-alpha', givenName: 'Cwd', familyName: 'Alpha' });
        writePackageHolon(path.join(root, '.op', 'build', 'built-beta.holon'), { slug: 'built-beta', uuid: 'uuid-built-beta', givenName: 'Built', familyName: 'Beta' });
        writePackageHolon(path.join(opBin, 'installed-gamma.holon'), { slug: 'installed-gamma', uuid: 'uuid-installed-gamma', givenName: 'Installed', familyName: 'Gamma' });
        writePackageHolon(path.join(opHome, 'cache', 'deps', 'cached-delta.holon'), { slug: 'cached-delta', uuid: 'uuid-cached-delta', givenName: 'Cached', familyName: 'Delta' });

        const result = await discoverModule.Discover(LOCAL, null, root, ALL, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['built-beta', 'cached-delta', 'cwd-alpha', 'installed-gamma']);
    });

    it('discover_filter_by_specifiers', async (t) => {
        const { root, opBin } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'cwd-alpha.holon'), { slug: 'cwd-alpha', uuid: 'uuid-cwd-alpha', givenName: 'Cwd', familyName: 'Alpha' });
        writePackageHolon(path.join(root, '.op', 'build', 'built-beta.holon'), { slug: 'built-beta', uuid: 'uuid-built-beta', givenName: 'Built', familyName: 'Beta' });
        writePackageHolon(path.join(opBin, 'installed-gamma.holon'), { slug: 'installed-gamma', uuid: 'uuid-installed-gamma', givenName: 'Installed', familyName: 'Gamma' });

        const result = await discoverModule.Discover(LOCAL, null, root, BUILT | INSTALLED, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['built-beta', 'installed-gamma']);
    });

    it('discover_match_by_slug', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });
        writePackageHolon(path.join(root, 'beta.holon'), { slug: 'beta', uuid: 'uuid-beta', givenName: 'Beta', familyName: 'Two' });

        const result = await discoverModule.Discover(LOCAL, 'beta', root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['beta']);
    });

    it('discover_match_by_alias', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), {
            slug: 'alpha',
            uuid: 'uuid-alpha',
            givenName: 'Alpha',
            familyName: 'One',
            aliases: ['first'],
        });

        const result = await discoverModule.Discover(LOCAL, 'first', root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['alpha']);
    });

    it('discover_match_by_uuid_prefix', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), {
            slug: 'alpha',
            uuid: '12345678-aaaa',
            givenName: 'Alpha',
            familyName: 'One',
        });

        const result = await discoverModule.Discover(LOCAL, '12345678', root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['alpha']);
    });

    it('discover_match_by_path', async (t) => {
        const { root } = useRuntimeFixture(t);

        const packageDir = path.join(root, 'alpha.holon');
        writePackageHolon(packageDir, { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });

        const result = await discoverModule.Discover(LOCAL, packageDir, root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(result.found.length, 1);
        assert.equal(result.found[0].info?.slug, 'alpha');
    });

    it('discover_limit_one', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });
        writePackageHolon(path.join(root, 'beta.holon'), { slug: 'beta', uuid: 'uuid-beta', givenName: 'Beta', familyName: 'Two' });

        const result = await discoverModule.Discover(LOCAL, null, root, CWD, 1, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(result.found.length, 1);
    });

    it('discover_limit_zero_means_unlimited', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });
        writePackageHolon(path.join(root, 'beta.holon'), { slug: 'beta', uuid: 'uuid-beta', givenName: 'Beta', familyName: 'Two' });

        const result = await discoverModule.Discover(LOCAL, null, root, CWD, 0, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(result.found.length, 2);
    });

    it('discover_negative_limit_returns_empty', async (t) => {
        const { root } = useRuntimeFixture(t);

        const result = await discoverModule.Discover(LOCAL, null, root, CWD, -1, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(result.found, []);
    });

    it('discover_invalid_specifiers', async (t) => {
        const { root } = useRuntimeFixture(t);

        const result = await discoverModule.Discover(LOCAL, null, root, 0xFF, NO_LIMIT, NO_TIMEOUT);
        assert.match(result.error || '', /invalid specifiers/i);
        assert.deepEqual(result.found, []);
    });

    it('discover_specifiers_zero_treated_as_all', async (t) => {
        const { root, opHome, opBin } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'cwd-alpha.holon'), { slug: 'cwd-alpha', uuid: 'uuid-cwd-alpha', givenName: 'Cwd', familyName: 'Alpha' });
        writePackageHolon(path.join(root, '.op', 'build', 'built-beta.holon'), { slug: 'built-beta', uuid: 'uuid-built-beta', givenName: 'Built', familyName: 'Beta' });
        writePackageHolon(path.join(opBin, 'installed-gamma.holon'), { slug: 'installed-gamma', uuid: 'uuid-installed-gamma', givenName: 'Installed', familyName: 'Gamma' });
        writePackageHolon(path.join(opHome, 'cache', 'deps', 'cached-delta.holon'), { slug: 'cached-delta', uuid: 'uuid-cached-delta', givenName: 'Cached', familyName: 'Delta' });

        const allResult = await discoverModule.Discover(LOCAL, null, root, ALL, NO_LIMIT, NO_TIMEOUT);
        const zeroResult = await discoverModule.Discover(LOCAL, null, root, 0, NO_LIMIT, NO_TIMEOUT);

        assert.equal(allResult.error, null);
        assert.equal(zeroResult.error, null);
        assert.deepEqual(sortedSlugs(zeroResult), sortedSlugs(allResult));
    });

    it('discover_null_expression_returns_all', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });
        writePackageHolon(path.join(root, 'beta.holon'), { slug: 'beta', uuid: 'uuid-beta', givenName: 'Beta', familyName: 'Two' });

        const result = await discoverModule.Discover(LOCAL, null, root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(result.found.length, 2);
    });

    it('discover_missing_expression_returns_empty', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });

        const result = await discoverModule.Discover(LOCAL, 'missing', root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(result.found.length, 0);
    });

    it('discover_skips_excluded_dirs', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'kept.holon'), { slug: 'kept', uuid: 'uuid-kept', givenName: 'Kept', familyName: 'Holon' });
        for (const skipped of ['.git', '.op', 'node_modules', 'vendor', 'build', 'testdata', '.cache']) {
            writePackageHolon(path.join(root, skipped, 'hidden.holon'), {
                slug: `${skipped.replace(/\W+/g, '-')}-hidden`,
                uuid: `${skipped}-uuid`,
                givenName: 'Ignored',
                familyName: 'Holon',
            });
        }

        const result = await discoverModule.Discover(LOCAL, null, root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['kept']);
    });

    it('discover_deduplicates_by_uuid', async (t) => {
        const { root } = useRuntimeFixture(t);

        const cwdPath = path.join(root, 'alpha.holon');
        const builtPath = path.join(root, '.op', 'build', 'alpha-built.holon');
        writePackageHolon(cwdPath, { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });
        writePackageHolon(builtPath, { slug: 'alpha-built', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });

        const result = await discoverModule.Discover(LOCAL, null, root, ALL, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(result.found.length, 1);
        assert.equal(result.found[0].url, pathToFileURL(cwdPath).toString());
    });

    it('discover_holon_json_fast_path', async (t) => {
        const { root } = useRuntimeFixture(t);

        let probeCalls = 0;
        discoverModule._internal.setPackageProber(async () => {
            probeCalls += 1;
            throw new Error('should not probe');
        });
        t.after(() => discoverModule._internal.setPackageProber(null));

        writePackageHolon(path.join(root, 'alpha.holon'), { slug: 'alpha', uuid: 'uuid-alpha', givenName: 'Alpha', familyName: 'One' });

        const result = await discoverModule.Discover(LOCAL, null, root, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(probeCalls, 0);
    });

    it('discover_describe_fallback_when_holon_json_missing', async (t) => {
        const { root } = useRuntimeFixture(t);
        useStaticDescribeEnv(t, {
            manifest: {
                identity: {
                    uuid: 'probe-uuid',
                    given_name: 'Probe',
                    family_name: 'Holon',
                    status: 'draft',
                },
                lang: 'js',
                kind: 'service',
                build: {
                    runner: 'node',
                },
                artifacts: {
                    binary: 'echo-wrapper',
                },
            },
            services: [],
        });

        writeProbeablePackage(path.join(root, 'probe.holon'), {
            givenName: 'Probe',
            familyName: 'Holon',
            includeJSON: false,
        });

        const result = await discoverModule.Discover(LOCAL, null, root, CWD, NO_LIMIT, 5000);
        assert.equal(result.error, null);
        assert.equal(result.found.length, 1);
        assert.equal(result.found[0].info?.slug, 'probe-holon');
        assert.equal(result.found[0].info?.uuid, 'probe-uuid');
    });

    it('discover_siblings_layer', async (t) => {
        const { root } = useRuntimeFixture(t);

        const appExecutable = path.join(root, 'TestApp.app', 'Contents', 'MacOS', 'TestApp');
        const bundleRoot = path.join(root, 'TestApp.app', 'Contents', 'Resources', 'Holons');

        fs.mkdirSync(path.dirname(appExecutable), { recursive: true });
        fs.writeFileSync(appExecutable, '#!/bin/sh\n', { mode: 0o755 });
        writePackageHolon(path.join(bundleRoot, 'bundle.holon'), {
            slug: 'bundle',
            uuid: 'uuid-bundle',
            givenName: 'Bundle',
            familyName: 'Holon',
        });

        discoverModule._internal.setExecutablePathResolver(() => appExecutable);
        t.after(() => discoverModule._internal.setExecutablePathResolver(null));

        const result = await discoverModule.Discover(LOCAL, null, root, SIBLINGS, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['bundle']);
    });

    it('discover_source_layer_offloads_to_local_op', async (t) => {
        const { root } = useRuntimeFixture(t);
        let captured = null;

        discoverModule._internal.setSourceDiscoverer(async (request) => {
            captured = request;
            return {
                found: [{
                    url: pathToFileURL(path.join(root, 'source-holon')).toString(),
                    info: sampleInfo('source-holon', 'source-uuid', 'Source', 'Holon', { has_source: true }),
                    error: null,
                }],
                error: null,
            };
        });
        t.after(() => discoverModule._internal.setSourceDiscoverer(null));

        const result = await discoverModule.Discover(LOCAL, null, root, SOURCE, NO_LIMIT, 1234);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['source-holon']);
        assert.equal(captured?.scope, LOCAL);
        assert.equal(captured?.root, root);
        assert.equal(captured?.specifiers, SOURCE);
        assert.equal(captured?.timeout, 1234);
    });

    it('discover_built_layer', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, '.op', 'build', 'built.holon'), {
            slug: 'built',
            uuid: 'uuid-built',
            givenName: 'Built',
            familyName: 'Holon',
        });

        const result = await discoverModule.Discover(LOCAL, null, root, BUILT, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['built']);
    });

    it('discover_installed_layer', async (t) => {
        const { root, opBin } = useRuntimeFixture(t);

        writePackageHolon(path.join(opBin, 'installed.holon'), {
            slug: 'installed',
            uuid: 'uuid-installed',
            givenName: 'Installed',
            familyName: 'Holon',
        });

        const result = await discoverModule.Discover(LOCAL, null, root, INSTALLED, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['installed']);
    });

    it('discover_cached_layer', async (t) => {
        const { root, opHome } = useRuntimeFixture(t);

        writePackageHolon(path.join(opHome, 'cache', 'deep', 'cached.holon'), {
            slug: 'cached',
            uuid: 'uuid-cached',
            givenName: 'Cached',
            familyName: 'Holon',
        });

        const result = await discoverModule.Discover(LOCAL, null, root, CACHED, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['cached']);
    });

    it('discover_nil_root_defaults_to_cwd', async (t) => {
        const { root } = useRuntimeFixture(t);
        useCwd(t, root);

        writePackageHolon(path.join(root, 'alpha.holon'), {
            slug: 'alpha',
            uuid: 'uuid-alpha',
            givenName: 'Alpha',
            familyName: 'One',
        });

        const result = await discoverModule.Discover(LOCAL, null, null, CWD, NO_LIMIT, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.deepEqual(sortedSlugs(result), ['alpha']);
    });

    it('discover_empty_root_returns_error', async () => {
        const result = await discoverModule.Discover(LOCAL, null, '', ALL, NO_LIMIT, NO_TIMEOUT);
        assert.match(result.error || '', /root cannot be empty/i);
    });

    it('discover_unsupported_scope_returns_error', async (t) => {
        const { root } = useRuntimeFixture(t);

        const proxy = await discoverModule.Discover(PROXY, null, root, ALL, NO_LIMIT, NO_TIMEOUT);
        const delegated = await discoverModule.Discover(DELEGATED, null, root, ALL, NO_LIMIT, NO_TIMEOUT);

        assert.match(proxy.error || '', /scope 1 not supported/i);
        assert.match(delegated.error || '', /scope 2 not supported/i);
    });
});

function sortedSlugs(result) {
    return result.found
        .map((ref) => ref.info?.slug)
        .filter(Boolean)
        .sort();
}

function sampleInfo(slug, uuid, givenName, familyName, extra = {}) {
    return {
        slug,
        uuid,
        identity: {
            given_name: givenName,
            family_name: familyName,
        },
        lang: 'js',
        runner: '',
        status: 'draft',
        kind: '',
        transport: '',
        entrypoint: '',
        architectures: [],
        has_dist: false,
        has_source: false,
        ...extra,
    };
}
