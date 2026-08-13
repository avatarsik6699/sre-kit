import { afterEach, describe, expect, it } from "vitest";
import { safeLs } from "~/shared/lib/safe-ls";

describe("safeLs", () => {
  afterEach(() => {
    window.localStorage.clear();
  });

  it("round-trips a value through set/get", () => {
    safeLs.set("theme", "dark");
    expect(safeLs.get("theme")).toBe("dark");
  });

  it("returns null for a key that was never set", () => {
    expect(safeLs.get("missing")).toBeNull();
  });

  it("removes a value", () => {
    safeLs.set("theme", "dark");
    safeLs.remove("theme");
    expect(safeLs.get("theme")).toBeNull();
  });

  it("namespaces keys so they don't collide with unrelated localStorage entries", () => {
    safeLs.set("theme", "dark");
    expect(window.localStorage.getItem("theme")).toBeNull();
  });
});
