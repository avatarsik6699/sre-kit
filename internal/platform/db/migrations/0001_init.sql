-- 0001_init.sql — initial schema, per docs/SPEC.md §3.
--
-- alerts / alert_rules are created here for forward-compat only: no Go code in change 01 reads or
-- writes them (internal/alerts is deferred to M5, see docs/changes/01-core-skeleton.md § Do NOT
-- touch). metrics_rollup is reserved for a v2 downsampling feature and is unused in v1.

CREATE TABLE IF NOT EXISTS sources (
  id TEXT PRIMARY KEY,
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

CREATE TABLE IF NOT EXISTS checks (
  source_id TEXT NOT NULL,
  name TEXT NOT NULL,
  ts DATETIME NOT NULL,
  status TEXT NOT NULL,
  meta_json TEXT NOT NULL DEFAULT '{}',
  schema_version TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_checks_source_name_ts ON checks (source_id, name, ts);

CREATE TABLE IF NOT EXISTS events (
  source_id TEXT NOT NULL,
  ts DATETIME NOT NULL,
  level TEXT NOT NULL,
  message TEXT NOT NULL,
  labels_json TEXT NOT NULL DEFAULT '{}',
  schema_version TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_source_ts ON events (source_id, ts);

-- Reserved for M5 (internal/alerts) — not read/written by any Go code in this change.
CREATE TABLE IF NOT EXISTS alerts (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  rule_id TEXT,
  severity TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  resolved_at DATETIME
);

-- Reserved for M5 (internal/alerts) — not read/written by any Go code in this change.
CREATE TABLE IF NOT EXISTS alert_rules (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  target_name TEXT NOT NULL,
  condition TEXT NOT NULL,
  threshold TEXT NOT NULL,
  debounce_seconds INTEGER NOT NULL DEFAULT 0,
  notify_channel TEXT,
  enabled BOOLEAN NOT NULL DEFAULT 1
);

-- Reserved for a v2 downsampling feature — not read/written in v1.
CREATE TABLE IF NOT EXISTS metrics_rollup (
  source_id TEXT NOT NULL,
  name TEXT NOT NULL,
  bucket_ts DATETIME NOT NULL,
  agg_json TEXT NOT NULL DEFAULT '{}'
);
