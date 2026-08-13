#!/usr/bin/env node
// Self-test guarding the ESLint policy groups in eslint.config.js against silent regressions: it
// lints known-good and known-bad code snippets and asserts the right rule fires (or doesn't).
// docs/STACK.md § Frontend tooling. The FSD layer-boundary group has nothing to assert yet — it
// lands with real page/feature/entity content at M4.
import { ESLint } from "eslint";

const eslint = new ESLint({ cwd: process.cwd() });

/** @type {{ name: string, filePath: string, code: string, expectRuleId: string | null }[]} */
const cases = [
  {
    name: "bare `window` is banned in ordinary src/ code",
    filePath: "src/shared/components/example/example.tsx",
    code: "export function example() { return window.innerWidth; }\n",
    expectRuleId: "no-restricted-globals",
  },
  {
    name: "`window` is allowed in safe-ls.ts (its owning adapter)",
    filePath: "src/shared/lib/safe-ls.ts",
    code: "export function example() { return window.localStorage.getItem('x'); }\n",
    expectRuleId: null,
  },
  {
    name: "`window` is allowed in client-errors/browser-adapter.ts (its owning adapter)",
    filePath: "src/shared/lib/client-errors/browser-adapter.ts",
    code: "export function example() { return window.location.href; }\n",
    expectRuleId: null,
  },
  {
    name: "bare `fetch` is banned outside its owning adapter",
    filePath: "src/shared/components/example/example.tsx",
    code: "export function example() { return fetch('/api/x'); }\n",
    expectRuleId: "no-restricted-globals",
  },
  {
    name: "`fetch` is allowed in shared/api/client.ts (its owning adapter)",
    filePath: "src/shared/api/client.ts",
    code: "export function example() { return fetch('/api/x'); }\n",
    expectRuleId: null,
  },
  {
    name: "namespace declarations are allowed in *.types.ts files",
    filePath: "src/shared/components/example/example.types.ts",
    code: "export namespace ExampleTypes { export type Props = { title: string }; }\n",
    expectRuleId: null,
  },
  {
    name: "namespace declarations are banned outside *.types.ts files",
    filePath: "src/shared/components/example/example.ts",
    code: "export namespace ExampleTypes { export type Props = { title: string }; }\n",
    expectRuleId: "@typescript-eslint/no-namespace",
  },
];

let failed = false;

for (const testCase of cases) {
  const results = await eslint.lintText(testCase.code, {
    filePath: testCase.filePath,
  });
  const ruleIds = results.flatMap((result) =>
    result.messages.map((message) => message.ruleId),
  );

  const fired =
    testCase.expectRuleId !== null && ruleIds.includes(testCase.expectRuleId);
  const shouldFire = testCase.expectRuleId !== null;

  if (fired !== shouldFire) {
    failed = true;
    console.error(`✗ ${testCase.name}`);
    console.error(
      `  expected rule ${testCase.expectRuleId ?? "(none)"} to ${shouldFire ? "fire" : "NOT fire"}, got: ${JSON.stringify(ruleIds)}`,
    );
  } else {
    console.log(`✓ ${testCase.name}`);
  }
}

if (failed) {
  console.error("\nverify-app-architecture: policy regression detected");
  process.exit(1);
}
console.log("\nverify-app-architecture: all policy assertions passed");
