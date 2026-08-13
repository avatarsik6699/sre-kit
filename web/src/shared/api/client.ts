// The one place allowed to call `fetch` (docs/STACK.md § Frontend tooling's no-restricted-globals
// allow-list) — an openapi-fetch instance typed against the generated OpenAPI schema (schema.ts,
// I12's pipeline). Single-origin app in the browser (docs/SPEC.md §8) — relative URLs resolve
// against the page origin there, and `credentials: "include"` attaches the session cookie
// (docs/SPEC.md §6) from the browser's own cookie jar. During SSR (route beforeLoad/loaders run
// server-side too, e.g. routes/_authenticated.tsx), Node's fetch has no page origin (needs an
// absolute baseUrl) and no browser cookie jar (the session cookie must be forwarded by hand from
// the incoming request) — createIsomorphicFn (TanStack Start's execution-model primitive) picks
// the right branch per environment and keeps the server branch's `process.env`/request access out
// of the client bundle.
import createClient from "openapi-fetch";
import { createIsomorphicFn } from "@tanstack/react-start";
import { getRequestHeader } from "@tanstack/react-start/server";
import type { paths } from "./schema";

const getBaseUrl = createIsomorphicFn()
  .server(
    () => process.env.SRE_KIT_INTERNAL_API_ORIGIN ?? "http://127.0.0.1:8080",
  )
  .client(() => undefined);

export const apiClient = createClient<paths>({
  baseUrl: getBaseUrl(),
  credentials: "include", // session cookie (docs/SPEC.md §6)
});

const forwardSsrCookie = createIsomorphicFn()
  .server(() => {
    apiClient.use({
      onRequest({ request }) {
        const cookie = getRequestHeader("cookie");
        if (cookie) request.headers.set("cookie", cookie);
        return request;
      },
    });
  })
  .client(() => {});
forwardSsrCookie();
