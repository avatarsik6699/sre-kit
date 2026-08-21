package infrastructure

import (
	"context"
	"testing"
	"time"

	platformdb "sre-kit/internal/platform/db"
	"sre-kit/internal/telemetry/domain"
)

func TestCheckQueryReturnsLatestStatusPerSourceAndName(t *testing.T) {
	database, err := platformdb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := platformdb.Migrate(database); err != nil {
		t.Fatal(err)
	}
	repo := NewSQLiteRepository(database).Checks
	for index, status := range []string{"critical", "ok"} {
		if err := repo.Insert(context.Background(), domain.Check{
			SourceID: "source-1", Name: "availability", TS: time.Unix(int64(index), 0).UTC(),
			Status: status, MetaJSON: "{}", SchemaVersion: "1.0",
		}); err != nil {
			t.Fatal(err)
		}
	}
	checks, err := repo.Query(context.Background(), domain.CheckQuery{SourceID: "source-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Status != "ok" {
		t.Fatalf("checks = %+v", checks)
	}
}
