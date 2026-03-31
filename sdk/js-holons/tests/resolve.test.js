'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const discoverModule = require('../src/discover');
const {
    LOCAL,
    CWD,
    ALL,
    NO_TIMEOUT,
} = require('../src/discovery_types');
const {
    useRuntimeFixture,
    writePackageHolon,
} = require('./helpers/discovery_fixtures');

describe('resolve', () => {
    it('resolve_known_slug', async (t) => {
        const { root } = useRuntimeFixture(t);

        writePackageHolon(path.join(root, 'alpha.holon'), {
            slug: 'alpha',
            uuid: 'uuid-alpha',
            givenName: 'Alpha',
            familyName: 'One',
        });

        const result = await discoverModule.resolve(LOCAL, 'alpha', root, CWD, NO_TIMEOUT);
        assert.equal(result.error, null);
        assert.equal(result.ref?.info?.slug, 'alpha');
    });

    it('resolve_missing', async (t) => {
        const { root } = useRuntimeFixture(t);

        const result = await discoverModule.resolve(LOCAL, 'missing', root, ALL, NO_TIMEOUT);
        assert.match(result.error || '', /not found/i);
        assert.equal(result.ref, null);
    });

    it('resolve_invalid_specifiers', async (t) => {
        const { root } = useRuntimeFixture(t);

        const result = await discoverModule.resolve(LOCAL, 'alpha', root, 0xFF, NO_TIMEOUT);
        assert.match(result.error || '', /invalid specifiers/i);
        assert.equal(result.ref, null);
    });
});
