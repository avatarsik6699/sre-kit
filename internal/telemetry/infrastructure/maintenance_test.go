package infrastructure

import (
	"context"
	"testing"
	"time"

	platformdb "sre-kit/internal/platform/db"
)

func TestMaintenanceRollsUpAndPrunesRawTelemetry(t *testing.T) {
	database, err := platformdb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := platformdb.Migrate(database); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	old := now.Add(-31 * 24 * time.Hour)
	for index, value := range []float64{10, 20} {
		if _, err := database.Exec(`INSERT INTO metrics(source_id,name,ts,value,labels_json,schema_version) VALUES('s','requests',?,?,'{}','1.0')`, old.Add(time.Duration(index)*time.Minute), value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(`INSERT INTO ingestion_batches(source_id,idempotency_key,received_at,record_count) VALUES('s','expired',?,1)`, old); err != nil {
		t.Fatal(err)
	}
	maintenance := NewMaintenance(database)
	maintenance.now = func() time.Time { return now }
	if err := maintenance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var raw, rollups, count, batches int
	var average float64
	if err := database.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*),avg_value,sample_count FROM metrics_rollup`).Scan(&rollups, &average, &count); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM ingestion_batches`).Scan(&batches); err != nil {
		t.Fatal(err)
	}
	if raw != 0 || rollups != 1 || average != 15 || count != 2 || batches != 0 {
		t.Fatalf("raw=%d rollups=%d avg=%v samples=%d batches=%d", raw, rollups, average, count, batches)
	}
}

func TestBatchRepositoryRejectsDuplicateKey(t *testing.T) {
	database, err := platformdb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := platformdb.Migrate(database); err != nil {
		t.Fatal(err)
	}
	repo := NewBatchRepository(database)
	first, err := repo.Reserve(context.Background(), "s", "batch-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Reserve(context.Background(), "s", "batch-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !first || second {
		t.Fatalf("first=%v second=%v", first, second)
	}
}
