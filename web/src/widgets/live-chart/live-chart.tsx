import { useEffect, useRef, useState } from "react";
import uPlot, { type AlignedData, type Options } from "uplot";
import "uplot/dist/uPlot.min.css";
import { Button, Group, Stack } from "~/shared/ui";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import { useMetricsQuery } from "~/entities/telemetry";
import type { MetricPoint } from "~/entities/telemetry";
import type { LiveChartTypes } from "./live-chart.types";
import styles from "./live-chart.module.css";

const WINDOW_MS: Record<LiveChartTypes.Window, number> = {
  "24h": 86_400_000,
  "7d": 604_800_000,
};
const COLORS = ["#6387f5", "#49c889", "#e8b14e", "#ef6670", "#a78bfa"];

function seriesName(point: MetricPoint): string {
  const dimensions = Object.entries(point.labels ?? {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join(" · ");
  return dimensions ? `${point.name} · ${dimensions}` : point.name;
}

export const liveChartUtils = {
  filterByWindow(
    metrics: MetricPoint[],
    windowKey: LiveChartTypes.Window,
  ): MetricPoint[] {
    const cutoff = Date.now() - WINDOW_MS[windowKey];
    return metrics.filter((point) => new Date(point.ts).getTime() >= cutoff);
  },
  align(metrics: MetricPoint[]): { names: string[]; data: AlignedData } {
    const names = Array.from(new Set(metrics.map(seriesName)));
    const timestamps = Array.from(
      new Set(metrics.map((point) => Date.parse(point.ts) / 1000)),
    ).sort((a, b) => a - b);
    const values = new Map(
      metrics.map((point) => [
        `${Date.parse(point.ts) / 1000}:${seriesName(point)}`,
        point.value,
      ]),
    );
    return {
      names,
      data: [
        timestamps,
        ...names.map((name) =>
          timestamps.map((ts) => values.get(`${ts}:${name}`) ?? null),
        ),
      ] as AlignedData,
    };
  },
  pivotByTimestamp(metrics: MetricPoint[]): {
    rows: Record<string, unknown>[];
    names: string[];
  } {
    const rowsByTs = new Map<string, Record<string, unknown>>();
    const names = new Set<string>();
    for (const point of metrics) {
      const name = seriesName(point);
      names.add(name);
      const row = rowsByTs.get(point.ts) ?? { ts: point.ts };
      row[name] = point.value;
      rowsByTs.set(point.ts, row);
    }
    return {
      rows: Array.from(rowsByTs.values()).sort((a, b) =>
        String(a.ts).localeCompare(String(b.ts)),
      ),
      names: Array.from(names),
    };
  },
};

export const LiveChart: React.FC<LiveChartTypes.Props> = (props) => {
  const [windowKey, setWindowKey] = useState<LiveChartTypes.Window>("24h");
  const metricsQuery = useMetricsQuery(props.sourceId);
  const hostRef = useRef<HTMLDivElement>(null);
  const filtered = liveChartUtils.filterByWindow(
    metricsQuery.data ?? [],
    windowKey,
  );
  const aligned = liveChartUtils.align(filtered);
  const hasPlot = aligned.data[0].length >= 2;

  useEffect(
    function renderPlotFx() {
      const host = hostRef.current;
      if (!host || !hasPlot) return;
      const options: Options = {
        width: Math.max(host.clientWidth, 280),
        height: 300,
        cursor: { drag: { x: true, y: false } },
        scales: { x: { time: true } },
        axes: [
          { stroke: "#929dac", grid: { stroke: "#202833" } },
          { stroke: "#929dac", grid: { stroke: "#202833" } },
        ],
        series: [
          {},
          ...aligned.names.map((name, index) => ({
            label: name,
            stroke: COLORS[index % COLORS.length],
            width: 2,
          })),
        ],
      };
      const plot = new uPlot(options, aligned.data, host);
      const observer = new ResizeObserver((entries) => {
        const width = entries[0]?.contentRect.width;
        if (width) plot.setSize({ width: Math.max(width, 280), height: 300 });
      });
      observer.observe(host);
      return () => {
        observer.disconnect();
        plot.destroy();
      };
    },
    [aligned.data, aligned.names, hasPlot],
  );

  if (filtered.length === 0)
    return (
      <EmptyState
        title="No metrics yet"
        description="Data will appear once the source reports."
      />
    );
  const latest = aligned.names
    .map((name) => filtered.filter((point) => seriesName(point) === name).at(-1))
    .filter((point): point is MetricPoint => Boolean(point));
  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <Typography variant="title" order={3}>
          Metrics
        </Typography>
        <Group gap="xs">
          <Button
            variant={windowKey === "24h" ? undefined : "light"}
            onClick={() => setWindowKey("24h")}
          >
            24h
          </Button>
          <Button
            variant={windowKey === "7d" ? undefined : "light"}
            onClick={() => setWindowKey("7d")}
          >
            7d
          </Button>
        </Group>
      </Group>
      {hasPlot ? (
        <div ref={hostRef} className={styles.plot} aria-hidden="true" />
      ) : (
        <Typography c="dimmed">
          The time-series plot will appear after a second timestamp arrives.
        </Typography>
      )}
      <div className={styles.tableWrap}>
        <table>
          <caption>Latest values for the plotted series</caption>
          <thead>
            <tr>
              <th>Measurement</th>
              <th>Value</th>
              <th>Timestamp</th>
            </tr>
          </thead>
          <tbody>
            {latest.map((point) => (
              <tr key={`${seriesName(point)}:${point.ts}`}>
                <td>{seriesName(point)}</td>
                <td className="mono">{point.value.toLocaleString()}</td>
                <td className="mono">{point.ts}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Stack>
  );
};
