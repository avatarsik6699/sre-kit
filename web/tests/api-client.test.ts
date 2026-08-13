import { describe, expect, it } from "vitest";
import { apiClient } from "~/shared/api/client";

describe("apiClient", () => {
  it("exposes the openapi-fetch method surface", () => {
    expect(typeof apiClient.GET).toBe("function");
    expect(typeof apiClient.POST).toBe("function");
    expect(typeof apiClient.PATCH).toBe("function");
    expect(typeof apiClient.DELETE).toBe("function");
  });
});
