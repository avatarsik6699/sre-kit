// Package infrastructure implements the telemetry repository ports (Metric/Check/Event) against
// SQLite. Each entity gets its own small repository type — a single Go type can't implement all
// three domain.*Repository interfaces at once, since they share the method names Insert/Query by
// design (each interface is scoped to its own file/entity in internal/telemetry/domain).
package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"sre-kit/internal/telemetry/domain"
)

// Repositories bundles the three SQLite-backed repositories NewSQLiteRepository constructs,
// ready to hand to telemetry/application.NewService.
type Repositories struct {
	Metrics domain.MetricRepository
	Checks  domain.CheckRepository
	Events  domain.EventRepository
}

// NewSQLiteRepository wires all three telemetry repository ports to the shared *sql.DB.
func NewSQLiteRepository(db *sql.DB) Repositories {
	return Repositories{
		Metrics: &metricRepository{db: db},
		Checks:  &checkRepository{db: db},
		Events:  &eventRepository{db: db},
	}
}

type metricRepository struct{ db *sql.DB }

func (r *metricRepository) Insert(ctx context.Context, metric domain.Metric) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO metrics (source_id, name, ts, value, labels_json, schema_version)
		VALUES (?, ?, ?, ?, ?, ?)`,
		metric.SourceID, metric.Name, metric.TS, metric.Value, metric.LabelsJSON, metric.SchemaVersion)
	if err != nil {
		return fmt.Errorf("telemetry: insert metric: %w", err)
	}
	return nil
}

func (r *metricRepository) Query(ctx context.Context, query domain.MetricQuery) ([]domain.Metric, error) {
	sqlQuery := `SELECT source_id, name, ts, value, labels_json, schema_version FROM metrics WHERE 1=1`
	var args []any
	if query.SourceID != "" {
		sqlQuery += ` AND source_id = ?`
		args = append(args, query.SourceID)
	}
	if query.Name != "" {
		sqlQuery += ` AND name = ?`
		args = append(args, query.Name)
	}
	if query.From != nil {
		sqlQuery += ` AND ts >= ?`
		args = append(args, *query.From)
	}
	if query.To != nil {
		sqlQuery += ` AND ts <= ?`
		args = append(args, *query.To)
	}
	sqlQuery += ` ORDER BY ts`

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query metrics: %w", err)
	}
	defer rows.Close()

	var metrics []domain.Metric
	for rows.Next() {
		var metric domain.Metric
		if err := rows.Scan(&metric.SourceID, &metric.Name, &metric.TS, &metric.Value, &metric.LabelsJSON, &metric.SchemaVersion); err != nil {
			return nil, fmt.Errorf("telemetry: scan metric: %w", err)
		}
		metrics = append(metrics, metric)
	}
	return metrics, rows.Err()
}

type checkRepository struct{ db *sql.DB }

func (r *checkRepository) Insert(ctx context.Context, check domain.Check) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO checks (source_id, name, ts, status, meta_json, schema_version)
		VALUES (?, ?, ?, ?, ?, ?)`,
		check.SourceID, check.Name, check.TS, check.Status, check.MetaJSON, check.SchemaVersion)
	if err != nil {
		return fmt.Errorf("telemetry: insert check: %w", err)
	}
	return nil
}

func (r *checkRepository) Query(ctx context.Context, query domain.CheckQuery) ([]domain.Check, error) {
	sqlQuery := `SELECT source_id, name, ts, status, meta_json, schema_version FROM checks WHERE 1=1`
	var args []any
	if query.SourceID != "" {
		sqlQuery += ` AND source_id = ?`
		args = append(args, query.SourceID)
	}
	sqlQuery += ` ORDER BY ts`

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query checks: %w", err)
	}
	defer rows.Close()

	var checks []domain.Check
	for rows.Next() {
		var check domain.Check
		if err := rows.Scan(&check.SourceID, &check.Name, &check.TS, &check.Status, &check.MetaJSON, &check.SchemaVersion); err != nil {
			return nil, fmt.Errorf("telemetry: scan check: %w", err)
		}
		checks = append(checks, check)
	}
	return checks, rows.Err()
}

type eventRepository struct{ db *sql.DB }

func (r *eventRepository) Insert(ctx context.Context, event domain.Event) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO events (source_id, ts, level, message, labels_json, schema_version)
		VALUES (?, ?, ?, ?, ?, ?)`,
		event.SourceID, event.TS, event.Level, event.Message, event.LabelsJSON, event.SchemaVersion)
	if err != nil {
		return fmt.Errorf("telemetry: insert event: %w", err)
	}
	return nil
}

func (r *eventRepository) Query(ctx context.Context, query domain.EventQuery) ([]domain.Event, error) {
	sqlQuery := `SELECT source_id, ts, level, message, labels_json, schema_version FROM events WHERE 1=1`
	var args []any
	if query.SourceID != "" {
		sqlQuery += ` AND source_id = ?`
		args = append(args, query.SourceID)
	}
	sqlQuery += ` ORDER BY ts DESC`
	if query.Limit > 0 {
		sqlQuery += ` LIMIT ?`
		args = append(args, query.Limit)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query events: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var event domain.Event
		if err := rows.Scan(&event.SourceID, &event.TS, &event.Level, &event.Message, &event.LabelsJSON, &event.SchemaVersion); err != nil {
			return nil, fmt.Errorf("telemetry: scan event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
