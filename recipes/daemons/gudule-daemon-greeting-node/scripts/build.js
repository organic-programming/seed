#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..');
const source = path.join(root, 'src', 'main.js');
const distDir = path.join(root, 'dist');
const output = path.join(distDir, 'gudule-daemon-greeting-node.js');

fs.mkdirSync(distDir, { recursive: true });
fs.copyFileSync(source, output);
fs.chmodSync(output, 0o755);
