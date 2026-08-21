import { useState } from "react";
import {
  Button,
  Group,
  NumberInput,
  Select,
  Stack,
  TextInput,
} from "~/shared/ui";
import type { AlertRuleFormTypes } from "./alert-rule-form.types";

const CONDITIONS = [
  { value: ">", label: "> (metric greater than)" },
  { value: "<", label: "< (metric less than)" },
  { value: "=", label: "= (metric equals)" },
  { value: "status_is", label: "status is (check status)" },
];

/** Alert rule form: source + target metric/check name, condition, threshold, debounce, channel —
 * per docs/SPEC.md §5.1's Notifications page. */
export const AlertRuleForm: React.FC<AlertRuleFormTypes.Props> = (props) => {
  const [sourceId, setSourceId] = useState<string | null>(null);
  const [targetName, setTargetName] = useState("");
  const [condition, setCondition] = useState<string | null>(">");
  const [threshold, setThreshold] = useState("");
  const [debounceSeconds, setDebounceSeconds] = useState<number | string>(0);
  const [notifyChannelId, setNotifyChannelId] = useState<string | null>(null);

  const isValid =
    sourceId !== null &&
    targetName.trim() !== "" &&
    condition !== null &&
    threshold.trim() !== "";

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!isValid || sourceId === null || condition === null) {
      return;
    }
    props.onSubmit({
      sourceId,
      targetName,
      condition,
      threshold,
      debounceSeconds:
        typeof debounceSeconds === "number" ? debounceSeconds : 0,
      notifyChannelId: notifyChannelId ?? "",
    });
    setTargetName("");
    setThreshold("");
    setDebounceSeconds(0);
  }

  return (
    <form onSubmit={handleSubmit}>
      <Stack gap="sm">
        <Select
          label="Source"
          placeholder="Choose a source"
          required
          data={props.sources.map((source) => ({
            value: source.id,
            label: source.adapterId,
          }))}
          value={sourceId}
          onChange={setSourceId}
        />
        <TextInput
          label="Target name"
          description="Metric or check name, e.g. cpu.usage_percent or uptime.http"
          required
          value={targetName}
          onChange={(event) => setTargetName(event.currentTarget.value)}
        />
        <Select
          label="Condition"
          required
          data={CONDITIONS}
          value={condition}
          onChange={setCondition}
        />
        <TextInput
          label="Threshold"
          description={
            condition === "status_is"
              ? 'A check status, e.g. "critical"'
              : "A number, e.g. 90"
          }
          required
          value={threshold}
          onChange={(event) => setThreshold(event.currentTarget.value)}
        />
        <NumberInput
          label="Debounce (seconds)"
          description="Condition must hold this long before the alert fires"
          min={0}
          value={debounceSeconds}
          onChange={setDebounceSeconds}
        />
        <Select
          label="Notify via"
          placeholder="No notification"
          clearable
          data={props.channels.map((channel) => ({
            value: channel.id,
            label: `${channel.type} · ${channel.chatId}`,
          }))}
          value={notifyChannelId}
          onChange={setNotifyChannelId}
        />
        <Group justify="flex-end">
          <Button
            type="submit"
            disabled={!isValid}
            loading={props.isSubmitting}
          >
            Add rule
          </Button>
        </Group>
      </Stack>
    </form>
  );
};
