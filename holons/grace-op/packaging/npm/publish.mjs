#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { readdirSync, statSync } from "node:fs";
import path from "node:path";

const root = process.argv[2];

if (!root) {
  console.error("usage: node packaging/npm/publish.mjs <dist/npm>");
  process.exit(1);
}

const packageOrder = [
  "op-darwin-arm64",
  "op-darwin-x64",
  "op-linux-arm64",
  "op-linux-x64",
  "op-win32-x64",
  "op"
];

for (const name of packageOrder) {
  const cwd = path.join(root, name);
  if (!statSync(cwd).isDirectory()) {
    console.error(`missing package directory: ${cwd}`);
    process.exit(1);
  }
  const result = spawnSync("npm", ["publish", "--access", "public"], {
    cwd,
    stdio: "inherit"
  });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

for (const entry of readdirSync(root)) {
  if (!packageOrder.includes(entry)) {
    console.error(`unexpected package directory in publish root: ${entry}`);
    process.exit(1);
  }
}
