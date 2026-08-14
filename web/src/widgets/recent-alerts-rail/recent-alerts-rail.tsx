import { Card, Stack } from "@mantine/core";
import {
  StatusPulse,
  type StatusPulseStatus,
} from "~/shared/components/status-pulse";
import { Typography } from "~/shared/components/typography";
import { useAlertsQuery } from "~/entities/alert";

function toPulseStatus(severity: string): StatusPulseStatus {
  return severity === "critical" ? "critical" : "warn";
}

/** Slim persistent panel of active alerts across every source, per docs/SPEC.md §5.1/§5.3.
 * Reads the same "alerts" cache each rendered StatusTile's useLiveAlerts keeps in sync — no
 * separate all-sources subscription needed. */
export const RecentAlertsRail: React.FC = () => {
  const alertsQuery = useAlertsQuery("active");
  const alerts = alertsQuery.data ?? [];

  if (alerts.length === 0) {
    return null;
  }

  return (
    <Card withBorder padding="sm">
      <Stack gap={4}>
        <Typography variant="title" order={4}>
          Active alerts
        </Typography>
        {alerts.map((alert) => (
          <Stack key={alert.id} gap={0}>
            <StatusPulse
              status={toPulseStatus(alert.severity)}
              label={alert.createdAt}
            />
            <Typography mono>{alert.message}</Typography>
          </Stack>
        ))}
      </Stack>
    </Card>
  );
};
