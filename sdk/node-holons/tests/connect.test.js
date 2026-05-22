'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const connectModule = require('../src/connect');
const {
    LOCAL,
    INSTALLED,
} = require('../src/discovery_types');
const { useStaticDescribeEnv } = require('./helpers/static_describe');
const {
    invokePing,
    useRuntimeFixture,
    waitForPidExit,
    waitForPidFile,
    writeProbeablePackage,
} = require('./helpers/discovery_fixtures');

describe('connect', { concurrency: 1 }, () => {
    it('connect_unresolvable', async (t) => {
        const { root } = useRuntimeFixture(t);

        const result = await connectModule.connect(LOCAL, 'missing', root, INSTALLED, 1000);
        assert.equal(result.channel, null);
        assert.equal(result.origin, null);
        assert.match(result.error || '', /not found/i);
    });

    it('connect_returns_connect_result', async (t) => {
        const fixture = createInstalledFixture(t, 'known-slug');

        const result = await connectModule.connect(LOCAL, fixture.slug, fixture.root, INSTALLED, 5000);
        assert.equal(result.error, null);
        assert.ok(result.channel);
        assert.equal(typeof result.uid, 'string');
        assert.ok(result.origin);

        const out = await invokePing(result.channel, 'connect-result');
        assert.equal(out.message, 'connect-result');
        assert.equal(out.sdk, 'js-holons');

        connectModule.disconnect(result);
        const pid = await waitForPidFile(fixture.pidFile);
        await waitForPidExit(pid);
    });

    it('connect_returns_origin', async (t) => {
        const fixture = createInstalledFixture(t, 'origin-slug');

        const result = await connectModule.connect(LOCAL, fixture.slug, fixture.root, INSTALLED, 5000);
        assert.equal(result.error, null);
        assert.equal(result.origin?.info?.slug, fixture.slug);
        assert.equal(result.origin?.url, pathToFileURL(fixture.packageDir).toString());

        connectModule.disconnect(result);
        const pid = await waitForPidFile(fixture.pidFile);
        await waitForPidExit(pid);
    });

    it('disconnect_accepts_connect_result', async (t) => {
        const fixture = createInstalledFixture(t, 'disconnect-slug');

        const result = await connectModule.connect(LOCAL, fixture.slug, fixture.root, INSTALLED, 5000);
        assert.equal(result.error, null);

        const pid = await waitForPidFile(fixture.pidFile);
        connectModule.disconnect(result);
        await waitForPidExit(pid);
    });
});

function createInstalledFixture(t, slug) {
    const { root, opBin } = useRuntimeFixture(t);
    const packageDir = path.join(opBin, `${slug}.holon`);
    const pidFile = path.join(root, `${slug}.pid`);

    useStaticDescribeEnv(t, {
        manifest: {
            identity: {
                uuid: `${slug}-uuid`,
                given_name: slug.split('-')[0] || 'Connect',
                family_name: 'Fixture',
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
        services: [{
            name: 'echo.v1.Echo',
            methods: [{
                name: 'Ping',
                input_type: 'echo.v1.PingRequest',
                output_type: 'echo.v1.PingResponse',
            }],
        }],
    });

    writeProbeablePackage(packageDir, {
        slug,
        givenName: slug.split('-')[0] || 'Connect',
        familyName: 'Fixture',
        pidFile,
    });

    return {
        root,
        slug,
        packageDir,
        pidFile,
    };
}
