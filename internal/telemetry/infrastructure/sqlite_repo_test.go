package infrastructure_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	platformdb "sre-kit/internal/platform/db"
	"sre-kit/internal/telemetry/domain"
	"sre-kit/internal/telemetry/infrastructure"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real-SQLite test in short mode")
	}
	sqlDB, err := platformdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := platformdb.Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return sqlDB
}

func TestSQLiteRepository_MetricInsertAndQuery(t *testing.T) {
	repos := infrastructure.NewSQLiteRepository(openTestDB(t))
	ctx := context.Background()

	ts := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	metric := domain.Metric{SourceID: "src-1", Name: "cpu.usage_percent", TS: ts, Value: 42.5, LabelsJSON: "{}", SchemaVersion: "1.0"}
	if err := repos.Metrics.Insert(ctx, metric); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := repos.Metrics.Query(ctx, domain.MetricQuery{SourceID: "src-1", Name: "cpu.usage_percent"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].Value != 42.5 {
		t.Fatalf("Query = %+v, want one metric with value 42.5", got)
	}
}

func TestSQLiteRepository_CheckInsertAndQuery(t *testing.T) {
	repos := infrastructure.NewSQLiteRepository(openTestDB(t))
	ctx := context.Background()

	check := domain.Check{SourceID: "src-1", Name: "tls-expiry", TS: time.Now(), Status: "ok", MetaJSON: "{}", SchemaVersion: "1.0"}
	if err := repos.Checks.Insert(ctx, check); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := repos.Checks.Query(ctx, domain.CheckQuery{SourceID: "src-1"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].Status != "ok" {
		t.Fatalf("Query = %+v, want one check with status ok", got)
	}
}

func TestSQLiteRepository_EventInsertAndQueryRespectsLimit(t *testing.T) {
	repos := infrastructure.NewSQLiteRepository(openTestDB(t))
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		event := domain.Event{SourceID: "src-1", TS: time.Now(), Level: "info", Message: "tick", LabelsJSON: "{}", SchemaVersion: "1.0"}
		if err := repos.Events.Insert(ctx, event); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	got, err := repos.Events.Query(ctx, domain.EventQuery{SourceID: "src-1", Limit: 2})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Query with Limit=2 returned %d events, want 2", len(got))
	}
}
