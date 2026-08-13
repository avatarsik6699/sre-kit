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

type statusUpdateCall struct {
	sourceID string
	status   string
}

type fakeStatusUpdater struct{ calls []statusUpdateCall }

func (f *fakeStatusUpdater) MarkSeen(_ context.Context, sourceID, status string) error {
	f.calls = append(f.calls, statusUpdateCall{sourceID: sourceID, status: status})
	return nil
}

func TestIngestMetric_MarksSeenWithNoStatus(t *testing.T) {
	updater := &fakeStatusUpdater{}
	svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, &fakeEventRepo{}, application.WithSourceStatusUpdater(updater))

	if err := svc.IngestMetric(context.Background(), "src-1", "cpu.usage_percent", time.Now(), 1, nil); err != nil {
		t.Fatalf("IngestMetric: %v", err)
	}
	if len(updater.calls) != 1 || updater.calls[0] != (statusUpdateCall{sourceID: "src-1", status: ""}) {
		t.Fatalf("MarkSeen calls = %+v, want one call with empty status", updater.calls)
	}
}

func TestIngestEvent_MarksSeenWithNoStatus(t *testing.T) {
	updater := &fakeStatusUpdater{}
	svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, &fakeEventRepo{}, application.WithSourceStatusUpdater(updater))

	if err := svc.IngestEvent(context.Background(), "src-1", time.Now(), "warn", "msg", nil); err != nil {
		t.Fatalf("IngestEvent: %v", err)
	}
	if len(updater.calls) != 1 || updater.calls[0] != (statusUpdateCall{sourceID: "src-1", status: ""}) {
		t.Fatalf("MarkSeen calls = %+v, want one call with empty status", updater.calls)
	}
}

func TestIngestCheck_MarksSeenWithMappedStatus(t *testing.T) {
	tests := []struct {
		checkStatus string
		wantStatus  string
	}{
		{"ok", "ok"},
		{"warn", "ok"},
		{"critical", "error"},
	}
	for _, tt := range tests {
		t.Run(tt.checkStatus, func(t *testing.T) {
			updater := &fakeStatusUpdater{}
			svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, &fakeEventRepo{}, application.WithSourceStatusUpdater(updater))

			if err := svc.IngestCheck(context.Background(), "src-1", "http.reachable", time.Now(), tt.checkStatus, nil); err != nil {
				t.Fatalf("IngestCheck: %v", err)
			}
			if len(updater.calls) != 1 || updater.calls[0] != (statusUpdateCall{sourceID: "src-1", status: tt.wantStatus}) {
				t.Fatalf("MarkSeen calls = %+v, want one call with status %q", updater.calls, tt.wantStatus)
			}
		})
	}
}

func TestIngest_NilStatusUpdaterIsSafe(t *testing.T) {
	svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, &fakeEventRepo{})
	if err := svc.IngestCheck(context.Background(), "src-1", "http.reachable", time.Now(), "ok", nil); err != nil {
		t.Fatalf("IngestCheck without a status updater configured: %v", err)
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
