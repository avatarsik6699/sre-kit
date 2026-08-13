import { Card, Group, Stack } from "@mantine/core";
import { Sparkline } from "@mantine/charts";
import { Link } from "@tanstack/react-router";
import {
  StatusPulse,
  type StatusPulseStatus,
} from "~/shared/components/status-pulse";
import { Typography } from "~/shared/components/typography";
import {
  useChecksQuery,
  useLiveTelemetry,
  useMetricsQuery,
} from "~/entities/telemetry";
import type { MetricPoint } from "~/entities/telemetry";
import type { StatusTileTypes } from "./status-tile.types";

export const statusTileUtils = {
  /** Picks the metric name with the most recent points and returns its last-20 values, since a
   * source can emit several unrelated metric names (e.g. cpu/ram/disk) and a sparkline needs one
   * consistent series. */
  pickSparklineSeries(metrics: MetricPoint[] | undefined): number[] {
    if (!metrics || metrics.length === 0) {
      return [];
    }
    const countByName = new Map<string, number>();
    for (const point of metrics) {
      countByName.set(point.name, (countByName.get(point.name) ?? 0) + 1);
    }
    let leadingName = metrics[0].name;
    let leadingCount = 0;
    for (const [name, count] of countByName) {
      if (count > leadingCount) {
        leadingName = name;
        leadingCount = count;
      }
    }
    return metrics
      .filter((point) => point.name === leadingName)
      .slice(-20)
      .map((point) => point.value);
  },

  toPulseStatus(lastStatus: string): StatusPulseStatus {
    if (lastStatus === "ok" || lastStatus === "unreachable") {
      return lastStatus;
    }
    return lastStatus === "error" ? "critical" : "unreachable";
  },

  summarizeChecks(statuses: string[] | undefined): string {
    if (!statuses || statuses.length === 0) {
      return "no checks";
    }
    const counts = new Map<string, number>();
    for (const status of statuses) {
      counts.set(status, (counts.get(status) ?? 0) + 1);
    }
    return Array.from(counts.entries())
      .map(([status, count]) => `${count} ${status}`)
      .join(" · ");
  },
};

/** Renders one source's live status — reused on Dashboard and Sources (docs/SPEC.md §5.2). */
export const StatusTile: React.FC<StatusTileTypes.Props> = (props) => {
  useLiveTelemetry(props.source.id);
  const metricsQuery = useMetricsQuery(props.source.id);
  const checksQuery = useChecksQuery(props.source.id);

  const sparklineData = statusTileUtils.pickSparklineSeries(metricsQuery.data);
  const checkSummary = statusTileUtils.summarizeChecks(
    checksQuery.data?.map((check) => check.status),
  );

  return (
    <Link
      to="/sources/$id"
      params={{ id: props.source.id }}
      style={{ textDecoration: "none", color: "inherit" }}
    >
      <Card withBorder padding="md">
        <Stack gap="xs">
          <Group justify="space-between" wrap="nowrap">
            <StatusPulse
              status={statusTileUtils.toPulseStatus(props.source.lastStatus)}
              label={props.source.adapterId}
            />
            {!props.source.enabled ? (
              <Typography c="dimmed">disabled</Typography>
            ) : null}
          </Group>
          {sparklineData.length > 1 ? (
            <Sparkline
              data={sparklineData}
              trendColors={{
                positive: "statusOk",
                negative: "statusCritical",
                neutral: "statusUnreachable",
              }}
              h={40}
            />
          ) : (
            <Typography c="dimmed">no metrics yet</Typography>
          )}
          <Typography c="dimmed">{checkSummary}</Typography>
        </Stack>
      </Card>
    </Link>
  );
};
