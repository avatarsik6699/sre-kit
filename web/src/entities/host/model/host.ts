// Host domain type + query/mutation hooks over /api/hosts (docs/SPEC.md §12).
import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import type { components } from "~/shared/api";

type HostResponse =
  components["schemas"]["internal_hosts_interfaces_http.hostResponse"];

export type Host = {
  id: string;
  label: string;
  address: string;
  sshPort: number;
  sshUser: string;
  hostKeyFingerprint: string;
  dockerAvailable: boolean;
  lastConnectedAt: string | null;
  lastStatus: string;
};

function toHost(raw: HostResponse): Host {
  return {
    id: raw.id ?? "",
    label: raw.label ?? "",
    address: raw.address ?? "",
    sshPort: raw.ssh_port ?? 22,
    sshUser: raw.ssh_user ?? "",
    hostKeyFingerprint: raw.host_key_fingerprint ?? "",
    dockerAvailable: raw.docker_available ?? false,
    lastConnectedAt: raw.last_connected_at ?? null,
    lastStatus: raw.last_status ?? "unreachable",
  };
}

export const hostsQueryOptions = queryOptions({
  queryKey: ["hosts"] as const,
  queryFn: async () => {
    const result = await apiClient.GET("/api/hosts");
    if (result.error) {
      throw normalizeApiFailure(result.error, result.response.status);
    }
    return (result.data ?? []).map(toHost);
  },
});

export function useHostsQuery() {
  return useQuery(hostsQueryOptions);
}

export function useCreateHostMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      label: string;
      address: string;
      sshPort: number;
      sshUser: string;
      sshKey: string;
    }) => {
      const result = await apiClient.POST("/api/hosts", {
        body: {
          label: input.label,
          address: input.address,
          ssh_port: input.sshPort,
          ssh_user: input.sshUser,
          ssh_key: input.sshKey,
        },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toHost(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["hosts"] });
    },
  });
}

export function useCheckHostConnectionMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const result = await apiClient.POST("/api/hosts/{id}/check-connection", {
        params: { path: { id } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toHost(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["hosts"] });
    },
  });
}

export function useDeleteHostMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const result = await apiClient.DELETE("/api/hosts/{id}", {
        params: { path: { id } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["hosts"] });
    },
  });
}
