package db_test

import (
	"path/filepath"
	"testing"

	"sre-kit/internal/platform/db"
)

func TestMigrateCreatesOnlyCurrentBaseline(t *testing.T) {
	t.Parallel()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "baseline.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Migrate(sqlDB); err != nil {
		t.Fatalf("initial migrate: %v", err)
	}

	if err := db.Migrate(sqlDB); err != nil {
		t.Fatalf("repeat migrate: %v", err)
	}

	for _, retiredTable := range []string{"hosts", "provisioning_runs"} {
		var count int
		if err := sqlDB.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			retiredTable,
		).Scan(&count); err != nil {
			t.Fatalf("inspect retired table %q: %v", retiredTable, err)
		}
		if count != 0 {
			t.Fatalf("retired table %q exists in fresh baseline", retiredTable)
		}
	}

	var migrationCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("applied migration count = %d, want 1", migrationCount)
	}
}
