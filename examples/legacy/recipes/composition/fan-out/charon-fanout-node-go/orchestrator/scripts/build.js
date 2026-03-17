#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const src = path.join(root, "src", "main.js");
const dist = path.join(root, "dist");
const out = path.join(dist, "charon-fanout-node-go-orchestrator");
fs.mkdirSync(dist, { recursive: true });
fs.copyFileSync(src, out);
fs.chmodSync(out, 0o755);
