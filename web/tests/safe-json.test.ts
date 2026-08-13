import { afterEach, describe, expect, it } from "vitest";
import { safeJson } from "~/shared/lib/safe-json";

describe("safeJson", () => {
  afterEach(() => {
    window.localStorage.clear();
  });

  it("parses valid JSON", () => {
    expect(safeJson.parse('{"a":1}', {})).toEqual({ a: 1 });
  });

  it("falls back on malformed JSON instead of throwing", () => {
    expect(safeJson.parse("not json", { fallback: true })).toEqual({
      fallback: true,
    });
  });

  it("falls back when raw is null", () => {
    expect(safeJson.parse(null, "default")).toBe("default");
  });

  it("getItem/setItem round-trip through storage", () => {
    safeJson.setItem("prefs", { collapsed: true });
    expect(safeJson.getItem("prefs", { collapsed: false })).toEqual({
      collapsed: true,
    });
  });

  it("getItem returns fallback when nothing is stored", () => {
    expect(safeJson.getItem("missing", { collapsed: false })).toEqual({
      collapsed: false,
    });
  });
});
