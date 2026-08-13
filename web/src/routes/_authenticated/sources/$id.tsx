import { createFileRoute } from "@tanstack/react-router";
import { SourceDetailPage } from "~/pages/source-detail";

export const Route = createFileRoute("/_authenticated/sources/$id")({
  component: RouteComponent,
});

function RouteComponent() {
  const params = Route.useParams();
  return <SourceDetailPage sourceId={params.id} />;
}
