import { useState } from "react";
import {
  Alert,
  Button,
  Group,
  PasswordInput,
  Stack,
  TextInput,
} from "@mantine/core";
import { Typography } from "~/shared/components/typography";
import type { NotificationChannelFormTypes } from "./notification-channel-form.types";

export const notificationChannelFormUtils = {
  /**
   * Client-side "test-send": validates the entered chat_id/bot_token look plausible before
   * saving. No backend probe endpoint exists (only CRUD on /api/notification-channels, per
   * docs/SPEC.md §4) — same scope decision as change 04's add-source-form test-connection step
   * (see docs/changes/04-minimal-ui.md § Implementation Notes).
   */
  validate(chatId: string, botToken: string): string[] {
    const errors: string[] = [];
    if (chatId.trim() === "") {
      errors.push("chat_id is required");
    }
    if (botToken.trim() === "") {
      errors.push("bot_token is required");
    } else if (!botToken.includes(":")) {
      errors.push(
        'bot_token doesn\'t look like a Telegram bot token (missing ":")',
      );
    }
    return errors;
  },
};

/** Telegram channel form: bot token + chat id, per docs/SPEC.md §5.1's Notifications page. */
export const NotificationChannelForm: React.FC<
  NotificationChannelFormTypes.Props
> = (props) => {
  const [chatId, setChatId] = useState("");
  const [botToken, setBotToken] = useState("");
  const [testResult, setTestResult] = useState<string[] | null>(null);

  function handleTestSend() {
    setTestResult(notificationChannelFormUtils.validate(chatId, botToken));
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    const errors = notificationChannelFormUtils.validate(chatId, botToken);
    if (errors.length > 0) {
      setTestResult(errors);
      return;
    }
    props.onSubmit({ chatId, botToken });
    setChatId("");
    setBotToken("");
    setTestResult(null);
  }

  return (
    <form onSubmit={handleSubmit}>
      <Stack gap="sm">
        <TextInput
          label="Chat ID"
          description="Telegram chat/channel ID to send alerts to"
          required
          value={chatId}
          onChange={(event) => {
            setChatId(event.currentTarget.value);
            setTestResult(null);
          }}
        />
        <PasswordInput
          label="Bot token"
          description="From @BotFather — stored encrypted, never shown again"
          required
          value={botToken}
          onChange={(event) => {
            setBotToken(event.currentTarget.value);
            setTestResult(null);
          }}
        />
        {testResult ? (
          testResult.length === 0 ? (
            <Alert color="statusOk">Configuration looks valid.</Alert>
          ) : (
            <Alert color="statusCritical">
              {testResult.map((error) => (
                <Typography key={error}>{error}</Typography>
              ))}
            </Alert>
          )
        ) : null}
        <Group justify="flex-end">
          <Button variant="light" onClick={handleTestSend}>
            Test
          </Button>
          <Button type="submit" loading={props.isSubmitting}>
            Add channel
          </Button>
        </Group>
      </Stack>
    </form>
  );
};
