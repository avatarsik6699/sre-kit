// SSR-safe localStorage wrapper — the only module allowed to touch window/localStorage directly
// (docs/FRONTEND_CONVENTIONS.md §6; ESLint's no-restricted-globals allow-list for this file is
// wired in I9 alongside the rest of the no-restricted-globals policy group). Versioned so a future
// key-shape change can be migrated instead of silently misreading old data.

const STORAGE_VERSION = "1";

function isBrowser(): boolean {
  return typeof window !== "undefined";
}

function versionedKey(key: string): string {
  return `sre-kit:v${STORAGE_VERSION}:${key}`;
}

export const safeLs = {
  get(key: string): string | null {
    if (!isBrowser()) {
      return null;
    }
    try {
      return window.localStorage.getItem(versionedKey(key));
    } catch {
      return null;
    }
  },

  set(key: string, value: string): void {
    if (!isBrowser()) {
      return;
    }
    try {
      window.localStorage.setItem(versionedKey(key), value);
    } catch {
      // Storage full or blocked (private browsing) — losing a persisted preference isn't fatal.
    }
  },

  remove(key: string): void {
    if (!isBrowser()) {
      return;
    }
    try {
      window.localStorage.removeItem(versionedKey(key));
    } catch {
      // See set() above.
    }
  },
};
