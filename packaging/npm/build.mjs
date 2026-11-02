#!/usr/bin/env node
// Builds the npm packages from GoReleaser archives, esbuild-style:
//   why-isekai                 tiny launcher, optionalDependencies on all of:
//   why-isekai-<os>-<cpu>      one prebuilt static binary each; npm installs
//                              only the one matching the user's os/cpu.
//
//   node build.mjs --version 0.1.0 --dist ../../dist --out out
//   node build.mjs --publish --out out
//   node build.mjs --self-test
import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const MAIN = "why-isekai";
const REPO = "https://github.com/bauyrzhanospan/why-isekai";

// GoReleaser os/arch -> npm os/cpu
export const PLATFORMS = [
  { goos: "linux", goarch: "amd64", os: "linux", cpu: "x64" },
  { goos: "linux", goarch: "arm64", os: "linux", cpu: "arm64" },
  { goos: "darwin", goarch: "amd64", os: "darwin", cpu: "x64" },
  { goos: "darwin", goarch: "arm64", os: "darwin", cpu: "arm64" },
  { goos: "windows", goarch: "amd64", os: "win32", cpu: "x64" },
  { goos: "windows", goarch: "arm64", os: "win32", cpu: "arm64" },
];

const pkgName = (p) => `${MAIN}-${p.os}-${p.cpu}`;

function args() {
  const a = process.argv.slice(2);
  const o = {};
  for (let i = 0; i < a.length; i++) {
    if (!a[i].startsWith("--")) continue;
    const k = a[i].slice(2);
    if (a[i + 1] && !a[i + 1].startsWith("--")) o[k] = a[++i];
    else o[k] = true;
  }
  return o;
}

function write(file, content, mode) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, content);
  if (mode) fs.chmodSync(file, mode);
}

function extract(archive, exe, dest) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "isekai-npm-"));
  if (archive.endsWith(".zip")) execFileSync("unzip", ["-q", archive, "-d", tmp]);
  else execFileSync("tar", ["-xzf", archive, "-C", tmp]);
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.copyFileSync(path.join(tmp, exe), dest);
  fs.chmodSync(dest, 0o755);
  fs.rmSync(tmp, { recursive: true, force: true });
}

const common = (version) => ({
  version,
  license: "MIT",
  homepage: REPO,
  repository: { type: "git", url: `git+${REPO}.git` },
  keywords: ["ai-agents", "git", "commit-trailers", "agents-md", "llm", "isekai"],
});

export function build({ version, dist, out, platforms = PLATFORMS }) {
  if (!version) throw new Error("--version is required");
  fs.rmSync(out, { recursive: true, force: true });
  const optional = {};
  for (const p of platforms) {
    const exe = p.goos === "windows" ? "isekai.exe" : "isekai";
    const ext = p.goos === "windows" ? "zip" : "tar.gz";
    const archive = path.join(dist, `isekai_${version}_${p.goos}_${p.goarch}.${ext}`);
    if (!fs.existsSync(archive)) throw new Error(`missing ${archive}`);
    const dir = path.join(out, pkgName(p));
    extract(archive, exe, path.join(dir, "bin", exe));
    write(path.join(dir, "package.json"), JSON.stringify({
      name: pkgName(p),
      description: `isekai binary for ${p.os}-${p.cpu} (installed automatically by ${MAIN})`,
      ...common(version),
      os: [p.os],
      cpu: [p.cpu],
      files: ["bin"],
      preferUnplugged: true,
    }, null, 2) + "\n");
    write(path.join(dir, "README.md"), `Prebuilt \`isekai\` for ${p.os}-${p.cpu}. Install [${MAIN}](${REPO}) instead.\n`);
    optional[pkgName(p)] = version;
  }
  const main = path.join(out, MAIN);
  write(path.join(main, "package.json"), JSON.stringify({
    name: MAIN,
    description: "Project memory for AI agents, kept in git: Why/Tried/Next/Done/Agent commit trailers, secrets by reference, project wiki.",
    ...common(version),
    bin: { isekai: "bin/isekai.js", "why-isekai": "bin/isekai.js" },
    files: ["bin", "README.md"],
    optionalDependencies: optional,
    engines: { node: ">=16" },
  }, null, 2) + "\n");
  fs.mkdirSync(path.join(main, "bin"), { recursive: true });
  fs.copyFileSync(path.join(here, "isekai.js"), path.join(main, "bin", "isekai.js"));
  fs.chmodSync(path.join(main, "bin", "isekai.js"), 0o755);
  const readme = path.join(here, "..", "..", "README.md");
  if (fs.existsSync(readme)) fs.copyFileSync(readme, path.join(main, "README.md"));
  return { main, platforms: platforms.map((p) => path.join(out, pkgName(p))) };
}

function publish(out) {
  const dirs = fs.readdirSync(out).sort((a, b) => (a === MAIN) - (b === MAIN)); // platforms first
  for (const d of dirs) {
    const { version } = JSON.parse(fs.readFileSync(path.join(out, d, "package.json"), "utf8"));
    // Pre-releases (0.2.0-rc.1) must not become "latest".
    const tag = version.includes("-") ? ["--tag", "next"] : [];
    const r = spawnSync("npm", ["publish", "--access", "public", "--provenance", ...tag], { cwd: path.join(out, d), stdio: "inherit" });
    if (r.status !== 0) process.exit(r.status ?? 1);
  }
}

// Builds from a fake archive and runs the launcher through a node_modules layout.
function selfTest() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "isekai-npm-test-"));
  const dist = path.join(tmp, "dist");
  const plat = PLATFORMS.find((p) => p.os === process.platform && p.cpu === process.arch);
  if (!plat || plat.goos === "windows") {
    console.log("self-test skipped on this platform");
    return;
  }
  const stage = path.join(tmp, "stage");
  write(path.join(stage, "isekai"), "#!/bin/sh\necho \"fake isekai $*\"\nexit 3\n", 0o755);
  fs.mkdirSync(dist);
  execFileSync("tar", ["-czf", path.join(dist, `isekai_9.9.9_${plat.goos}_${plat.goarch}.tar.gz`), "-C", stage, "isekai"]);
  const { main, platforms } = build({ version: "9.9.9", dist, out: path.join(tmp, "out"), platforms: [plat] });
  const nm = path.join(tmp, "app", "node_modules");
  fs.mkdirSync(nm, { recursive: true });
  fs.cpSync(main, path.join(nm, MAIN), { recursive: true });
  fs.cpSync(platforms[0], path.join(nm, pkgName(plat)), { recursive: true });
  const r = spawnSync(process.execPath, [path.join(nm, MAIN, "bin", "isekai.js"), "brief", "-n", "1"], { encoding: "utf8" });
  if (r.status !== 3 || r.stdout.trim() !== "fake isekai brief -n 1") {
    console.error("self-test failed", r);
    process.exit(1);
  }
  const pj = JSON.parse(fs.readFileSync(path.join(nm, MAIN, "package.json"), "utf8"));
  if (pj.optionalDependencies[pkgName(plat)] !== "9.9.9") throw new Error("optionalDependencies");
  fs.rmSync(tmp, { recursive: true, force: true });
  console.log("npm packaging self-test ok");
}

const o = args();
if (o["self-test"]) selfTest();
else if (o.publish) publish(o.out || path.join(here, "out"));
else {
  const res = build({ version: o.version, dist: o.dist || path.join(here, "..", "..", "dist"), out: o.out || path.join(here, "out") });
  console.log(`built ${res.platforms.length + 1} packages in ${path.dirname(res.main)}`);
}
