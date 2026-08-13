// Router configuration (docs/changes/01-core-skeleton.md I10): wires TanStack Router's
// pending/error/not-found defaults to shared/components/route-state (I8), and layers TanStack
// Query over the router per docs/SPEC.md §5.2's WS-push cache model (query-client.ts, I6).
import { QueryClientProvider } from "@tanstack/react-query";
import { createRouter as createTanStackRouter } from "@tanstack/react-router";
import { setupRouterSsrQueryIntegration } from "@tanstack/react-router-ssr-query";
import {
  RouteError,
  RouteNotFound,
  RoutePending,
} from "~/shared/components/route-state";
import { createQueryClient } from "~/shared/lib/query-client";
import { routeTree } from "./routeTree.gen";

export function getRouter() {
  const queryClient = createQueryClient();

  const router = createTanStackRouter({
    routeTree,
    context: { queryClient },
    defaultPreload: "intent",
    scrollRestoration: true,
    defaultPendingComponent: RoutePending,
    defaultErrorComponent: RouteError,
    defaultNotFoundComponent: RouteNotFound,
    Wrap: (props: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>
        {props.children}
      </QueryClientProvider>
    ),
  });

  setupRouterSsrQueryIntegration({ router, queryClient });

  return router;
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof getRouter>;
  }
}
