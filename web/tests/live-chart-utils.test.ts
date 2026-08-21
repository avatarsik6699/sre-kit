import { describe, expect, it } from "vitest";
import { liveChartUtils } from "~/widgets/live-chart/live-chart";
import type { MetricPoint } from "~/entities/telemetry";

describe("liveChartUtils.filterByWindow", () => {
  it("drops points older than the window", () => {
    const now = Date.now();
    const metrics: MetricPoint[] = [
      {
        name: "cpu",
        ts: new Date(now - 30 * 60 * 60 * 1000).toISOString(),
        value: 1,
      },
      {
        name: "cpu",
        ts: new Date(now - 1 * 60 * 60 * 1000).toISOString(),
        value: 2,
      },
    ];
    const filtered = liveChartUtils.filterByWindow(metrics, "24h");
    expect(filtered).toHaveLength(1);
    expect(filtered[0].value).toBe(2);
  });
});

describe("liveChartUtils.pivotByTimestamp", () => {
  it("groups points by timestamp into one row per name", () => {
    const metrics: MetricPoint[] = [
      { name: "cpu", ts: "t1", value: 1 },
      { name: "ram", ts: "t1", value: 2 },
      { name: "cpu", ts: "t2", value: 3 },
    ];
    const result = liveChartUtils.pivotByTimestamp(metrics);
    expect(result.names.sort()).toEqual(["cpu", "ram"]);
    expect(result.rows).toEqual([
      { ts: "t1", cpu: 1, ram: 2 },
      { ts: "t2", cpu: 3 },
    ]);
  });

  it("keeps labeled series with the same metric name separate", () => {
    const metrics: MetricPoint[] = [
      {
        name: "requests",
        ts: "2026-01-01T00:00:00Z",
        value: 10,
        labels: { traffic_class: "known_bot" },
      },
      {
        name: "requests",
        ts: "2026-01-01T00:00:00Z",
        value: 20,
        labels: { traffic_class: "unclassified" },
      },
    ];

    const result = liveChartUtils.align(metrics);

    expect(result.names).toEqual([
      "requests · traffic_class=known_bot",
      "requests · traffic_class=unclassified",
    ]);
    expect(result.data).toEqual([[1767225600], [10], [20]]);
  });
});
