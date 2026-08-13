// The one place allowed to read import.meta.env directly (docs/FRONTEND_CONVENTIONS.md §6). No
// frontend-specific env vars exist yet — this is single-origin app (UI served by the same Go
// binary as the API, docs/SPEC.md §8), so there's no API base URL to configure. Add typed exports
// here as real vars are needed; don't read import.meta.env anywhere else.

export const clientEnvConstants = {
  isDev: import.meta.env.DEV,
  mode: import.meta.env.MODE,
};
