package application_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"sre-kit/internal/adapterengine/application"
	"sre-kit/internal/adapterengine/infrastructure"
	platformdb "sre-kit/internal/platform/db"
	telemetryapp "sre-kit/internal/telemetry/application"
	"sre-kit/internal/telemetry/domain"
	telemetryinfra "sre-kit/internal/telemetry/infrastructure"
)

// TestStubAdapter_EndToEndPullPipeline exercises the real pull-mode path adapters/stub exists for
// (docs/changes/01-core-skeleton.md B11): build the stub binary, spawn it through the real
// ProcessSpawner, run it through Runner (real contract.schema.json validation), and confirm its
// fixture metrics land in a real SQLite telemetry store via /api/metrics' backing Query.
func TestStubAdapter_EndToEndPullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end pipeline test (builds a binary, uses real SQLite) in short mode")
	}

	stubSourceDir := filepath.Join("..", "..", "..", "adapters", "stub")
	stubBinary := filepath.Join(t.TempDir(), "stub")
	build := exec.Command("go", "build", "-o", stubBinary, ".")
	build.Dir = stubSourceDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build stub adapter: %v\n%s", err, out)
	}

	sqlDB, err := platformdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()
	if err := platformdb.Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	repos := telemetryinfra.NewSQLiteRepository(sqlDB)
	telemetryService := telemetryapp.NewService(repos.Metrics, repos.Checks, repos.Events)

	runner := application.NewRunner(infrastructure.NewProcessSpawner(), telemetryService, nil)
	result, err := runner.RunOnce(context.Background(), "src-stub", stubBinary, nil, []byte("{}"))
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.LinesProcessed != 3 || result.InvalidLines != 0 {
		t.Fatalf("result = %+v, want 3 processed lines and 0 invalid", result)
	}

	metrics, err := telemetryService.QueryMetrics(context.Background(), domain.MetricQuery{SourceID: "src-stub"})
	if err != nil {
		t.Fatalf("QueryMetrics: %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("got %d metrics in storage, want 3", len(metrics))
	}
}
