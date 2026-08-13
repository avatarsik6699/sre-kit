import { describe, expect, it } from "vitest";
import { statusTileUtils } from "~/widgets/status-tile/status-tile";
import type { MetricPoint } from "~/entities/telemetry";

describe("statusTileUtils.pickSparklineSeries", () => {
  it("picks the metric name with the most points", () => {
    const metrics: MetricPoint[] = [
      { name: "cpu.usage_percent", ts: "2026-01-01T00:00:00Z", value: 1 },
      { name: "cpu.usage_percent", ts: "2026-01-01T00:01:00Z", value: 2 },
      { name: "disk.free_bytes", ts: "2026-01-01T00:00:00Z", value: 100 },
    ];
    expect(statusTileUtils.pickSparklineSeries(metrics)).toEqual([1, 2]);
  });

  it("returns an empty array for no metrics", () => {
    expect(statusTileUtils.pickSparklineSeries(undefined)).toEqual([]);
    expect(statusTileUtils.pickSparklineSeries([])).toEqual([]);
  });
});

describe("statusTileUtils.toPulseStatus", () => {
  it("maps error to critical", () => {
    expect(statusTileUtils.toPulseStatus("error")).toBe("critical");
  });

  it("passes through ok and unreachable", () => {
    expect(statusTileUtils.toPulseStatus("ok")).toBe("ok");
    expect(statusTileUtils.toPulseStatus("unreachable")).toBe("unreachable");
  });
});

describe("statusTileUtils.summarizeChecks", () => {
  it("reports no checks when empty", () => {
    expect(statusTileUtils.summarizeChecks([])).toBe("no checks");
    expect(statusTileUtils.summarizeChecks(undefined)).toBe("no checks");
  });

  it("counts statuses", () => {
    expect(statusTileUtils.summarizeChecks(["ok", "ok", "warn"])).toBe(
      "2 ok · 1 warn",
    );
  });
});
