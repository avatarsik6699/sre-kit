// Package application holds the telemetry use-cases: Ingest (structurally implements
// adapterengine's TelemetryIngestor port — see docs/STACK.md § Backend Architecture on ports over
// direct imports) and the Query use-cases behind /api/metrics, /api/checks, /api/events.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sre-kit/internal/telemetry/domain"
)

const schemaVersion = "1.0"

// Frame is one record pushed to Publisher after a successful ingest — deliberately a plain struct
// (not internal/platform/wshub.Frame) so this package doesn't import a websocket/hub dependency;
// cmd/server/main.go wires an adapter that translates Frame into wshub.Frame.
type Frame struct {
	Type     string // "metric" | "check" | "event", per docs/SPEC.md §4
	SourceID string
	Payload  any
}

// Publisher is the port telemetry/application uses to fan newly-ingested records out to live
// WebSocket subscribers (GET /api/stream, internal/platform/wshub). Optional — a Service with no
// Publisher configured simply doesn't push live updates.
type Publisher interface {
	Publish(frame Frame)
}

// SourceStatusUpdater is the port telemetry/application uses to keep a Source's rollup fields
// (sources.last_status/last_seen_at, docs/SPEC.md §3) current as telemetry arrives — kept as a
// narrow port, not a direct import of internal/sources, per docs/STACK.md's ports-over-direct-
// imports rule (mirrors adapterengine's own SourceDisabler). Optional — a Service with no
// SourceStatusUpdater configured simply doesn't touch source rollup state.
type SourceStatusUpdater interface {
	// MarkSeen updates sourceID's last_seen_at to now and, when status is non-empty, its
	// last_status. Metric/event ingestion passes "ok" (a source that's successfully emitting
	// telemetry is, by definition, reachable — see sourceStatusForCheck for how a check's finer
	// "ok"/"warn"/"critical" verdict maps down to this same coarser enum).
	MarkSeen(ctx context.Context, sourceID, status string) error
}

// AlertEvaluator is the port telemetry/application uses to hand new Metric/Check data and source
// connectivity status to the alert router (internal/alertrouter/application.Service) for rule
// evaluation, per docs/SPEC.md §6 — kept as a narrow port, not a direct import, mirroring
// SourceStatusUpdater. Optional — a Service with no AlertEvaluator configured simply doesn't
// evaluate alerts.
type AlertEvaluator interface {
	EvaluateMetric(ctx context.Context, sourceID, name string, value float64) error
	EvaluateCheck(ctx context.Context, sourceID, name, status string) error
	EvaluateSourceStatus(ctx context.Context, sourceID, status string) error
}

// Service implements telemetry ingest and query against the three domain repository ports.
type Service struct {
	metrics        domain.MetricRepository
	checks         domain.CheckRepository
	events         domain.EventRepository
	publisher      Publisher
	statusUpdater  SourceStatusUpdater
	alertEvaluator AlertEvaluator
}

// Option configures optional Service dependencies.
type Option func(*Service)

// WithPublisher wires pub as the Service's live-stream fan-out target.
func WithPublisher(pub Publisher) Option {
	return func(s *Service) { s.publisher = pub }
}

// WithSourceStatusUpdater wires updater to keep sources.last_status/last_seen_at current as
// telemetry is ingested.
func WithSourceStatusUpdater(updater SourceStatusUpdater) Option {
	return func(s *Service) { s.statusUpdater = updater }
}

// WithAlertEvaluator wires evaluator to run alert-rule/source-status evaluation as telemetry is
// ingested.
func WithAlertEvaluator(evaluator AlertEvaluator) Option {
	return func(s *Service) { s.alertEvaluator = evaluator }
}

// NewService wires a Service to its three repositories, plus any Options (e.g. WithPublisher).
func NewService(metrics domain.MetricRepository, checks domain.CheckRepository, events domain.EventRepository, opts ...Option) *Service {
	s := &Service{metrics: metrics, checks: checks, events: events}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) publish(frame Frame) {
	if s.publisher != nil {
		s.publisher.Publish(frame)
	}
}

// markSeen best-effort updates the source's rollup fields — a failure here (e.g. the source was
// deleted between the adapter spawning and this line landing) must never fail an already-persisted
// ingest, so the error is deliberately dropped (mirrors adapterengine/application.Runner's
// same-shaped auto-disable call).
func (s *Service) markSeen(ctx context.Context, sourceID, status string) {
	if s.statusUpdater != nil {
		_ = s.statusUpdater.MarkSeen(ctx, sourceID, status)
	}
}

// evaluateMetricAlerts and evaluateCheckAlerts are best-effort, mirroring markSeen: an evaluation
// failure (e.g. a malformed rule threshold) must never fail an already-persisted ingest.
func (s *Service) evaluateMetricAlerts(ctx context.Context, sourceID, name string, value float64) {
	if s.alertEvaluator != nil {
		_ = s.alertEvaluator.EvaluateMetric(ctx, sourceID, name, value)
	}
}

func (s *Service) evaluateCheckAlerts(ctx context.Context, sourceID, name, status string) {
	if s.alertEvaluator == nil {
		return
	}
	_ = s.alertEvaluator.EvaluateCheck(ctx, sourceID, name, status)
	_ = s.alertEvaluator.EvaluateSourceStatus(ctx, sourceID, sourceStatusForCheck(status))
}

// IngestMetric persists one metric point. ts is always stamped by the adapter runner
// (docs/SPEC.md §3), never trusted from the adapter/remote host beyond what's already in ts.
// Signature intentionally uses only stdlib types so it structurally satisfies
// adapterengine/application.TelemetryIngestor without either package importing the other.
func (s *Service) IngestMetric(ctx context.Context, sourceID, name string, ts time.Time, value float64, labels map[string]string) error {
	labelsJSON, err := marshalLabels(labels)
	if err != nil {
		return fmt.Errorf("telemetry: ingest metric: %w", err)
	}
	metric := domain.Metric{
		SourceID:      sourceID,
		Name:          name,
		TS:            ts,
		Value:         value,
		LabelsJSON:    labelsJSON,
		SchemaVersion: schemaVersion,
	}
	if err := s.metrics.Insert(ctx, metric); err != nil {
		return fmt.Errorf("telemetry: ingest metric: %w", err)
	}
	s.markSeen(ctx, sourceID, "ok")
	s.evaluateMetricAlerts(ctx, sourceID, name, value)
	s.publish(Frame{Type: "metric", SourceID: sourceID, Payload: map[string]any{
		"source_id": sourceID, "name": name, "ts": ts.Format(time.RFC3339), "value": value, "labels": labelsJSON,
	}})
	return nil
}

// IngestCheck persists one check snapshot.
func (s *Service) IngestCheck(ctx context.Context, sourceID, name string, ts time.Time, status string, meta map[string]any) error {
	metaJSON, err := marshalMeta(meta)
	if err != nil {
		return fmt.Errorf("telemetry: ingest check: %w", err)
	}
	check := domain.Check{
		SourceID:      sourceID,
		Name:          name,
		TS:            ts,
		Status:        status,
		MetaJSON:      metaJSON,
		SchemaVersion: schemaVersion,
	}
	if err := s.checks.Insert(ctx, check); err != nil {
		return fmt.Errorf("telemetry: ingest check: %w", err)
	}
	s.markSeen(ctx, sourceID, sourceStatusForCheck(status))
	s.evaluateCheckAlerts(ctx, sourceID, name, status)
	s.publish(Frame{Type: "check", SourceID: sourceID, Payload: map[string]any{
		"source_id": sourceID, "name": name, "ts": ts.Format(time.RFC3339), "status": status, "meta": metaJSON,
	}})
	return nil
}

// sourceStatusForCheck maps a check's status (contract.schema.json's "ok"/"warn"/"critical") down
// to sources.last_status's coarser enum (docs/SPEC.md §3: "ok"/"unreachable"/"error") — last_status
// tracks whether the source is reachable/reporting at all, not the fine-grained per-check verdict
// (that detail is already visible per-check via GET /api/checks). "warn" maps to "ok" since the
// source itself is still reporting fine; "unreachable" is reserved for sources that have never
// reported and is never re-derived here.
func sourceStatusForCheck(status string) string {
	if status == "critical" {
		return "error"
	}
	return "ok"
}

// IngestEvent persists one event.
func (s *Service) IngestEvent(ctx context.Context, sourceID string, ts time.Time, level, message string, labels map[string]string) error {
	labelsJSON, err := marshalLabels(labels)
	if err != nil {
		return fmt.Errorf("telemetry: ingest event: %w", err)
	}
	event := domain.Event{
		SourceID:      sourceID,
		TS:            ts,
		Level:         level,
		Message:       message,
		LabelsJSON:    labelsJSON,
		SchemaVersion: schemaVersion,
	}
	if err := s.events.Insert(ctx, event); err != nil {
		return fmt.Errorf("telemetry: ingest event: %w", err)
	}
	s.markSeen(ctx, sourceID, "ok")
	s.publish(Frame{Type: "event", SourceID: sourceID, Payload: map[string]any{
		"source_id": sourceID, "ts": ts.Format(time.RFC3339), "level": level, "message": message, "labels": labelsJSON,
	}})
	return nil
}

// QueryMetrics returns the time-series slice matching query.
func (s *Service) QueryMetrics(ctx context.Context, query domain.MetricQuery) ([]domain.Metric, error) {
	metrics, err := s.metrics.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query metrics: %w", err)
	}
	return metrics, nil
}

// QueryChecks returns the current statuses matching query.
func (s *Service) QueryChecks(ctx context.Context, query domain.CheckQuery) ([]domain.Check, error) {
	checks, err := s.checks.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query checks: %w", err)
	}
	return checks, nil
}

// QueryEvents returns the event feed matching query.
func (s *Service) QueryEvents(ctx context.Context, query domain.EventQuery) ([]domain.Event, error) {
	events, err := s.events.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query events: %w", err)
	}
	return events, nil
}

func marshalLabels(labels map[string]string) (string, error) {
	if labels == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(labels)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func marshalMeta(meta map[string]any) (string, error) {
	if meta == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
