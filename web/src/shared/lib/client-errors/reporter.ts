// Core capture/fingerprint/dedupe logic for client-side errors. Framework-agnostic — no `window`
// access here (that's browser-adapter.ts's job) so this stays trivially unit-testable.

export type ClientError = {
  message: string;
  stack?: string;
  fingerprint: string;
  firstSeenAt: string;
  count: number;
};

export type ClientErrorSink = (error: ClientError) => void;

function fingerprintOf(message: string, stack?: string): string {
  const firstStackFrame = stack?.split("\n")[1]?.trim() ?? "";
  return `${message}::${firstStackFrame}`;
}

export type ClientErrorReporter = {
  report(input: { message: string; stack?: string }): void;
};

/**
 * Creates a reporter that dedupes repeated errors by fingerprint (message + top stack frame) and
 * forwards only the first occurrence to sink; later occurrences just increment `count` in place.
 */
export function createClientErrorReporter(
  sink: ClientErrorSink,
): ClientErrorReporter {
  const seen = new Map<string, ClientError>();

  return {
    report(input) {
      const fingerprint = fingerprintOf(input.message, input.stack);
      const existing = seen.get(fingerprint);
      if (existing) {
        existing.count += 1;
        return;
      }
      const error: ClientError = {
        message: input.message,
        stack: input.stack,
        fingerprint,
        firstSeenAt: new Date().toISOString(),
        count: 1,
      };
      seen.set(fingerprint, error);
      sink(error);
    },
  };
}

/**
 * Stubbed submit sink — logs locally only. No `/api/client-errors` backend endpoint exists in
 * this change (docs/changes/01-core-skeleton.md § Do NOT touch); swap this for a network sink
 * once a real endpoint lands.
 */
export const consoleSink: ClientErrorSink = (error) => {
  console.error("[client-error]", error);
};
