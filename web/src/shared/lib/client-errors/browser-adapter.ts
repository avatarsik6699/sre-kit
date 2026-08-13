// The one place in shared/lib/client-errors allowed to touch `window` (docs/STACK.md § Frontend
// tooling's no-restricted-globals allow-list) — subscribes to global `error`/`unhandledrejection`
// events and forwards them through a ClientErrorReporter. Kept apart from reporter.ts so the
// dedupe/fingerprint logic stays framework-agnostic and unit-testable without jsdom event wiring.
import type { ClientErrorReporter } from "./reporter";

function toReportableError(reason: unknown): {
  message: string;
  stack?: string;
} {
  if (reason instanceof Error) {
    return { message: reason.message, stack: reason.stack };
  }
  return { message: String(reason) };
}

/** Subscribes reporter to window error events; returns an unsubscribe function. */
export function attachBrowserErrorListeners(
  reporter: ClientErrorReporter,
): () => void {
  function handleWindowError(event: ErrorEvent): void {
    reporter.report(
      event.error instanceof Error
        ? toReportableError(event.error)
        : { message: event.message },
    );
    // Suppresses the browser's default "log to devtools console" action — we've already
    // forwarded it to our own sink, so the default action would just be a duplicate.
    event.preventDefault();
  }

  function handleUnhandledRejection(event: PromiseRejectionEvent): void {
    reporter.report(toReportableError(event.reason));
    event.preventDefault();
  }

  window.addEventListener("error", handleWindowError);
  window.addEventListener("unhandledrejection", handleUnhandledRejection);

  return function detachBrowserErrorListeners(): void {
    window.removeEventListener("error", handleWindowError);
    window.removeEventListener("unhandledrejection", handleUnhandledRejection);
  };
}
