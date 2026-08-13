// Helpers for persisted JSON (docs/FRONTEND_CONVENTIONS.md §6): parsing/serializing data that
// will be stored (e.g. via safe-ls). Plain JSON.stringify remains fine for HTTP request bodies —
// that's transport serialization, not storage — so this module is only for the storage path.
import { safeLs } from "~/shared/lib/safe-ls";

export const safeJson = {
  parse<T>(raw: string | null, fallback: T): T {
    if (raw === null) {
      return fallback;
    }
    try {
      return JSON.parse(raw) as T;
    } catch {
      return fallback;
    }
  },

  stringify(value: unknown): string {
    return JSON.stringify(value);
  },

  getItem<T>(key: string, fallback: T): T {
    return safeJson.parse(safeLs.get(key), fallback);
  },

  setItem(key: string, value: unknown): void {
    safeLs.set(key, safeJson.stringify(value));
  },
};
