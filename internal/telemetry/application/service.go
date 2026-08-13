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

// Service implements telemetry ingest and query against the three domain repository ports.
type Service struct {
	metrics domain.MetricRepository
	checks  domain.CheckRepository
	events  domain.EventRepository
}

// NewService wires a Service to its three repositories.
func NewService(metrics domain.MetricRepository, checks domain.CheckRepository, events domain.EventRepository) *Service {
	return &Service{metrics: metrics, checks: checks, events: events}
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
	return nil
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
