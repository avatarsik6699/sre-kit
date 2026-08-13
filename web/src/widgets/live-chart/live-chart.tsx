import { useState } from "react";
import { Group, SegmentedControl, Stack } from "@mantine/core";
import { LineChart } from "@mantine/charts";
import { mantineThemeConstants } from "~/shared/config/mantine-theme";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import { useMetricsQuery } from "~/entities/telemetry";
import type { MetricPoint } from "~/entities/telemetry";
import type { LiveChartTypes } from "./live-chart.types";

const WINDOW_MS: Record<LiveChartTypes.Window, number> = {
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
};

export const liveChartUtils = {
  filterByWindow(
    metrics: MetricPoint[],
    windowKey: LiveChartTypes.Window,
  ): MetricPoint[] {
    const cutoff = Date.now() - WINDOW_MS[windowKey];
    return metrics.filter((point) => new Date(point.ts).getTime() >= cutoff);
  },

  /** Pivots flat {name, ts, value} points into one record per timestamp, one column per metric
   * name — the shape @mantine/charts' LineChart expects for a multi-series time series. */
  pivotByTimestamp(metrics: MetricPoint[]): {
    rows: Record<string, unknown>[];
    names: string[];
  } {
    const rowsByTs = new Map<string, Record<string, unknown>>();
    const names = new Set<string>();
    for (const point of metrics) {
      names.add(point.name);
      const row = rowsByTs.get(point.ts) ?? { ts: point.ts };
      row[point.name] = point.value;
      rowsByTs.set(point.ts, row);
    }
    const rows = Array.from(rowsByTs.values()).sort(
      (a, b) =>
        new Date(a.ts as string).getTime() - new Date(b.ts as string).getTime(),
    );
    return { rows, names: Array.from(names) };
  },
};

const SERIES_COLORS = [
  "accentPrimary",
  "statusOk",
  "statusWarn",
  "statusCritical",
];

/**
 * Full-width live time-series chart with a 24h/7d historical toggle (docs/SPEC.md §5.1/§5.2).
 * Purely reads the metrics query cache — the caller (pages/source-detail) owns the single
 * useLiveTelemetry(sourceId) subscription for its whole page, so cache writes aren't duplicated
 * across this and any sibling widget reading the same sourceId.
 */
export const LiveChart: React.FC<LiveChartTypes.Props> = (props) => {
  const [windowKey, setWindowKey] = useState<LiveChartTypes.Window>("24h");
  const metricsQuery = useMetricsQuery(props.sourceId);

  const filtered = liveChartUtils.filterByWindow(
    metricsQuery.data ?? [],
    windowKey,
  );
  const pivoted = liveChartUtils.pivotByTimestamp(filtered);

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <Typography variant="title" order={3}>
          Metrics
        </Typography>
        <SegmentedControl
          value={windowKey}
          onChange={(value) => setWindowKey(value as LiveChartTypes.Window)}
          data={[
            { label: "24h", value: "24h" },
            { label: "7d", value: "7d" },
          ]}
        />
      </Group>
      {pivoted.rows.length > 0 ? (
        <div style={{ fontFamily: mantineThemeConstants.fontFamilyMonospace }}>
          <LineChart
            h={320}
            data={pivoted.rows}
            dataKey="ts"
            series={pivoted.names.map((name, index) => ({
              name,
              color: SERIES_COLORS[index % SERIES_COLORS.length],
            }))}
            withDots={false}
            curveType="monotone"
          />
        </div>
      ) : (
        <EmptyState
          title="No metrics yet"
          description="Data will appear once the source reports."
        />
      )}
    </Stack>
  );
};
