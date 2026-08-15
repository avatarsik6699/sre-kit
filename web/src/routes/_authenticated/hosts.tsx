import { createFileRoute } from "@tanstack/react-router";
import { HostsPage } from "~/pages/hosts";

export const Route = createFileRoute("/_authenticated/hosts")({
  component: HostsPage,
});
