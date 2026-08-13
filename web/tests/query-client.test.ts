import { describe, expect, it } from "vitest";
import { createQueryClient } from "~/shared/lib/query-client";

describe("createQueryClient", () => {
  it("disables retry, matching the WS-push cache model (docs/SPEC.md §5.2)", () => {
    const client = createQueryClient();
    expect(client.getDefaultOptions().queries?.retry).toBe(false);
  });

  it("returns a fresh instance on every call", () => {
    expect(createQueryClient()).not.toBe(createQueryClient());
  });
});
