// ProvisioningRun domain type + query/mutation hooks over /api/hosts/{id}/provision and
// /api/provisioning-runs (docs/SPEC.md §4/§12.2). v1's workflow runs synchronously within the
// triggering request (see internal/provisioner/application/workflow.go's own doc comment) — there
// is no separate "poll until done" step; the provision/retry mutations resolve once the run has
// reached a terminal state (done or failed).
import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import type { components } from "~/shared/api";

type RunResponse =
  components["schemas"]["internal_provisioner_interfaces_http.runResponse"];

export type ProvisioningRun = {
  id: string;
  hostId: string;
  presetName: string;
  status: string;
  step: string;
  errorMessage: string;
  producedSourceId: string;
  startedAt: string;
  finishedAt: string | null;
};

function toRun(raw: RunResponse): ProvisioningRun {
  return {
    id: raw.id ?? "",
    hostId: raw.host_id ?? "",
    presetName: raw.preset_name ?? "",
    status: raw.status ?? "pending",
    step: raw.step ?? "",
    errorMessage: raw.error_message ?? "",
    producedSourceId: raw.produced_source_id ?? "",
    startedAt: raw.started_at ?? "",
    finishedAt: raw.finished_at ?? null,
  };
}

export function provisioningRunsQueryOptions(hostId: string) {
  return queryOptions({
    queryKey: ["provisioning-runs", hostId] as const,
    queryFn: async () => {
      const result = await apiClient.GET("/api/provisioning-runs", {
        params: { query: { host: hostId } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return (result.data ?? []).map(toRun);
    },
    enabled: hostId.length > 0,
  });
}

export function useProvisioningRunsQuery(hostId: string) {
  return useQuery(provisioningRunsQueryOptions(hostId));
}

export function useProvisionMutation(hostId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (presetName: string) => {
      const result = await apiClient.POST("/api/hosts/{id}/provision", {
        params: { path: { id: hostId } },
        body: { preset_name: presetName },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toRun(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["provisioning-runs", hostId],
      });
      void queryClient.invalidateQueries({ queryKey: ["sources"] });
    },
  });
}

export function useRetryRunMutation(hostId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (runId: string) => {
      const result = await apiClient.POST("/api/provisioning-runs/{id}/retry", {
        params: { path: { id: runId } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toRun(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["provisioning-runs", hostId],
      });
      void queryClient.invalidateQueries({ queryKey: ["sources"] });
    },
  });
}
