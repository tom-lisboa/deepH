#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const path = require("node:path");
const https = require("node:https");

const pkg = require("../package.json");

const OWNER = process.env.DEEPH_GITHUB_OWNER || "tom-lisboa";
const REPO = process.env.DEEPH_GITHUB_REPO || "deepH";
const TAG = process.env.DEEPH_RELEASE_TAG || `v${pkg.version}`;

const PLATFORM_MAP = {
  "darwin-arm64": "deeph-darwin-arm64",
  "darwin-x64": "deeph-darwin-amd64",
  "linux-arm64": "deeph-linux-arm64",
  "linux-x64": "deeph-linux-amd64",
  "win32-arm64": "deeph-windows-arm64.exe",
  "win32-x64": "deeph-windows-amd64.exe"
};

const platformKey = `${process.platform}-${process.arch}`;
const assetName = PLATFORM_MAP[platformKey];
if (!assetName) {
  console.error(`[deeph] unsupported platform: ${platformKey}`);
  process.exit(1);
}

const isWindows = process.platform === "win32";
const outDir = path.join(__dirname, "..", "vendor");
const outFile = path.join(outDir, isWindows ? "deeph.exe" : "deeph");
const tmpFile = `${outFile}.tmp`;

const url = `https://github.com/${OWNER}/${REPO}/releases/download/${TAG}/${assetName}`;

ensureDir(outDir);

download(url, tmpFile)
  .then(() => {
    fs.renameSync(tmpFile, outFile);
    if (!isWindows) {
      fs.chmodSync(outFile, 0o755);
    }
    console.log(`[deeph] installed binary: ${path.basename(outFile)} (${platformKey})`);
  })
  .catch((err) => {
    try {
      if (fs.existsSync(tmpFile)) fs.unlinkSync(tmpFile);
    } catch (_) {
      // best effort cleanup
    }
    console.error(`[deeph] install failed: ${err.message}`);
    console.error(`[deeph] release: ${TAG}`);
    console.error(`[deeph] expected asset: ${assetName}`);
    process.exit(1);
  });

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function download(urlString, destination) {
  return new Promise((resolve, reject) => {
    const req = https.get(urlString, { headers: { "User-Agent": "deeph-cli-installer" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        return resolve(download(res.headers.location, destination));
      }
      if (res.statusCode !== 200) {
        const chunks = [];
        res.on("data", (d) => chunks.push(d));
        res.on("end", () => {
          reject(new Error(`download failed with status ${res.statusCode}: ${Buffer.concat(chunks).toString("utf8").slice(0, 240)}`));
        });
        return;
      }
      const file = fs.createWriteStream(destination, { mode: 0o755 });
      res.pipe(file);
      file.on("finish", () => file.close(resolve));
      file.on("error", (err) => reject(err));
    });
    req.on("error", reject);
  });
}
