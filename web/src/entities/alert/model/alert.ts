// Alert + AlertRule domain types and query/mutation hooks over /api/alerts, /api/alert-rules
// (docs/SPEC.md §3/§4), plus useLiveAlerts which layers the WS stream's "alert" frames on top per
// §5.2 — same pattern as entities/telemetry's useLiveTelemetry.
import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import type { components } from "~/shared/api";
import { useStreamSubscription } from "~/shared/lib/use-stream-subscription";
import type { StreamFrame } from "~/shared/lib/ws-stream-store";

type AlertResponse =
  components["schemas"]["internal_alertrouter_interfaces_http.alertResponse"];
type AlertRuleResponse =
  components["schemas"]["internal_alertrouter_interfaces_http.alertRuleResponse"];

export type Alert = {
  id: string;
  sourceId: string;
  ruleId: string | null;
  severity: string;
  message: string;
  createdAt: string;
  resolvedAt: string | null;
};

export type AlertRule = {
  id: string;
  sourceId: string;
  targetName: string;
  condition: string;
  threshold: string;
  debounceSeconds: number;
  notifyChannelId: string;
  enabled: boolean;
};

function toAlert(raw: AlertResponse): Alert {
  return {
    id: raw.id ?? "",
    sourceId: raw.source_id ?? "",
    ruleId: raw.rule_id ?? null,
    severity: raw.severity ?? "warning",
    message: raw.message ?? "",
    createdAt: raw.created_at ?? "",
    resolvedAt: raw.resolved_at ?? null,
  };
}

function toAlertRule(raw: AlertRuleResponse): AlertRule {
  return {
    id: raw.id ?? "",
    sourceId: raw.source_id ?? "",
    targetName: raw.target_name ?? "",
    condition: raw.condition ?? ">",
    threshold: raw.threshold ?? "",
    debounceSeconds: raw.debounce_seconds ?? 0,
    notifyChannelId: raw.notify_channel_id ?? "",
    enabled: raw.enabled ?? false,
  };
}

// --- Alerts (read-only) ---

export function alertsQueryKey(status: "active" | "resolved" | "" = "active") {
  return ["alerts", status] as const;
}

export function alertsQueryOptions(
  status: "active" | "resolved" | "" = "active",
) {
  return queryOptions({
    queryKey: alertsQueryKey(status),
    queryFn: async () => {
      const result = await apiClient.GET("/api/alerts", {
        params: { query: status ? { status } : {} },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return (result.data ?? []).map(toAlert);
    },
  });
}

export function useAlertsQuery(status: "active" | "resolved" | "" = "active") {
  return useQuery(alertsQueryOptions(status));
}

/**
 * Subscribes sourceId's live stream for "alert" frames and keeps the active-alerts cache in sync.
 * Call alongside useLiveTelemetry wherever a source is already rendered live (StatusTile owns
 * this per docs/SPEC.md §5.3's "same pulse motif everywhere" signature) — the Recent Alerts rail
 * then reads the same cache passively, no separate all-sources subscription needed.
 */
export function useLiveAlerts(sourceId: string): void {
  const queryClient = useQueryClient();
  useStreamSubscription(
    sourceId,
    (frame: StreamFrame) => {
      if (frame.type !== "alert") {
        return;
      }
      const incoming = toAlert(frame.payload);
      queryClient.setQueryData(
        alertsQueryKey("active"),
        (prev: Alert[] | undefined) => {
          const withoutStale = (prev ?? []).filter(
            (alert) => alert.id !== incoming.id,
          );
          return incoming.resolvedAt
            ? withoutStale
            : [...withoutStale, incoming];
        },
      );
    },
    () => {
      void queryClient.invalidateQueries({
        queryKey: alertsQueryKey("active"),
      });
    },
  );
}

// --- Alert rules (CRUD) ---

export function alertRulesQueryKey(sourceId = "") {
  return ["alert-rules", sourceId] as const;
}

export function useAlertRulesQuery(sourceId = "") {
  return useQuery({
    queryKey: alertRulesQueryKey(sourceId),
    queryFn: async () => {
      const result = await apiClient.GET("/api/alert-rules", {
        params: { query: sourceId ? { source: sourceId } : {} },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return (result.data ?? []).map(toAlertRule);
    },
  });
}

export type AlertRuleInput = {
  sourceId: string;
  targetName: string;
  condition: string;
  threshold: string;
  debounceSeconds: number;
  notifyChannelId: string;
};

export function useCreateAlertRuleMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: AlertRuleInput) => {
      const result = await apiClient.POST("/api/alert-rules", {
        body: {
          source_id: input.sourceId,
          target_name: input.targetName,
          condition: input.condition,
          threshold: input.threshold,
          debounce_seconds: input.debounceSeconds,
          notify_channel_id: input.notifyChannelId,
        },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toAlertRule(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
  });
}

export function useUpdateAlertRuleMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; enabled: boolean }) => {
      const result = await apiClient.PATCH("/api/alert-rules/{id}", {
        params: { path: { id: input.id } },
        body: { enabled: input.enabled },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toAlertRule(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
  });
}

export function useDeleteAlertRuleMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const result = await apiClient.DELETE("/api/alert-rules/{id}", {
        params: { path: { id } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
  });
}
