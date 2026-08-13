import { createFileRoute } from "@tanstack/react-router";
import { SourcesPage } from "~/pages/sources";

export const Route = createFileRoute("/_authenticated/sources/")({
  component: SourcesPage,
});
