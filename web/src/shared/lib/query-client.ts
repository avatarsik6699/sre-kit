// QueryClient factory with retry disabled — matches docs/SPEC.md §5.2's WS-push cache model:
// TanStack Query is layered over the WS stream store for historical/paginated reads, and cache
// updates are driven by WS push, not refetch intervals or automatic retry.
import { QueryClient } from "@tanstack/react-query";

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        refetchOnWindowFocus: false,
      },
    },
  });
}
