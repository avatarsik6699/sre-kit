package application_test

import (
	"context"
	"testing"
	"time"

	"sre-kit/internal/telemetry/application"
	"sre-kit/internal/telemetry/domain"
)

type fakeMetricRepo struct{ inserted []domain.Metric }

func (f *fakeMetricRepo) Insert(_ context.Context, metric domain.Metric) error {
	f.inserted = append(f.inserted, metric)
	return nil
}
func (f *fakeMetricRepo) Query(context.Context, domain.MetricQuery) ([]domain.Metric, error) {
	return f.inserted, nil
}

type fakeCheckRepo struct{ inserted []domain.Check }

func (f *fakeCheckRepo) Insert(_ context.Context, check domain.Check) error {
	f.inserted = append(f.inserted, check)
	return nil
}
func (f *fakeCheckRepo) Query(context.Context, domain.CheckQuery) ([]domain.Check, error) {
	return f.inserted, nil
}

type fakeEventRepo struct{ inserted []domain.Event }

func (f *fakeEventRepo) Insert(_ context.Context, event domain.Event) error {
	f.inserted = append(f.inserted, event)
	return nil
}
func (f *fakeEventRepo) Query(context.Context, domain.EventQuery) ([]domain.Event, error) {
	return f.inserted, nil
}

func TestIngestMetric_StampsSchemaVersionAndLabels(t *testing.T) {
	metrics := &fakeMetricRepo{}
	svc := application.NewService(metrics, &fakeCheckRepo{}, &fakeEventRepo{})

	ts := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	err := svc.IngestMetric(context.Background(), "src-1", "cpu.usage_percent", ts, 42.5, map[string]string{"host": "vps-1"})
	if err != nil {
		t.Fatalf("IngestMetric: %v", err)
	}
	if len(metrics.inserted) != 1 {
		t.Fatalf("got %d inserted metrics, want 1", len(metrics.inserted))
	}
	got := metrics.inserted[0]
	if got.SchemaVersion != "1.0" {
		t.Fatalf("SchemaVersion = %q, want %q", got.SchemaVersion, "1.0")
	}
	if got.LabelsJSON != `{"host":"vps-1"}` {
		t.Fatalf("LabelsJSON = %q, want %q", got.LabelsJSON, `{"host":"vps-1"}`)
	}
}

func TestIngestCheck_NilMetaMarshalsToEmptyObject(t *testing.T) {
	checks := &fakeCheckRepo{}
	svc := application.NewService(&fakeMetricRepo{}, checks, &fakeEventRepo{})

	err := svc.IngestCheck(context.Background(), "src-1", "tls-expiry", time.Now(), "ok", nil)
	if err != nil {
		t.Fatalf("IngestCheck: %v", err)
	}
	if checks.inserted[0].MetaJSON != "{}" {
		t.Fatalf("MetaJSON = %q, want %q", checks.inserted[0].MetaJSON, "{}")
	}
}

func TestIngestEvent_PersistsFields(t *testing.T) {
	events := &fakeEventRepo{}
	svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, events)

	err := svc.IngestEvent(context.Background(), "src-1", time.Now(), "warn", "fail2ban banned 1.2.3.4", nil)
	if err != nil {
		t.Fatalf("IngestEvent: %v", err)
	}
	if events.inserted[0].Message != "fail2ban banned 1.2.3.4" {
		t.Fatalf("Message = %q, want the banned-IP message", events.inserted[0].Message)
	}
}

func TestQueryMetrics_ReturnsRepoResults(t *testing.T) {
	metrics := &fakeMetricRepo{}
	svc := application.NewService(metrics, &fakeCheckRepo{}, &fakeEventRepo{})
	_ = svc.IngestMetric(context.Background(), "src-1", "cpu.usage_percent", time.Now(), 1, nil)

	got, err := svc.QueryMetrics(context.Background(), domain.MetricQuery{SourceID: "src-1"})
	if err != nil {
		t.Fatalf("QueryMetrics: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d metrics, want 1", len(got))
	}
}
