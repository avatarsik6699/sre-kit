import { useState } from "react";
import { ActionIcon, Badge, Button, Group, Table } from "@mantine/core";
import { PageContainer } from "~/shared/components/page-container";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import {
  StatusPulse,
  type StatusPulseStatus,
} from "~/shared/components/status-pulse";
import {
  useCheckHostConnectionMutation,
  useDeleteHostMutation,
  useHostsQuery,
} from "~/entities/host";
import { AddHostDrawer } from "~/widgets/add-host-drawer";
import { DeployDrawer } from "~/widgets/deploy-drawer";

function toPulseStatus(lastStatus: string): StatusPulseStatus {
  if (lastStatus === "ok" || lastStatus === "unreachable") {
    return lastStatus;
  }
  return lastStatus === "error" ? "critical" : "unreachable";
}

/** Route-level composition for /hosts (docs/SPEC.md §5.1/§12) — added post-M6. */
export const HostsPage: React.FC = () => {
  const hostsQuery = useHostsQuery();
  const checkConnectionMutation = useCheckHostConnectionMutation();
  const deleteHostMutation = useDeleteHostMutation();
  const [drawerOpened, setDrawerOpened] = useState(false);
  const [deployHostId, setDeployHostId] = useState<string | null>(null);

  return (
    <PageContainer>
      <Group justify="space-between" mb="md">
        <Typography variant="title" order={2}>
          Hosts
        </Typography>
        <Button onClick={() => setDrawerOpened(true)}>Add host</Button>
      </Group>

      {hostsQuery.data && hostsQuery.data.length === 0 ? (
        <EmptyState
          title="No hosts yet"
          description="Add an SSH-reachable host to deploy observability tools onto it (docs/SPEC.md §12)."
          action={
            <Button onClick={() => setDrawerOpened(true)}>Add host</Button>
          }
        />
      ) : (
        <Table>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Status</Table.Th>
              <Table.Th>Address</Table.Th>
              <Table.Th>Docker</Table.Th>
              <Table.Th>Last connected</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {(hostsQuery.data ?? []).map((host) => (
              <Table.Tr key={host.id}>
                <Table.Td>
                  <StatusPulse status={toPulseStatus(host.lastStatus)} />
                </Table.Td>
                <Table.Td>
                  <Typography mono>
                    {host.label || host.address} ({host.address})
                  </Typography>
                </Table.Td>
                <Table.Td>
                  <Badge
                    color={
                      host.dockerAvailable ? "statusOk" : "statusUnreachable"
                    }
                  >
                    {host.dockerAvailable ? "available" : "unavailable"}
                  </Badge>
                </Table.Td>
                <Table.Td>
                  <Typography mono c="dimmed">
                    {host.lastConnectedAt ?? "never"}
                  </Typography>
                </Table.Td>
                <Table.Td>
                  <Group gap="xs" justify="flex-end">
                    <Button
                      size="xs"
                      variant="light"
                      loading={
                        checkConnectionMutation.isPending &&
                        checkConnectionMutation.variables === host.id
                      }
                      onClick={() => checkConnectionMutation.mutate(host.id)}
                    >
                      Check connection
                    </Button>
                    <Button
                      size="xs"
                      disabled={!host.hostKeyFingerprint}
                      onClick={() => setDeployHostId(host.id)}
                    >
                      Deploy
                    </Button>
                    <ActionIcon
                      variant="subtle"
                      color="statusCritical"
                      aria-label="Remove host"
                      onClick={() => deleteHostMutation.mutate(host.id)}
                    >
                      ✕
                    </ActionIcon>
                  </Group>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      <AddHostDrawer
        opened={drawerOpened}
        onClose={() => setDrawerOpened(false)}
      />
      <DeployDrawer
        hostId={deployHostId}
        onClose={() => setDeployHostId(null)}
      />
    </PageContainer>
  );
};
