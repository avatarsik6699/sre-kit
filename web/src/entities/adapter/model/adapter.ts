// Installed-adapter domain type + query hook over GET /api/adapters (docs/SPEC.md §4) — drives
// the schema-driven add-source form (features/add-source-form).
import { queryOptions, useQuery } from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import type { components } from "~/shared/api";

type AdapterResponse =
  components["schemas"]["internal_adapterengine_interfaces_http.adapterResponse"];

export type AdapterManifest = {
  name: string;
  version: string;
  mode: string;
  emits: string[];
  // See entities/source/model/source.ts's comment: json.RawMessage fields are mistyped as
  // number[] by the generator; the wire value is really a JSON Schema object.
  configSchema: Record<string, unknown>;
  heartbeatSeconds?: number;
};

function toAdapterManifest(raw: AdapterResponse): AdapterManifest {
  return {
    name: raw.name ?? "",
    version: raw.version ?? "",
    mode: raw.mode ?? "pull",
    emits: raw.emits ?? [],
    configSchema:
      (raw.config_schema as unknown as Record<string, unknown>) ?? {},
    heartbeatSeconds: raw.heartbeat_seconds,
  };
}

export const adaptersQueryOptions = queryOptions({
  queryKey: ["adapters"] as const,
  queryFn: async () => {
    const result = await apiClient.GET("/api/adapters");
    if (result.error) {
      throw normalizeApiFailure(result.error, result.response.status);
    }
    return (result.data ?? []).map(toAdapterManifest);
  },
});

export function useAdaptersQuery() {
  return useQuery(adaptersQueryOptions);
}
