#!/usr/bin/env node

const { spawnSync } = require("node:child_process");
const path = require("node:path");
const { createRequire } = require("node:module");

const requireFromHere = createRequire(__filename);

const packageMap = {
  "darwin:arm64": "@organic-programming/op-darwin-arm64",
  "darwin:x64": "@organic-programming/op-darwin-x64",
  "linux:arm64": "@organic-programming/op-linux-arm64",
  "linux:x64": "@organic-programming/op-linux-x64",
  "win32:x64": "@organic-programming/op-win32-x64"
};

const key = `${process.platform}:${process.arch}`;
const packageName = packageMap[key];

if (!packageName) {
  console.error(`@organic-programming/op: unsupported platform ${key}`);
  process.exit(1);
}

let packageJSONPath;
try {
  packageJSONPath = requireFromHere.resolve(`${packageName}/package.json`);
} catch (error) {
  console.error(`@organic-programming/op: ${packageName} is not installed`);
  process.exit(1);
}

const binaryName = process.platform === "win32" ? "op.exe" : "op";
const binaryPath = path.join(path.dirname(packageJSONPath), "bin", binaryName);
const result = spawnSync(binaryPath, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
