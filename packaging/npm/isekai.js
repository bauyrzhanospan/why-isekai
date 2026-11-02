#!/usr/bin/env node
// Launcher for the prebuilt isekai binary installed as an optional
// dependency (why-isekai-<platform>-<arch>).
"use strict";
const { spawnSync } = require("child_process");
const path = require("path");

const key = `${process.platform}-${process.arch}`;
const pkg = `why-isekai-${key}`;
const exe = process.platform === "win32" ? "isekai.exe" : "isekai";

let bin = process.env.ISEKAI_BINARY;
if (!bin) {
  try {
    bin = path.join(path.dirname(require.resolve(`${pkg}/package.json`)), "bin", exe);
  } catch (e) {
    console.error(
      `why-isekai: no prebuilt isekai for ${key} (npm package ${pkg} is not installed).\n` +
        "  Supported: linux, darwin, win32 on x64 and arm64.\n" +
        "  If you installed with --no-optional / --omit=optional, reinstall without it.\n" +
        "  Other options: go install github.com/bauyrzhanospan/why-isekai/cmd/isekai@latest\n" +
        "                 curl -fsSL https://raw.githubusercontent.com/bauyrzhanospan/why-isekai/main/install.sh | sh"
    );
    process.exit(1);
  }
}

const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (r.error) {
  console.error(`why-isekai: cannot run ${bin}: ${r.error.message}`);
  process.exit(1);
}
if (r.signal) process.kill(process.pid, r.signal);
process.exit(r.status === null ? 1 : r.status);
