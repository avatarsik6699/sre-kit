// @ts-check
// The FSD-layer-boundary `no-restricted-imports` group documented in docs/STACK.md § Frontend
// tooling lands once real page/feature/entity layers exist at M4 (there's nothing to bound yet —
// see docs/changes/01-core-skeleton.md). The no-restricted-globals group
// (window/document/navigator/localStorage/sessionStorage/fetch/process) is fully wired below,
// including shared/api/client.ts's `fetch` exemption (I13).
import js from "@eslint/js";
import playwright from "eslint-plugin-playwright";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";
import prettier from "eslint-config-prettier";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      "dist/**",
      ".output/**",
      ".vinxi/**",
      "node_modules/**",
      "src/routeTree.gen.ts",
      "src/shared/api/schema.ts",
      "playwright-report/**",
      "test-results/**",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    plugins: {
      react,
      "react-hooks": reactHooks,
    },
    languageOptions: {
      parserOptions: {
        ecmaFeatures: { jsx: true },
      },
    },
    settings: {
      react: { version: "detect" },
    },
    rules: {
      ...react.configs.recommended.rules,
      ...react.configs["jsx-runtime"].rules,
      ...reactHooks.configs.recommended.rules,
      "react/prop-types": "off",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_" },
      ],
    },
  },
  {
    // docs/FRONTEND_CONVENTIONS.md §7: namespace declarations are allowed only in *.types.ts
    // files (to qualify prop-type ownership, e.g. `ExamplePageTypes.Props`) — everywhere else
    // stays on tseslint's recommended `no-namespace` ban.
    files: ["**/*.types.ts"],
    rules: {
      "@typescript-eslint/no-namespace": "off",
    },
  },
  {
    // docs/STACK.md § Frontend tooling: raw browser/Node globals are banned everywhere in src/
    // except the one owning adapter file for each — narrowed back open just below.
    files: ["src/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-globals": [
        "error",
        {
          name: "window",
          message:
            "Use shared/lib/safe-ls (storage) or shared/lib/client-errors (error capture).",
        },
        {
          name: "document",
          message:
            "Use shared/lib/safe-ls (storage) or shared/lib/client-errors (error capture).",
        },
        {
          name: "navigator",
          message:
            "Use shared/lib/safe-ls (storage) or shared/lib/client-errors (error capture).",
        },
        { name: "localStorage", message: "Use shared/lib/safe-ls." },
        { name: "sessionStorage", message: "Use shared/lib/safe-ls." },
        { name: "fetch", message: "Use shared/api/client." },
        {
          name: "process",
          message: "Use shared/config/client-env.ts (or a *.server.ts module).",
        },
      ],
    },
  },
  {
    // Owning adapters, allow-listed back open (docs/STACK.md § Frontend tooling).
    files: [
      "src/shared/lib/safe-ls.ts",
      "src/shared/lib/client-errors/browser-adapter.ts",
      "src/shared/api/client.ts",
    ],
    rules: {
      "no-restricted-globals": "off",
    },
  },
  {
    files: ["e2e/**/*.{ts,tsx}"],
    plugins: { playwright },
    rules: {
      ...playwright.configs["flat/recommended"].rules,
    },
  },
  {
    // Plain Node tooling scripts (not application code, not type-checked as part of the app).
    files: ["scripts/**/*.{mjs,cjs}", "*.config.{js,mjs,ts}"],
    languageOptions: {
      globals: globals.node,
    },
  },
  prettier,
);
