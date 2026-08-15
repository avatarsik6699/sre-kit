// Installed-preset domain type + query hook over GET /api/presets (docs/SPEC.md §12.3) — drives
// the preset picker in widgets/deploy-drawer.
import { queryOptions, useQuery } from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import type { components } from "~/shared/api";

type PresetResponse =
  components["schemas"]["internal_provisioner_interfaces_http.presetResponse"];

export type Preset = {
  name: string;
  version: string;
  producesAdapter: string;
};

function toPreset(raw: PresetResponse): Preset {
  return {
    name: raw.name ?? "",
    version: raw.version ?? "",
    producesAdapter: raw.produces_adapter ?? "",
  };
}

export const presetsQueryOptions = queryOptions({
  queryKey: ["presets"] as const,
  queryFn: async () => {
    const result = await apiClient.GET("/api/presets");
    if (result.error) {
      throw normalizeApiFailure(result.error, result.response.status);
    }
    return (result.data ?? []).map(toPreset);
  },
});

export function usePresetsQuery() {
  return useQuery(presetsQueryOptions);
}
