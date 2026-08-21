import { queryOptions, useQuery } from "@tanstack/react-query";
import { apiClient } from "~/shared/api";

export type Project = {
  id: string;
  name: string;
  slug: string;
  description: string;
};
export const projectsQueryOptions = queryOptions({
  queryKey: ["projects"] as const,
  queryFn: async () => {
    const result = await apiClient.GET("/api/projects");
    return (result.data ?? []).map((raw) => ({
      id: raw.id ?? "",
      name: raw.name ?? "",
      slug: raw.slug ?? "",
      description: raw.description ?? "",
    }));
  },
});
export function useProjectsQuery() {
  return useQuery(projectsQueryOptions);
}
