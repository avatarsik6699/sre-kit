// ApiError + normalizeApiFailure (docs/FRONTEND_CONVENTIONS.md §7: *.dto.ts is reserved for
// transport shapes only — this file owns the transport-boundary error type instead).

export class ApiError extends Error {
  status: number;
  body: unknown;

  constructor(status: number, message: string, body?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.body = body;
  }
}

/**
 * Normalizes anything openapi-fetch's `error` field (or a thrown network failure) might be into a
 * single ApiError shape, so callers only ever branch on one error type. `status` should be passed
 * as the caller's `result.response.status` whenever available (openapi-fetch's `error` field
 * alone doesn't carry the HTTP status) — e.g. for route guards branching on 401.
 */
export function normalizeApiFailure(failure: unknown, status = 0): ApiError {
  if (failure instanceof ApiError) {
    return failure;
  }
  if (failure instanceof Error) {
    return new ApiError(status, failure.message);
  }
  if (isRecord(failure) && typeof failure.error === "string") {
    return new ApiError(status, failure.error, failure);
  }
  return new ApiError(status, "Unknown API error", failure);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
