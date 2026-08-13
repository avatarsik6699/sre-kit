#!/usr/bin/env node
// Orchestrates API contract generation (docs/STACK.md § API contract generation):
//   1. Go side: `go generate ./...` runs swaggo/swag (see the //go:generate directive in
//      cmd/server/main.go) into a throwaway staging dir. swag emits Swagger 2.0, which
//      `openapi-typescript` (step 2) can't read, so `swagger2openapi` converts it to OpenAPI 3.x
//      before this script writes it out as the committed contracts/openapi.json.
//   2. Frontend side: `openapi-typescript` turns contracts/openapi.json into
//      web/src/shared/api/schema.ts — skipped gracefully if web/ isn't scaffolded yet (I3+).
//
// Usage:
//   node scripts/api-contracts.mjs          regenerate contracts/openapi.json (+ web schema.ts)
//   node scripts/api-contracts.mjs --check   generate into a temp dir, diff against the committed
//                                            files, exit 1 on drift without touching them

import { execFileSync } from "node:child_process";
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const check = process.argv.includes("--check");

function run(cmd, args, cwd) {
  execFileSync(cmd, args, { cwd, stdio: "inherit" });
}

// generateOpenAPI runs swag (Swagger 2.0) then swagger2openapi (-> OpenAPI 3.x) into stagingDir
// and returns the pretty-printed openapi.json text.
function generateOpenAPI(stagingDir) {
  run(
    "swag",
    [
      "init",
      "-g",
      "cmd/server/main.go",
      "--output",
      stagingDir,
      "--parseInternal",
      "--parseDependency",
    ],
    repoRoot,
  );

  const openapi3Path = join(stagingDir, "openapi3.json");
  run(
    "npx",
    [
      "--yes",
      "swagger2openapi",
      join(stagingDir, "swagger.json"),
      "-o",
      openapi3Path,
    ],
    repoRoot,
  );

  const raw = readFileSync(openapi3Path, "utf8");
  return JSON.stringify(JSON.parse(raw), null, 2) + "\n";
}

function generateSchemaTS(openapiJSONText) {
  const webDir = join(repoRoot, "web");
  if (!existsSync(join(webDir, "package.json"))) {
    console.log(
      "api-contracts: web/ not scaffolded yet — skipping schema.ts generation",
    );
    return null;
  }
  const staging = mkdtempSync(join(tmpdir(), "sre-kit-schema-"));
  const inFile = join(staging, "openapi.json");
  const outFile = join(staging, "schema.ts");
  writeFileSync(inFile, openapiJSONText);
  run("npx", ["--yes", "openapi-typescript", inFile, "-o", outFile], webDir);
  const contents = readFileSync(outFile, "utf8");
  rmSync(staging, { recursive: true, force: true });
  return contents;
}

function main() {
  const staging = mkdtempSync(join(tmpdir(), "sre-kit-swag-"));
  let openapi;
  try {
    openapi = generateOpenAPI(staging);
  } finally {
    rmSync(staging, { recursive: true, force: true });
  }

  const openapiPath = join(repoRoot, "contracts", "openapi.json");
  const schemaPath = join(repoRoot, "web", "src", "shared", "api", "schema.ts");
  const schema = generateSchemaTS(openapi);

  if (!check) {
    writeFileSync(openapiPath, openapi);
    console.log(`api-contracts: wrote ${openapiPath}`);
    if (schema !== null) {
      writeFileSync(schemaPath, schema);
      console.log(`api-contracts: wrote ${schemaPath}`);
    }
    return;
  }

  // --check: compare freshly generated content against the committed files, write nothing.
  let drift = false;
  const committedOpenAPI = existsSync(openapiPath)
    ? readFileSync(openapiPath, "utf8")
    : null;
  if (committedOpenAPI !== openapi) {
    console.error(
      `api-contracts --check: ${openapiPath} is out of date — run without --check to regenerate`,
    );
    drift = true;
  }
  if (schema !== null) {
    const committedSchema = existsSync(schemaPath)
      ? readFileSync(schemaPath, "utf8")
      : null;
    if (committedSchema !== schema) {
      console.error(
        `api-contracts --check: ${schemaPath} is out of date — run without --check to regenerate`,
      );
      drift = true;
    }
  }

  if (drift) {
    process.exit(1);
  }
  console.log("api-contracts --check: no drift");
}

main();
