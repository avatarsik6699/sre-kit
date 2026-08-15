-- 0003_hosts_provisioning.sql — Observability Auto-Provisioning, per docs/SPEC.md §12.
--
-- hosts/provisioning_runs are independent of the telemetry chain (sources/metrics/checks/events):
-- a Host is infrastructure internal/provisioner mutates, not part of the read-only monitoring
-- projection. sources.host_id is nullable and additive-only — NULL for every pre-existing source,
-- set only when a source is produced by the provisioner.

CREATE TABLE IF NOT EXISTS hosts (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL DEFAULT '',
  address TEXT NOT NULL,
  ssh_port INTEGER NOT NULL DEFAULT 22,
  ssh_user TEXT NOT NULL DEFAULT '',
  ssh_key_secret_ref TEXT NOT NULL DEFAULT '',
  host_key_fingerprint TEXT NOT NULL DEFAULT '',
  docker_available BOOLEAN NOT NULL DEFAULT 0,
  last_connected_at DATETIME,
  last_status TEXT NOT NULL DEFAULT 'unreachable',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE sources ADD COLUMN host_id TEXT;

CREATE TABLE IF NOT EXISTS provisioning_runs (
  id TEXT PRIMARY KEY,
  host_id TEXT NOT NULL,
  preset_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  step TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  admin_password_secret_ref TEXT NOT NULL DEFAULT '',
  produced_source_id TEXT,
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_provisioning_runs_host ON provisioning_runs (host_id);
