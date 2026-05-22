'use strict';

const fs = require('node:fs');
const path = require('node:path');

function member(id) {
    const executable = (process.env.OP_HOLON_EXECUTABLE || '').trim() || process.argv[1] || process.execPath;
    return memberFromExecutable(executable, id);
}

function memberFromExecutable(executable, id) {
    if (!String(id || '').trim()) {
        throw new Error('member id is required');
    }
    const memberDir = path.join(path.dirname(path.resolve(String(executable))), 'holons', String(id));
    const entries = fs.readdirSync(memberDir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
        if (entry.isDirectory()) continue;
        const candidate = path.join(memberDir, entry.name);
        if (entry.name.endsWith('.exe') || isExecutable(candidate)) {
            return candidate;
        }
    }
    throw new Error(`no executable found in ${memberDir}`);
}

function isExecutable(candidate) {
    try {
        fs.accessSync(candidate, fs.constants.X_OK);
        return true;
    } catch (_) {
        return false;
    }
}

module.exports = {
    member,
    memberFromExecutable,
};
