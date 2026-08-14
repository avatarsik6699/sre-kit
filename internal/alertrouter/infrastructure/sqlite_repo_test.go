package infrastructure_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"sre-kit/internal/alertrouter/domain"
	"sre-kit/internal/alertrouter/infrastructure"
	platformdb "sre-kit/internal/platform/db"
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

func TestSQLiteRepository_NotificationChannelCRUD(t *testing.T) {
	repos := infrastructure.NewSQLiteRepository(openTestDB(t))
	ctx := context.Background()

	channel := domain.NotificationChannel{ID: "chan-1", Type: "telegram", ConfigJSON: `{"chat_id":"123"}`, SecretRef: "secret-1", Enabled: true}
	if err := repos.Channels.Create(ctx, channel); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repos.Channels.Get(ctx, "chan-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != channel {
		t.Fatalf("got %+v, want %+v", got, channel)
	}

	channel.Enabled = false
	if err := repos.Channels.Update(ctx, channel); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := repos.Channels.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Enabled {
		t.Fatalf("List after update = %+v", list)
	}

	if err := repos.Channels.Delete(ctx, "chan-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repos.Channels.Get(ctx, "chan-1"); err != domain.ErrChannelNotFound {
		t.Fatalf("Get after delete: err = %v, want ErrChannelNotFound", err)
	}
}

func TestSQLiteRepository_AlertRuleCRUD(t *testing.T) {
	repos := infrastructure.NewSQLiteRepository(openTestDB(t))
	ctx := context.Background()

	rule := domain.AlertRule{
		ID: "rule-1", SourceID: "src-1", TargetName: "cpu.usage_percent",
		Condition: ">", Threshold: "90", DebounceSeconds: 30, NotifyChannelID: "chan-1", Enabled: true,
	}
	if err := repos.Rules.Create(ctx, rule); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repos.Rules.Get(ctx, "rule-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != rule {
		t.Fatalf("got %+v, want %+v", got, rule)
	}

	bySource, err := repos.Rules.List(ctx, "src-1")
	if err != nil || len(bySource) != 1 {
		t.Fatalf("List(src-1) = %+v, err %v", bySource, err)
	}
	if _, err := repos.Rules.List(ctx, "other-src"); err != nil {
		t.Fatalf("List(other-src): %v", err)
	}

	byChannel, err := repos.Rules.ListEnabledByChannel(ctx, "chan-1")
	if err != nil || len(byChannel) != 1 {
		t.Fatalf("ListEnabledByChannel = %+v, err %v", byChannel, err)
	}

	rule.Enabled = false
	if err := repos.Rules.Update(ctx, rule); err != nil {
		t.Fatalf("Update: %v", err)
	}
	byChannel, err = repos.Rules.ListEnabledByChannel(ctx, "chan-1")
	if err != nil || len(byChannel) != 0 {
		t.Fatalf("ListEnabledByChannel after disable = %+v, err %v", byChannel, err)
	}

	if err := repos.Rules.Delete(ctx, "rule-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repos.Rules.Get(ctx, "rule-1"); err != domain.ErrRuleNotFound {
		t.Fatalf("Get after delete: err = %v, want ErrRuleNotFound", err)
	}
}

func TestSQLiteRepository_AlertLifecycle(t *testing.T) {
	repos := infrastructure.NewSQLiteRepository(openTestDB(t))
	ctx := context.Background()

	ruleID := "rule-1"
	createdAt := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	alert := domain.Alert{ID: "alert-1", SourceID: "src-1", RuleID: &ruleID, Severity: domain.SeverityCritical, Message: "cpu high", CreatedAt: createdAt}
	if err := repos.Alerts.Create(ctx, alert); err != nil {
		t.Fatalf("Create: %v", err)
	}

	open, found, err := repos.Alerts.FindOpenByRule(ctx, ruleID)
	if err != nil || !found || open.ID != "alert-1" {
		t.Fatalf("FindOpenByRule = %+v, found %v, err %v", open, found, err)
	}

	active, err := repos.Alerts.List(ctx, "active")
	if err != nil || len(active) != 1 {
		t.Fatalf("List(active) = %+v, err %v", active, err)
	}

	resolvedAt := createdAt.Add(time.Minute)
	if err := repos.Alerts.Resolve(ctx, "alert-1", resolvedAt); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	_, found, err = repos.Alerts.FindOpenByRule(ctx, ruleID)
	if err != nil || found {
		t.Fatalf("FindOpenByRule after resolve: found %v, err %v", found, err)
	}
	resolved, err := repos.Alerts.List(ctx, "resolved")
	if err != nil || len(resolved) != 1 {
		t.Fatalf("List(resolved) = %+v, err %v", resolved, err)
	}

	system := domain.Alert{ID: "alert-2", SourceID: "src-2", Severity: domain.SeverityWarning, Message: "unreachable", CreatedAt: createdAt}
	if err := repos.Alerts.Create(ctx, system); err != nil {
		t.Fatalf("Create system alert: %v", err)
	}
	openSystem, found, err := repos.Alerts.FindOpenSystemAlert(ctx, "src-2")
	if err != nil || !found || openSystem.ID != "alert-2" || openSystem.RuleID != nil {
		t.Fatalf("FindOpenSystemAlert = %+v, found %v, err %v", openSystem, found, err)
	}
}
