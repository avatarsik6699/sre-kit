// Metric/Check/Event domain types + query hooks over /api/metrics, /api/checks, /api/events
// (docs/SPEC.md §3/§4), plus useLiveTelemetry which layers the WS stream on top per §5.2: cache
// entries are updated directly from pushed frames, and a reconnect triggers one REST re-fetch —
// no polling.
import {
  queryOptions,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import { useStreamSubscription } from "~/shared/lib/use-stream-subscription";
import type { StreamFrame } from "~/shared/lib/ws-stream-store";

export type MetricPoint = {
  name: string;
  ts: string;
  value: number;
  labels?: Record<string, string>;
};
export type CheckStatus = {
  name: string;
  ts: string;
  status: string;
  meta: Record<string, unknown>;
};
export type EventItem = {
  ts: string;
  level: string;
  message: string;
  labels: Record<string, string>;
};

// Loose shapes accepted by the converters below — both the generated REST response types (every
// field optional, per openapi-typescript) and the WS StreamXPayload types (fields guaranteed
// present) satisfy these structurally.
type MetricLike = {
  name?: string;
  ts?: string;
  value?: number;
  labels?: unknown;
};
type CheckLike = {
  name?: string;
  ts?: string;
  status?: string;
  meta?: unknown;
};
type EventLike = {
  ts?: string;
  level?: string;
  message?: string;
  labels?: unknown;
};

function labels(raw: unknown): Record<string, string> {
  if (raw && typeof raw === "object" && !Array.isArray(raw))
    return raw as Record<string, string>;
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw) as Record<string, string>;
    } catch {
      return {};
    }
  }
  return {};
}

function toMetricPoint(raw: MetricLike): MetricPoint {
  return {
    name: raw.name ?? "",
    ts: raw.ts ?? "",
    value: raw.value ?? 0,
    labels: labels(raw.labels),
  };
}

function toCheckStatus(raw: CheckLike): CheckStatus {
  return {
    name: raw.name ?? "",
    ts: raw.ts ?? "",
    status: raw.status ?? "unreachable",
    meta:
      raw.meta && typeof raw.meta === "object"
        ? (raw.meta as Record<string, unknown>)
        : {},
  };
}

function toEventItem(raw: EventLike): EventItem {
  return {
    ts: raw.ts ?? "",
    level: raw.level ?? "info",
    message: raw.message ?? "",
    labels: labels(raw.labels),
  };
}

export function metricsQueryKey(sourceId: string) {
  return ["metrics", sourceId] as const;
}

export function metricsQueryOptions(sourceId: string) {
  return queryOptions({
    queryKey: metricsQueryKey(sourceId),
    queryFn: async () => {
      const result = await apiClient.GET("/api/metrics", {
        params: { query: { source: sourceId } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return (result.data ?? []).map(toMetricPoint);
    },
  });
}

export function useMetricsQuery(sourceId: string) {
  return useQuery(metricsQueryOptions(sourceId));
}

export function checksQueryKey(sourceId: string) {
  return ["checks", sourceId] as const;
}

export function checksQueryOptions(sourceId: string) {
  return queryOptions({
    queryKey: checksQueryKey(sourceId),
    queryFn: async () => {
      const result = await apiClient.GET("/api/checks", {
        params: { query: { source: sourceId } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return (result.data ?? []).map(toCheckStatus);
    },
  });
}

export function useChecksQuery(sourceId: string) {
  return useQuery(checksQueryOptions(sourceId));
}

export function eventsQueryKey(sourceId: string) {
  return ["events", sourceId] as const;
}

export function eventsQueryOptions(sourceId: string, limit = 50) {
  return queryOptions({
    queryKey: eventsQueryKey(sourceId),
    queryFn: async () => {
      const result = await apiClient.GET("/api/events", {
        params: { query: { source: sourceId, limit } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return (result.data ?? []).map(toEventItem);
    },
  });
}

export function useEventsQuery(sourceId: string, limit = 50) {
  return useQuery(eventsQueryOptions(sourceId, limit));
}

const MAX_LIVE_METRIC_POINTS = 200;

function applyFrame(
  queryClient: QueryClient,
  sourceId: string,
  frame: StreamFrame,
): void {
  if (frame.type === "metric") {
    queryClient.setQueryData(
      metricsQueryKey(sourceId),
      (prev: MetricPoint[] | undefined) => {
        const next = [...(prev ?? []), toMetricPoint(frame.payload)];
        return next.length > MAX_LIVE_METRIC_POINTS
          ? next.slice(next.length - MAX_LIVE_METRIC_POINTS)
          : next;
      },
    );
    return;
  }
  if (frame.type === "check") {
    queryClient.setQueryData(
      checksQueryKey(sourceId),
      (prev: CheckStatus[] | undefined) => {
        const incoming = toCheckStatus(frame.payload);
        const withoutStale = (prev ?? []).filter(
          (check) => check.name !== incoming.name,
        );
        return [...withoutStale, incoming];
      },
    );
    return;
  }
  queryClient.setQueryData(
    eventsQueryKey(sourceId),
    (prev: EventItem[] | undefined) => [
      toEventItem(frame.payload),
      ...(prev ?? []),
    ],
  );
}

/** Subscribes sourceId to the live stream and keeps the metrics/checks/events caches in sync. */
export function useLiveTelemetry(sourceId: string): void {
  const queryClient = useQueryClient();
  useStreamSubscription(
    sourceId,
    (frame) => applyFrame(queryClient, sourceId, frame),
    () => {
      void queryClient.invalidateQueries({
        queryKey: metricsQueryKey(sourceId),
      });
      void queryClient.invalidateQueries({
        queryKey: checksQueryKey(sourceId),
      });
      void queryClient.invalidateQueries({
        queryKey: eventsQueryKey(sourceId),
      });
    },
  );
}
