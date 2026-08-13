// The one place allowed to call `fetch` (docs/STACK.md § Frontend tooling's no-restricted-globals
// allow-list) — an openapi-fetch instance typed against the generated OpenAPI schema (schema.ts,
// I12's pipeline). Single-origin app (docs/SPEC.md §8), so no base URL beyond same-origin "/".
import createClient from "openapi-fetch";
import type { paths } from "./schema";

export const apiClient = createClient<paths>({
  credentials: "include", // session cookie (docs/SPEC.md §6)
});
