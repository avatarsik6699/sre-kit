import { Stack } from "~/shared/ui";
import {
  StatusPulse,
  type StatusPulseStatus,
} from "~/shared/components/status-pulse";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import { useEventsQuery } from "~/entities/telemetry";
import type { EventFeedTypes } from "./event-feed.types";

function toPulseStatus(level: string): StatusPulseStatus {
  if (level === "error" || level === "critical") {
    return "critical";
  }
  if (level === "warn" || level === "warning") {
    return "warn";
  }
  return "ok";
}

/** Dense timestamped event feed for one source, using the same pulse motif per line
 * (docs/SPEC.md §5.3). Reads the events query cache only — the page owns the live subscription. */
export const EventFeed: React.FC<EventFeedTypes.Props> = (props) => {
  const eventsQuery = useEventsQuery(props.sourceId);

  if (eventsQuery.data && eventsQuery.data.length === 0) {
    return <EmptyState title="No events yet" />;
  }

  return (
    <Stack gap={4}>
      {(eventsQuery.data ?? []).map((event, index) => (
        // Events have no stable ID from the API — ts+index is stable enough for a read-only feed.
        <Stack key={`${event.ts}-${index}`} gap={0}>
          <StatusPulse status={toPulseStatus(event.level)} label={event.ts} />
          <Typography mono>{event.message}</Typography>
        </Stack>
      ))}
    </Stack>
  );
};
