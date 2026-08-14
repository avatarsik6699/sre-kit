import { ActionIcon, Group, Stack, Switch, Table } from "@mantine/core";
import { PageContainer } from "~/shared/components/page-container";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import {
  useAlertRulesQuery,
  useCreateAlertRuleMutation,
  useDeleteAlertRuleMutation,
  useUpdateAlertRuleMutation,
} from "~/entities/alert";
import {
  useCreateNotificationChannelMutation,
  useDeleteNotificationChannelMutation,
  useNotificationChannelsQuery,
  useUpdateNotificationChannelMutation,
} from "~/entities/notification-channel";
import { useSourcesQuery } from "~/entities/source";
import { AlertRuleForm } from "~/widgets/alert-rule-form";
import { NotificationChannelForm } from "~/widgets/notification-channel-form";

/** Route-level composition for /notifications — configure Telegram channels and alert rules
 * (docs/SPEC.md §5.1). */
export const NotificationsPage: React.FC = () => {
  const sourcesQuery = useSourcesQuery();
  const channelsQuery = useNotificationChannelsQuery();
  const rulesQuery = useAlertRulesQuery();

  const createChannelMutation = useCreateNotificationChannelMutation();
  const updateChannelMutation = useUpdateNotificationChannelMutation();
  const deleteChannelMutation = useDeleteNotificationChannelMutation();

  const createRuleMutation = useCreateAlertRuleMutation();
  const updateRuleMutation = useUpdateAlertRuleMutation();
  const deleteRuleMutation = useDeleteAlertRuleMutation();

  const sources = sourcesQuery.data ?? [];
  const channels = channelsQuery.data ?? [];
  const rules = rulesQuery.data ?? [];

  function sourceLabel(sourceId: string): string {
    return (
      sources.find((source) => source.id === sourceId)?.adapterId ?? sourceId
    );
  }

  return (
    <PageContainer>
      <Stack gap="xl">
        <Typography variant="title" order={2}>
          Notifications
        </Typography>

        <Stack gap="sm">
          <Typography variant="title" order={3}>
            Channels
          </Typography>
          {channels.length === 0 ? (
            <EmptyState
              title="No channels yet"
              description="Add a Telegram channel to receive alert notifications."
            />
          ) : (
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Type</Table.Th>
                  <Table.Th>Chat ID</Table.Th>
                  <Table.Th>Enabled</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {channels.map((channel) => (
                  <Table.Tr key={channel.id}>
                    <Table.Td>
                      <Typography mono>{channel.type}</Typography>
                    </Table.Td>
                    <Table.Td>
                      <Typography mono>{channel.chatId}</Typography>
                    </Table.Td>
                    <Table.Td>
                      <Switch
                        checked={channel.enabled}
                        onChange={(event) =>
                          updateChannelMutation.mutate({
                            id: channel.id,
                            enabled: event.currentTarget.checked,
                          })
                        }
                      />
                    </Table.Td>
                    <Table.Td>
                      <ActionIcon
                        variant="subtle"
                        color="statusCritical"
                        aria-label="Remove channel"
                        onClick={() => deleteChannelMutation.mutate(channel.id)}
                      >
                        ✕
                      </ActionIcon>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          )}
          <NotificationChannelForm
            isSubmitting={createChannelMutation.isPending}
            onSubmit={(input) =>
              createChannelMutation.mutate({ type: "telegram", ...input })
            }
          />
        </Stack>

        <Stack gap="sm">
          <Typography variant="title" order={3}>
            Alert rules
          </Typography>
          {rules.length === 0 ? (
            <EmptyState
              title="No alert rules yet"
              description="Add a rule to get notified when a metric or check crosses a threshold."
            />
          ) : (
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Source</Table.Th>
                  <Table.Th>Target</Table.Th>
                  <Table.Th>Condition</Table.Th>
                  <Table.Th>Enabled</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {rules.map((rule) => (
                  <Table.Tr key={rule.id}>
                    <Table.Td>
                      <Typography mono>{sourceLabel(rule.sourceId)}</Typography>
                    </Table.Td>
                    <Table.Td>
                      <Typography mono>{rule.targetName}</Typography>
                    </Table.Td>
                    <Table.Td>
                      <Typography mono>
                        {rule.condition} {rule.threshold}
                      </Typography>
                    </Table.Td>
                    <Table.Td>
                      <Switch
                        checked={rule.enabled}
                        onChange={(event) =>
                          updateRuleMutation.mutate({
                            id: rule.id,
                            enabled: event.currentTarget.checked,
                          })
                        }
                      />
                    </Table.Td>
                    <Table.Td>
                      <ActionIcon
                        variant="subtle"
                        color="statusCritical"
                        aria-label="Remove rule"
                        onClick={() => deleteRuleMutation.mutate(rule.id)}
                      >
                        ✕
                      </ActionIcon>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          )}
          <Group>
            <AlertRuleForm
              sources={sources}
              channels={channels}
              isSubmitting={createRuleMutation.isPending}
              onSubmit={(input) => createRuleMutation.mutate(input)}
            />
          </Group>
        </Stack>
      </Stack>
    </PageContainer>
  );
};
