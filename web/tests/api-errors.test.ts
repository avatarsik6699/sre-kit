import { describe, expect, it } from "vitest";
import { ApiError, normalizeApiFailure } from "~/shared/api/errors";

describe("normalizeApiFailure", () => {
  it("passes an ApiError through unchanged", () => {
    const original = new ApiError(404, "not found");
    expect(normalizeApiFailure(original)).toBe(original);
  });

  it("wraps a plain Error", () => {
    const normalized = normalizeApiFailure(new Error("network down"));
    expect(normalized).toBeInstanceOf(ApiError);
    expect(normalized.message).toBe("network down");
  });

  it("extracts the message from an apierror.Write-shaped body ({ error: string })", () => {
    const normalized = normalizeApiFailure({ error: "invalid password" });
    expect(normalized.message).toBe("invalid password");
    expect(normalized.body).toEqual({ error: "invalid password" });
  });

  it("falls back to a generic message for anything else", () => {
    const normalized = normalizeApiFailure(null);
    expect(normalized).toBeInstanceOf(ApiError);
    expect(normalized.message).toBe("Unknown API error");
  });
});
