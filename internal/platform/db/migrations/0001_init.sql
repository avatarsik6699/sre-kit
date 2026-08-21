-- 0001_init.sql — initial schema, per docs/SPEC.md §3.
--
-- Change 22 intentionally defines a fresh development baseline; historical telemetry and retired
-- provisioning tables are not migrated.

CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO projects (id, name, slug, description)
VALUES ('default', 'Default', 'default', 'Sources not yet assigned to a named project');

CREATE TABLE IF NOT EXISTS sources (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL DEFAULT 'default' REFERENCES projects(id) ON DELETE RESTRICT,
  adapter_name TEXT NOT NULL,
  config_json TEXT NOT NULL DEFAULT '{}',
  enabled BOOLEAN NOT NULL DEFAULT 1,
  last_status TEXT NOT NULL DEFAULT 'unreachable',
  last_seen_at DATETIME
);

CREATE TABLE IF NOT EXISTS metrics (
  source_id TEXT NOT NULL,
  name TEXT NOT NULL,
  ts DATETIME NOT NULL,
  value REAL NOT NULL,
  labels_json TEXT NOT NULL DEFAULT '{}',
  schema_version TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_metrics_source_name_ts ON metrics (source_id, name, ts);
CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_identity
  ON metrics (source_id, name, ts, labels_json);

CREATE TABLE IF NOT EXISTS checks (
  source_id TEXT NOT NULL,
  name TEXT NOT NULL,
  ts DATETIME NOT NULL,
  status TEXT NOT NULL,
  meta_json TEXT NOT NULL DEFAULT '{}',
  schema_version TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_checks_source_name_ts ON checks (source_id, name, ts);
CREATE UNIQUE INDEX IF NOT EXISTS idx_checks_identity
  ON checks (source_id, name, ts);

CREATE TABLE IF NOT EXISTS events (
  source_id TEXT NOT NULL,
  ts DATETIME NOT NULL,
  level TEXT NOT NULL,
  message TEXT NOT NULL,
  labels_json TEXT NOT NULL DEFAULT '{}',
  schema_version TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_source_ts ON events (source_id, ts);
CREATE UNIQUE INDEX IF NOT EXISTS idx_events_identity
  ON events (source_id, ts, level, message, labels_json);

CREATE TABLE IF NOT EXISTS alerts (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  rule_id TEXT,
  severity TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  resolved_at DATETIME
);

CREATE TABLE IF NOT EXISTS alert_rules (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  target_name TEXT NOT NULL,
  condition TEXT NOT NULL,
  threshold TEXT NOT NULL,
  debounce_seconds INTEGER NOT NULL DEFAULT 0,
  notify_channel_id TEXT,
  enabled BOOLEAN NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS notification_channels (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  config_json TEXT NOT NULL DEFAULT '{}',
  secret_ref TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS metrics_rollup (
  source_id TEXT NOT NULL,
  name TEXT NOT NULL,
  labels_json TEXT NOT NULL DEFAULT '{}',
  bucket_ts DATETIME NOT NULL,
  min_value REAL NOT NULL,
  max_value REAL NOT NULL,
  avg_value REAL NOT NULL,
  sample_count INTEGER NOT NULL,
  PRIMARY KEY (source_id, name, labels_json, bucket_ts)
);

CREATE INDEX IF NOT EXISTS idx_metrics_rollup_source_name_bucket
  ON metrics_rollup (source_id, name, bucket_ts);

CREATE TABLE IF NOT EXISTS ingestion_batches (
  source_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  record_count INTEGER NOT NULL,
  PRIMARY KEY (source_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS maintenance_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  started_at DATETIME NOT NULL,
  finished_at DATETIME,
  raw_deleted INTEGER NOT NULL DEFAULT 0,
  rollups_deleted INTEGER NOT NULL DEFAULT 0,
  batches_deleted INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  error_message TEXT NOT NULL DEFAULT ''
);
