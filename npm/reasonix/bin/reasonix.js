#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const pkg = `@reasonix/cli-${process.platform}-${process.arch}`;
const exe = `reasonix${process.platform === "win32" ? ".exe" : ""}`;

function resolveBinary() {
  if (process.env.REASONIX_BINARY && fs.existsSync(process.env.REASONIX_BINARY)) {
    return process.env.REASONIX_BINARY;
  }
  const candidates = [pkg];
  if (process.platform === "android") {
    candidates.push(`@reasonix/cli-linux-${process.arch}`);
    if (process.arch === "arm") {
      candidates.push("@reasonix/cli-android-armv7", "@reasonix/cli-linux-armv7");
    }
  } else if (process.platform === "linux" && process.arch === "arm") {
    candidates.push("@reasonix/cli-linux-armv7", "@reasonix/cli-android-arm", "@reasonix/cli-android-armv7");
  }
  for (const candidate of candidates) {
    try {
      return require.resolve(`${candidate}/bin/${exe}`);
    } catch {
      // Try next candidate.
    }
  }
  if (process.platform === "android" || process.env.TERMUX_VERSION || (process.env.PREFIX && process.env.PREFIX.includes("com.termux"))) {
    const prefix = process.env.PREFIX || "/data/data/com.termux/files/usr";
    const termuxBin = path.join(prefix, "bin", exe);
    if (fs.existsSync(termuxBin) && path.resolve(termuxBin) !== path.resolve(__filename)) {
      return termuxBin;
    }
  }
  return null;
}

const binary = resolveBinary();
if (!binary) {
  console.error(
    `reasonix: no prebuilt binary for ${process.platform}-${process.arch}.\n` +
      `Install the matching optional package (${pkg}), or build from source:\n` +
      `  https://github.com/esengine/DeepSeek-Reasonix`,
  );
  process.exit(1);
}

const res = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
if (res.error) throw res.error;
process.exit(res.status === null ? 1 : res.status);
