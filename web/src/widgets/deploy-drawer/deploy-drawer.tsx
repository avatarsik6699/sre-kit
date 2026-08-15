import { useState } from "react";
import {
  Alert,
  Button,
  Drawer,
  Group,
  Select,
  Stack,
  Table,
} from "@mantine/core";
import { usePresetsQuery } from "~/entities/preset";
import {
  useProvisionMutation,
  useProvisioningRunsQuery,
  useRetryRunMutation,
} from "~/entities/provisioning-run";
import { StatusPulse } from "~/shared/components/status-pulse";
import type { StatusPulseStatus } from "~/shared/components/status-pulse";
import { Typography } from "~/shared/components/typography";
import type { DeployDrawerTypes } from "./deploy-drawer.types";

function toPulseStatus(status: string): StatusPulseStatus {
  if (status === "done") return "ok";
  if (status === "failed") return "critical";
  return "unreachable";
}

/**
 * Preset picker + provisioning progress view (docs/SPEC.md §12.2), opened for one host at a time.
 * v1's workflow runs synchronously within the triggering request, so "Deploy" blocks (button shows
 * `loading`) until the run reaches done/failed rather than polling a background job — matches the
 * backend's documented v1 simplification (internal/provisioner/application/workflow.go).
 */
export const DeployDrawer: React.FC<DeployDrawerTypes.Props> = (props) => {
  const hostId = props.hostId ?? "";
  const presetsQuery = usePresetsQuery();
  const runsQuery = useProvisioningRunsQuery(hostId);
  const provisionMutation = useProvisionMutation(hostId);
  const retryMutation = useRetryRunMutation(hostId);
  const [selectedPreset, setSelectedPreset] = useState<string | null>(null);

  function handleClose() {
    setSelectedPreset(null);
    props.onClose();
  }

  return (
    <Drawer
      opened={props.hostId !== null}
      onClose={handleClose}
      title="Deploy"
      position="right"
    >
      <Stack gap="md">
        <Select
          label="Preset"
          placeholder="Choose a preset"
          data={(presetsQuery.data ?? []).map((preset) => ({
            value: preset.name,
            label: `${preset.name} (${preset.producesAdapter})`,
          }))}
          value={selectedPreset}
          onChange={setSelectedPreset}
        />
        <Group justify="flex-end">
          <Button
            disabled={!selectedPreset}
            loading={provisionMutation.isPending}
            onClick={() =>
              selectedPreset && provisionMutation.mutate(selectedPreset)
            }
          >
            Deploy
          </Button>
        </Group>

        {provisionMutation.data ? (
          provisionMutation.data.status === "done" ? (
            <Alert color="statusOk" title="Provisioned">
              Source registered: {provisionMutation.data.producedSourceId}
            </Alert>
          ) : (
            <Alert color="statusCritical" title="Provisioning failed">
              {provisionMutation.data.errorMessage}
            </Alert>
          )
        ) : null}

        <Typography variant="title" order={4}>
          Past runs
        </Typography>
        <Table>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Status</Table.Th>
              <Table.Th>Preset</Table.Th>
              <Table.Th>Step</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {(runsQuery.data ?? []).map((run) => (
              <Table.Tr key={run.id}>
                <Table.Td>
                  <StatusPulse status={toPulseStatus(run.status)} />
                </Table.Td>
                <Table.Td>
                  <Typography mono>{run.presetName}</Typography>
                </Table.Td>
                <Table.Td>
                  <Typography mono c="dimmed">
                    {run.step || "—"}
                  </Typography>
                </Table.Td>
                <Table.Td>
                  {run.status === "failed" ? (
                    <Button
                      size="xs"
                      variant="light"
                      loading={retryMutation.isPending}
                      onClick={() => retryMutation.mutate(run.id)}
                    >
                      Retry
                    </Button>
                  ) : null}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Stack>
    </Drawer>
  );
};
