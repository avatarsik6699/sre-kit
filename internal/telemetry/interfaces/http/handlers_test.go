package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sre-kit/internal/telemetry/application"
	"sre-kit/internal/telemetry/domain"
	telemetryhttp "sre-kit/internal/telemetry/interfaces/http"
)

type fakeMetricRepo struct{ items []domain.Metric }

func (f *fakeMetricRepo) Insert(_ context.Context, metric domain.Metric) error {
	f.items = append(f.items, metric)
	return nil
}
func (f *fakeMetricRepo) Query(_ context.Context, query domain.MetricQuery) ([]domain.Metric, error) {
	var out []domain.Metric
	for _, item := range f.items {
		if query.SourceID != "" && item.SourceID != query.SourceID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

type fakeCheckRepo struct{ items []domain.Check }

func (f *fakeCheckRepo) Insert(_ context.Context, check domain.Check) error {
	f.items = append(f.items, check)
	return nil
}
func (f *fakeCheckRepo) Query(context.Context, domain.CheckQuery) ([]domain.Check, error) {
	return f.items, nil
}

type fakeEventRepo struct{ items []domain.Event }

func (f *fakeEventRepo) Insert(_ context.Context, event domain.Event) error {
	f.items = append(f.items, event)
	return nil
}
func (f *fakeEventRepo) Query(_ context.Context, query domain.EventQuery) ([]domain.Event, error) {
	items := f.items
	if query.Limit > 0 && query.Limit < len(items) {
		items = items[:query.Limit]
	}
	return items, nil
}

func TestListMetrics_FiltersBySource(t *testing.T) {
	metrics := &fakeMetricRepo{}
	svc := application.NewService(metrics, &fakeCheckRepo{}, &fakeEventRepo{})
	if err := svc.IngestMetric(context.Background(), "src-1", "cpu.usage_percent", time.Now(), 1, nil); err != nil {
		t.Fatalf("IngestMetric: %v", err)
	}
	if err := svc.IngestMetric(context.Background(), "src-2", "cpu.usage_percent", time.Now(), 2, nil); err != nil {
		t.Fatalf("IngestMetric: %v", err)
	}

	mux := http.NewServeMux()
	telemetryhttp.NewHandlers(svc).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/metrics?source=src-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0]["source_id"] != "src-1" {
		t.Fatalf("got %+v, want one metric for src-1", got)
	}
}

func TestListMetrics_RejectsMalformedFrom(t *testing.T) {
	svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, &fakeEventRepo{})
	mux := http.NewServeMux()
	telemetryhttp.NewHandlers(svc).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/metrics?from=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestListEvents_RespectsLimit(t *testing.T) {
	events := &fakeEventRepo{}
	svc := application.NewService(&fakeMetricRepo{}, &fakeCheckRepo{}, events)
	for i := 0; i < 3; i++ {
		if err := svc.IngestEvent(context.Background(), "src-1", time.Now(), "info", "tick", nil); err != nil {
			t.Fatalf("IngestEvent: %v", err)
		}
	}

	mux := http.NewServeMux()
	telemetryhttp.NewHandlers(svc).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/events?limit=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
}

func TestListChecks_ReturnsStatus(t *testing.T) {
	checks := &fakeCheckRepo{}
	svc := application.NewService(&fakeMetricRepo{}, checks, &fakeEventRepo{})
	if err := svc.IngestCheck(context.Background(), "src-1", "tls-expiry", time.Now(), "ok", nil); err != nil {
		t.Fatalf("IngestCheck: %v", err)
	}

	mux := http.NewServeMux()
	telemetryhttp.NewHandlers(svc).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/checks", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0]["status"] != "ok" {
		t.Fatalf("got %+v, want one check with status ok", got)
	}
}
