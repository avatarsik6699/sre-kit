package infrastructure_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	platformdb "sre-kit/internal/platform/db"
	"sre-kit/internal/sources/domain"
	"sre-kit/internal/sources/infrastructure"
)

func TestSQLiteRepository_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-SQLite test in short mode")
	}

	sqlDB, err := platformdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()
	if err := platformdb.Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	repo := infrastructure.NewSQLiteRepository(sqlDB)
	ctx := context.Background()

	source := domain.Source{
		ID:          "src-1",
		AdapterName: "host-metrics-ssh",
		ConfigJSON:  `{"host":"1.2.3.4"}`,
		Enabled:     true,
		LastStatus:  domain.StatusUnreachable,
	}
	if err := repo.Create(ctx, source); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AdapterName != source.AdapterName {
		t.Fatalf("Get: AdapterName = %q, want %q", got.AdapterName, source.AdapterName)
	}

	got.LastStatus = domain.StatusOK
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, err := repo.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("Get after Update: %v", err)
	}
	if updated.LastStatus != domain.StatusOK {
		t.Fatalf("Get after Update: LastStatus = %q, want %q", updated.LastStatus, domain.StatusOK)
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List: got %d sources, want 1", len(all))
	}

	if err := repo.Delete(ctx, source.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, source.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want domain.ErrNotFound", err)
	}
}

func TestSQLiteRepository_GetMissingReturnsErrNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-SQLite test in short mode")
	}

	sqlDB, err := platformdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()
	if err := platformdb.Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	repo := infrastructure.NewSQLiteRepository(sqlDB)
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get missing: got %v, want domain.ErrNotFound", err)
	}
}
