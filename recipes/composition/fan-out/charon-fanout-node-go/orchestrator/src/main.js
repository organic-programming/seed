#!/usr/bin/env node
const { spawnSync } = require("node:child_process");
const path = require("node:path");

const script = process.env.CHARON_RUN_SCRIPT || path.join(__dirname, "scripts", "run.sh");
const result = spawnSync("/bin/sh", [script], { stdio: "inherit" });
process.exit(result.status ?? 1);
