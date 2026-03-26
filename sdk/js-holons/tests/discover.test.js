'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const discoverModule = require('../src/discover');

function writeHolon(root, relativeDir, seed) {
    const dir = path.join(root, relativeDir);
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, 'holon.proto'), [
        'syntax = "proto3";',
        '',
        'package test.v1;',
        '',
        'option (holons.v1.manifest) = {',
        '  identity: {',
        `    uuid: "${seed.uuid}"`,
        `    given_name: "${seed.givenName}"`,
        `    family_name: "${seed.familyName}"`,
        '    motto: "Test"',
        '    composer: "test"',
        '    clade: "deterministic/pure"',
        '    status: "draft"',
        '    born: "2026-03-07"',
        '  }',
        '  lineage: {',
        '    generated_by: "test"',
        '  }',
        '  kind: "native"',
        '  build: {',
        '    runner: "go-module"',
        '  }',
        '  artifacts: {',
        `    binary: "${seed.binary}"`,
        '  }',
        '};',
        '',
    ].join('\n'));
}

describe('discover', () => {
    it('recurses skips and dedups by uuid', async () => {
        const root = fs.mkdtempSync(path.join(os.tmpdir(), 'holons-js-discover-'));
        try {
            writeHolon(root, 'holons/alpha', {
                uuid: 'uuid-alpha',
                givenName: 'Alpha',
                familyName: 'Go',
                binary: 'alpha-go',
            });
            writeHolon(root, 'nested/beta', {
                uuid: 'uuid-beta',
                givenName: 'Beta',
                familyName: 'Rust',
                binary: 'beta-rust',
            });
            writeHolon(root, 'nested/dup/alpha', {
                uuid: 'uuid-alpha',
                givenName: 'Alpha',
                familyName: 'Go',
                binary: 'alpha-go',
            });

            for (const skipped of ['.git/hidden', '.op/hidden', 'node_modules/hidden', 'vendor/hidden', 'build/hidden', '.cache/hidden']) {
                writeHolon(root, skipped, {
                    uuid: `ignored-${path.basename(skipped)}`,
                    givenName: 'Ignored',
                    familyName: 'Holon',
                    binary: 'ignored-holon',
                });
            }

            const entries = await discoverModule.discover(root);
            assert.equal(entries.length, 2);

            const alpha = entries.find((entry) => entry.uuid === 'uuid-alpha');
            assert.equal(alpha.slug, 'alpha-go');
            assert.equal(alpha.relative_path, 'holons/alpha');
            assert.equal(alpha.manifest.build.runner, 'go-module');

            const beta = entries.find((entry) => entry.uuid === 'uuid-beta');
            assert.equal(beta.relative_path, 'nested/beta');

            const previousCwd = process.cwd();
            process.chdir(root);
            try {
                const match = await discoverModule.findBySlug('alpha-go');
                assert.equal(match?.uuid, 'uuid-alpha');
                assert.equal(
                    fs.realpathSync(match?.dir),
                    fs.realpathSync(path.join(root, 'holons', 'alpha')),
                );
            } finally {
                process.chdir(previousCwd);
            }
        } finally {
            fs.rmSync(root, { recursive: true, force: true });
        }
    });
});
