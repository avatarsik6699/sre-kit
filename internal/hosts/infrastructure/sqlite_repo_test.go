package infrastructure_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"sre-kit/internal/hosts/domain"
	"sre-kit/internal/hosts/infrastructure"
	platformdb "sre-kit/internal/platform/db"
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

	host := domain.Host{
		ID:              "host-1",
		Label:           "prod-vps",
		Address:         "1.2.3.4",
		SSHPort:         22,
		SSHUser:         "operator",
		SSHKeySecretRef: "ref-1",
		LastStatus:      domain.StatusUnreachable,
		CreatedAt:       time.Now().UTC().Truncate(time.Second),
	}
	if err := repo.Create(ctx, host); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, host.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Address != host.Address {
		t.Fatalf("Get: Address = %q, want %q", got.Address, host.Address)
	}

	got.HostKeyFingerprint = "SHA256:abc"
	got.DockerAvailable = true
	got.LastStatus = domain.StatusOK
	connectedAt := time.Now().UTC().Truncate(time.Second)
	got.LastConnectedAt = &connectedAt
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := repo.Get(ctx, host.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if updated.HostKeyFingerprint != "SHA256:abc" || !updated.DockerAvailable || updated.LastConnectedAt == nil {
		t.Fatalf("Get after update: got %+v", updated)
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List: len = %d, want 1", len(all))
	}

	if err := repo.Delete(ctx, host.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, host.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get after delete: err = %v, want ErrNotFound", err)
	}
}
