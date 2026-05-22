'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const composite = require('../src/composite');

describe('composite member resolution', () => {
    it('resolves an executable relative to the launcher', () => {
        const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'js-composite-'));
        try {
            const launcher = path.join(dir, 'bin', 'darwin_arm64', 'parent');
            const memberDir = path.join(path.dirname(launcher), 'holons', 'node-node');
            const member = path.join(memberDir, 'observability-cascade-node-node');
            fs.mkdirSync(memberDir, { recursive: true });
            fs.writeFileSync(launcher, '#!/bin/sh\n');
            fs.writeFileSync(member, '#!/bin/sh\n');
            fs.chmodSync(launcher, 0o755);
            fs.chmodSync(member, 0o755);

            assert.equal(composite.memberFromExecutable(launcher, 'node-node'), member);
        } finally {
            fs.rmSync(dir, { recursive: true, force: true });
        }
    });

    it('errors when no executable exists', () => {
        const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'js-composite-'));
        try {
            const launcher = path.join(dir, 'bin', 'darwin_arm64', 'parent');
            fs.mkdirSync(path.join(path.dirname(launcher), 'holons', 'node-node'), { recursive: true });
            fs.writeFileSync(launcher, '#!/bin/sh\n');

            assert.throws(() => composite.memberFromExecutable(launcher, 'node-node'), /no executable found/);
        } finally {
            fs.rmSync(dir, { recursive: true, force: true });
        }
    });
});
