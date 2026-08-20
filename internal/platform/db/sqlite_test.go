package db_test

import (
	"path/filepath"
	"testing"

	"sre-kit/internal/platform/db"
)

func TestMigratePreservesRetiredProvisioningData(t *testing.T) {
	t.Parallel()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Migrate(sqlDB); err != nil {
		t.Fatalf("initial migrate: %v", err)
	}

	if _, err := sqlDB.Exec(`
		INSERT INTO sources (id, adapter_name, config_json, enabled, last_status, host_id)
		VALUES ('source-1', 'stub', '{}', 1, 'ok', 'host-1');
		INSERT INTO hosts (id, label, address, ssh_port, ssh_user, ssh_key_secret_ref)
		VALUES ('host-1', 'legacy', '192.0.2.1', 22, 'operator', 'secret-1');
		INSERT INTO provisioning_runs (id, host_id, preset_name, status, produced_source_id)
		VALUES ('run-1', 'host-1', 'beszel', 'done', 'source-1');
	`); err != nil {
		t.Fatalf("seed legacy provisioning data: %v", err)
	}

	if err := db.Migrate(sqlDB); err != nil {
		t.Fatalf("repeat migrate: %v", err)
	}

	var hostID string
	if err := sqlDB.QueryRow(`SELECT host_id FROM sources WHERE id = 'source-1'`).Scan(&hostID); err != nil {
		t.Fatalf("read preserved source host_id: %v", err)
	}
	if hostID != "host-1" {
		t.Fatalf("source host_id = %q, want host-1", hostID)
	}

	var hostCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM hosts WHERE id = 'host-1'`).Scan(&hostCount); err != nil {
		t.Fatalf("count preserved host: %v", err)
	}
	if hostCount != 1 {
		t.Fatalf("legacy host count = %d, want 1", hostCount)
	}

	var runStatus string
	if err := sqlDB.QueryRow(`SELECT status FROM provisioning_runs WHERE id = 'run-1'`).Scan(&runStatus); err != nil {
		t.Fatalf("read preserved provisioning run: %v", err)
	}
	if runStatus != "done" {
		t.Fatalf("legacy provisioning run status = %q, want done", runStatus)
	}
}
