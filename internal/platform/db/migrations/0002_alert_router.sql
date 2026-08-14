-- 0002_alert_router.sql — M5 (internal/alertrouter), per docs/SPEC.md §3.
--
-- alert_rules.notify_channel (0001, unused placeholder) becomes a real FK-shaped reference to the
-- new notification_channels table, renamed to notify_channel_id for clarity at the point it
-- actually gets read/written.

ALTER TABLE alert_rules RENAME COLUMN notify_channel TO notify_channel_id;

CREATE TABLE IF NOT EXISTS notification_channels (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  config_json TEXT NOT NULL DEFAULT '{}',
  secret_ref TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT 1
);
