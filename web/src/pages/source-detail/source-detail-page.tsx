import { Stack } from "@mantine/core";
import { PageContainer } from "~/shared/components/page-container";
import { Typography } from "~/shared/components/typography";
import { useLiveTelemetry } from "~/entities/telemetry";
import { useSourceQuery } from "~/entities/source";
import { LiveChart } from "~/widgets/live-chart";
import { EventFeed } from "~/widgets/event-feed";
import type { SourceDetailPageTypes } from "./source-detail-page.types";

/** Route-level composition for /sources/:id — owns the single live-telemetry subscription this
 * page's widgets (LiveChart, EventFeed) both read from (docs/SPEC.md §5.1/§5.2). */
export const SourceDetailPage: React.FC<SourceDetailPageTypes.Props> = (
  props,
) => {
  useLiveTelemetry(props.sourceId);
  const sourceQuery = useSourceQuery(props.sourceId);

  return (
    <PageContainer>
      <Stack gap="lg">
        <Typography variant="title" order={2} mono>
          {sourceQuery.data?.adapterId ?? props.sourceId}
        </Typography>
        <LiveChart sourceId={props.sourceId} />
        <Typography variant="title" order={3}>
          Events
        </Typography>
        <EventFeed sourceId={props.sourceId} />
      </Stack>
    </PageContainer>
  );
};
