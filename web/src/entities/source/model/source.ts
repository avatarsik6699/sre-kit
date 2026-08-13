// Source domain type + query/mutation hooks over /api/sources (docs/SPEC.md §3/§4).
import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import type { components } from "~/shared/api";

type SourceResponse =
  components["schemas"]["internal_sources_interfaces_http.sourceResponse"];

export type Source = {
  id: string;
  adapterId: string;
  config: string;
  enabled: boolean;
  lastStatus: string;
  lastSeenAt: string | null;
};

function toSource(raw: SourceResponse): Source {
  return {
    id: raw.id ?? "",
    adapterId: raw.adapter_id ?? "",
    config: raw.config ?? "{}",
    enabled: raw.enabled ?? false,
    lastStatus: raw.last_status ?? "unreachable",
    lastSeenAt: raw.last_seen_at ?? null,
  };
}

export const sourcesQueryOptions = queryOptions({
  queryKey: ["sources"] as const,
  queryFn: async () => {
    const result = await apiClient.GET("/api/sources");
    if (result.error) {
      throw normalizeApiFailure(result.error, result.response.status);
    }
    return (result.data ?? []).map(toSource);
  },
});

export function useSourcesQuery() {
  return useQuery(sourcesQueryOptions);
}

export function useSourceQuery(sourceId: string) {
  return useQuery({
    ...sourcesQueryOptions,
    select: (sources) => sources.find((source) => source.id === sourceId),
  });
}

export function useCreateSourceMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { adapterId: string; config: unknown }) => {
      // swag/openapi-typescript mistypes json.RawMessage request/response fields as number[]
      // (they're actually arbitrary JSON objects on the wire) — cast through unknown at the
      // boundary rather than trusting the generated type here.
      const result = await apiClient.POST("/api/sources", {
        body: {
          adapter_id: input.adapterId,
          config: input.config,
        } as unknown as { adapter_id?: string; config?: number[] },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toSource(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["sources"] });
    },
  });
}

export function useUpdateSourceMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; enabled: boolean }) => {
      const result = await apiClient.PATCH("/api/sources/{id}", {
        params: { path: { id: input.id } },
        body: { enabled: input.enabled },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toSource(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["sources"] });
    },
  });
}

export function useDeleteSourceMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const result = await apiClient.DELETE("/api/sources/{id}", {
        params: { path: { id } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["sources"] });
    },
  });
}
