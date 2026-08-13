// Typed Playwright fixtures (docs/FRONTEND_CONVENTIONS.md §9): specs import `test`/`expect` from
// here, never from `@playwright/test` directly, and never instantiate Page Object Model classes
// themselves — POM construction lives in this file's fixtures. No POMs exist yet (no pages/
// features land until M4); add a fixture per docs/e2e/pages/*.page.ts as each one ships.
import { test as base, expect } from "@playwright/test";

type Fixtures = Record<string, never>;

export const test = base.extend<Fixtures>({});
export { expect };
