#!/usr/bin/env node
"use strict";

/**
 * Launcher for the platform binary.
 *
 * The real program is a Go executable shipped in one of the optional
 * per-platform packages; npm installs only the one matching this machine's
 * os/cpu. This shim finds it and hands over.
 *
 * stdio is inherited rather than piped: reharvester is a full-screen terminal
 * UI, and it needs the real TTY for input, resize events and colour detection.
 */

const { spawnSync } = require("node:child_process");

const PLATFORMS = {
  "darwin-arm64": "reharvester-darwin-arm64",
  "darwin-x64": "reharvester-darwin-x64",
  "linux-arm64": "reharvester-linux-arm64",
  "linux-x64": "reharvester-linux-x64",
  "win32-x64": "reharvester-win32-x64",
  "win32-arm64": "reharvester-win32-arm64",
};

function resolveBinary() {
  const key = `${process.platform}-${process.arch}`;
  const pkg = PLATFORMS[key];
  if (!pkg) {
    fail(
      `No reharvester build for ${key}.`,
      "Supported: " + Object.keys(PLATFORMS).join(", "),
      "Build from source instead: https://github.com/yothgewalt/reharvester"
    );
  }
  const exe = process.platform === "win32" ? "reharvester.exe" : "reharvester";
  try {
    return require.resolve(`${pkg}/bin/${exe}`);
  } catch {
    fail(
      `The ${pkg} package is not installed.`,
      "npm skips optional dependencies when installed with --no-optional,",
      "and some lockfiles omit them. Reinstall with:",
      "",
      "  npm install -g reharvester --force"
    );
  }
}

function fail(...lines) {
  console.error("reharvester: " + lines.join("\n"));
  process.exit(1);
}

const result = spawnSync(resolveBinary(), process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: false,
});

if (result.error) {
  fail(`could not start the binary: ${result.error.message}`);
}
// A signalled child reports null status; mirror the shell's 128+signal
// convention so callers can tell a crash from a clean non-zero exit.
if (result.status === null && result.signal) {
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
