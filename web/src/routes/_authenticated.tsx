// Pathless layout route (docs/SPEC.md §6: every endpoint except /api/auth/login and /healthz
// requires a session). No dedicated "whoami" endpoint exists, so beforeLoad reuses the sources
// query every child route needs anyway: ensureQueryData warms the cache for children AND doubles
// as the auth probe — a 401 ApiError redirects to /login instead of rendering.
import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";
import { ApiError } from "~/shared/api";
import { sourcesQueryOptions } from "~/entities/source";
import { AppShell } from "~/widgets/app-shell";

export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async ({ context }) => {
    try {
      await context.queryClient.ensureQueryData(sourcesQueryOptions);
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        throw redirect({ to: "/login" });
      }
      throw error;
    }
  },
  component: AuthenticatedLayout,
});

function AuthenticatedLayout() {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
