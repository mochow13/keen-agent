#!/usr/bin/env node
// Shim: locates the downloaded keen-agent binary and passes through all arguments.

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const binaryName = process.platform === "win32" ? "keen-agent.exe" : "keen-agent";
const binaryPath = path.join(__dirname, binaryName);

if (!fs.existsSync(binaryPath)) {
  console.error(
    "[keen-agent] Binary not found. Try reinstalling: npm install -g keen-agent@latest\n" +
      "[keen-agent] If updating an existing global install, run: npm update -g keen-agent"
  );
  process.exit(1);
}

try {
  execFileSync(binaryPath, process.argv.slice(2), { stdio: "inherit" });
} catch (err) {
  process.exit(err.status ?? 1);
}
