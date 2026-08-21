import {
  useChecksQuery,
  useMetricsQuery,
  type CheckStatus,
} from "~/entities/telemetry";
import type {
  PresentationGroup,
  PresentationMeasurement,
} from "~/entities/adapter";
import { Typography } from "~/shared/components/typography";
import type { TelemetrySummaryTypes } from "./telemetry-summary.types";
import styles from "./telemetry-summary.module.css";

type MetricPoint = NonNullable<ReturnType<typeof useMetricsQuery>["data"]>[number];

function descriptorFor(
  name: string,
  measurements: PresentationMeasurement[],
): PresentationMeasurement | undefined {
  return (
    measurements.find((measurement) => measurement.name === name) ??
    measurements.find((measurement) => measurement.name === "*")
  );
}

function displayValue(point: MetricPoint, descriptor?: PresentationMeasurement) {
  const value = point.value.toLocaleString(undefined, {
    maximumFractionDigits: 2,
  });
  if (descriptor?.unit === "seconds") return `${value} s`;
  if (descriptor?.unit === "percent") return `${value}%`;
  return value;
}

function groupsFor(
  points: MetricPoint[],
  declaredGroups: PresentationGroup[],
  measurements: PresentationMeasurement[],
) {
  const fallback: PresentationGroup = {
    id: "other",
    title: "Other measurements",
    order: Number.MAX_SAFE_INTEGER,
  };
  const groups = declaredGroups.length > 0 ? declaredGroups : [fallback];
  const knownIds = new Set(groups.map((group) => group.id));
  const grouped = new Map<string, MetricPoint[]>();
  for (const point of points) {
    const descriptor = descriptorFor(point.name, measurements);
    const groupId =
      descriptor?.group && knownIds.has(descriptor.group)
        ? descriptor.group
        : fallback.id;
    grouped.set(groupId, [...(grouped.get(groupId) ?? []), point]);
  }
  const result = groups
    .filter((group) => grouped.has(group.id))
    .sort((left, right) => (left.order ?? 0) - (right.order ?? 0));
  if (grouped.has(fallback.id) && !result.some((group) => group.id === fallback.id))
    result.push(fallback);
  return result.map((group) => ({ group, points: grouped.get(group.id) ?? [] }));
}

export const TelemetrySummary: React.FC<TelemetrySummaryTypes.Props> = (
  props,
) => {
  const metricsQuery = useMetricsQuery(props.sourceId);
  const checksQuery = useChecksQuery(props.sourceId);
  const latestBySeries = new Map<
    string,
    NonNullable<typeof metricsQuery.data>[number]
  >();
  for (const point of metricsQuery.data ?? []) {
    const key = `${point.name}:${JSON.stringify(point.labels ?? {})}`;
    const previous = latestBySeries.get(key);
    if (!previous || previous.ts < point.ts) latestBySeries.set(key, point);
  }
  const points = Array.from(latestBySeries.values());
  const measurements = props.presentationSchema?.measurements ?? [];
  const groups = groupsFor(
    points,
    props.presentationSchema?.groups ?? [],
    measurements,
  );
  if (metricsQuery.isPending || checksQuery.isPending) {
    return (
      <section className={styles.panel} aria-busy="true">
        <Typography c="dimmed">Loading telemetry…</Typography>
      </section>
    );
  }
  if (metricsQuery.isError || checksQuery.isError) {
    return (
      <section className={styles.panel} role="alert">
        <Typography>Telemetry is temporarily unavailable.</Typography>
      </section>
    );
  }
  if (points.length === 0 && (checksQuery.data?.length ?? 0) === 0) return null;
  return (
    <section className={styles.panel}>
      <Typography variant="title" order={3}>
        {props.title ?? "Telemetry"}
      </Typography>
      {groups.map(({ group, points: groupPoints }) => (
        <section className={styles.group} key={group.id}>
          <Typography variant="title" order={4}>
            {group.title}
          </Typography>
          <dl className={styles.grid}>
            {groupPoints.map((point) => {
              const descriptor = descriptorFor(point.name, measurements);
              return (
                <div
                  className={styles.metric}
                  key={`${point.name}:${JSON.stringify(point.labels ?? {})}`}
                >
                  <dt>{descriptor?.title ?? point.name}</dt>
                  <dd>{displayValue(point, descriptor)}</dd>
                  {Object.keys(point.labels ?? {}).length > 0 ? (
                    <div className={styles.labels}>
                      {Object.entries(point.labels ?? {})
                        .map(([key, value]) => `${key}: ${value}`)
                        .join(" · ")}
                    </div>
                  ) : null}
                </div>
              );
            })}
          </dl>
        </section>
      ))}
      <div className={styles.checks}>
        {(checksQuery.data ?? []).map((check: CheckStatus) => (
          <span className={styles.check} key={`${check.name}:${check.ts}`}>
            {check.name}: {check.status}
          </span>
        ))}
      </div>
    </section>
  );
};
