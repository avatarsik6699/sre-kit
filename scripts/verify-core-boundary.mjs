#!/usr/bin/env node

import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const openapi = JSON.parse(
  readFileSync(join(repoRoot, "contracts", "openapi.json"), "utf8"),
);

const requiredPaths = [
  "/api/sources",
  "/api/adapters",
  "/api/metrics",
  "/api/checks",
  "/api/events",
];
const retiredPrefixes = [
  "/api/hosts",
  "/api/presets",
  "/api/provisioning-runs",
  "/api/installations",
];

for (const path of requiredPaths) {
  if (!(path in openapi.paths)) {
    throw new Error(`core API path is missing: ${path}`);
  }
}

for (const path of Object.keys(openapi.paths)) {
  if (retiredPrefixes.some((prefix) => path.startsWith(prefix))) {
    throw new Error(`retired deployment API path is still published: ${path}`);
  }
}

const railNav = readFileSync(
  join(repoRoot, "web", "src", "widgets", "rail-nav", "rail-nav.tsx"),
  "utf8",
);
const routeTree = readFileSync(
  join(repoRoot, "web", "src", "routeTree.gen.ts"),
  "utf8",
);

for (const retiredText of ["/hosts", "Deploy", "ProvisioningRun"]) {
  if (railNav.includes(retiredText) || routeTree.includes(retiredText)) {
    throw new Error(`retired deployment UI remains visible: ${retiredText}`);
  }
}

console.log("core boundary: PASS");
