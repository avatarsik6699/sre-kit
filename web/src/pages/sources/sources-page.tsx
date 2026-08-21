import { useState } from "react";
import { ActionIcon, Button, Group, Switch, Table } from "~/shared/ui";
import { Link } from "@tanstack/react-router";
import { PageContainer } from "~/shared/components/page-container";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import {
  StatusPulse,
  type StatusPulseStatus,
} from "~/shared/components/status-pulse";
import {
  useDeleteSourceMutation,
  useSourcesQuery,
  useUpdateSourceMutation,
  type Source,
} from "~/entities/source";
import { AddSourceDrawer } from "~/widgets/add-source-drawer";

function toPulseStatus(lastStatus: string): StatusPulseStatus {
  if (lastStatus === "ok" || lastStatus === "unreachable") {
    return lastStatus;
  }
  return lastStatus === "error" ? "critical" : "unreachable";
}

/** Route-level composition for /sources (docs/SPEC.md §5.1). */
export const SourcesPage: React.FC = () => {
  const sourcesQuery = useSourcesQuery();
  const updateSourceMutation = useUpdateSourceMutation();
  const deleteSourceMutation = useDeleteSourceMutation();
  const [drawerOpened, setDrawerOpened] = useState(false);

  return (
    <PageContainer>
      <Group justify="space-between" mb="md">
        <Typography variant="title" order={2}>
          Sources
        </Typography>
        <Button onClick={() => setDrawerOpened(true)}>Add source</Button>
      </Group>

      {sourcesQuery.data && sourcesQuery.data.length === 0 ? (
        <EmptyState
          title="No sources yet"
          description="Add a source to start collecting metrics and checks."
          action={
            <Button onClick={() => setDrawerOpened(true)}>Add source</Button>
          }
        />
      ) : (
        <Table>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Status</Table.Th>
              <Table.Th>Adapter</Table.Th>
              <Table.Th>Enabled</Table.Th>
              <Table.Th>Last seen</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {(sourcesQuery.data ?? []).map((source: Source) => (
              <Table.Tr key={source.id}>
                <Table.Td>
                  <StatusPulse status={toPulseStatus(source.lastStatus)} />
                </Table.Td>
                <Table.Td>
                  <Link to="/sources/$id" params={{ id: source.id }}>
                    <Typography mono>{source.adapterId}</Typography>
                  </Link>
                </Table.Td>
                <Table.Td>
                  <Switch
                    checked={source.enabled}
                    onChange={(event) =>
                      updateSourceMutation.mutate({
                        id: source.id,
                        enabled: event.currentTarget.checked,
                      })
                    }
                  />
                </Table.Td>
                <Table.Td>
                  <Typography mono c="dimmed">
                    {source.lastSeenAt ?? "never"}
                  </Typography>
                </Table.Td>
                <Table.Td>
                  <ActionIcon
                    variant="subtle"
                    color="statusCritical"
                    aria-label="Remove source"
                    onClick={() => deleteSourceMutation.mutate(source.id)}
                  >
                    ✕
                  </ActionIcon>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      <AddSourceDrawer
        opened={drawerOpened}
        onClose={() => setDrawerOpened(false)}
      />
    </PageContainer>
  );
};
