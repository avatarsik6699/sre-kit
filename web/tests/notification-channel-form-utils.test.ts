import { describe, expect, it } from "vitest";
import { notificationChannelFormUtils } from "~/widgets/notification-channel-form/notification-channel-form";

describe("notificationChannelFormUtils.validate", () => {
  it("requires chat_id and bot_token", () => {
    expect(notificationChannelFormUtils.validate("", "")).toEqual([
      "chat_id is required",
      "bot_token is required",
    ]);
  });

  it("flags a bot_token missing the id:secret shape", () => {
    expect(notificationChannelFormUtils.validate("123", "not-a-token")).toEqual(
      ['bot_token doesn\'t look like a Telegram bot token (missing ":")'],
    );
  });

  it("passes for a plausible chat_id/bot_token pair", () => {
    expect(
      notificationChannelFormUtils.validate("123", "111:AAAbbbCCC"),
    ).toEqual([]);
  });
});
