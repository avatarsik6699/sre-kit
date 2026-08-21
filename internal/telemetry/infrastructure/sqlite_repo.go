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
		INSERT OR IGNORE INTO metrics (source_id, name, ts, value, labels_json, schema_version)
		VALUES (?, ?, ?, ?, ?, ?)`,
		metric.SourceID, metric.Name, metric.TS, metric.Value, metric.LabelsJSON, metric.SchemaVersion)
	if err != nil {
		return fmt.Errorf("telemetry: insert metric: %w", err)
	}
	return nil
}

func (r *metricRepository) Query(ctx context.Context, query domain.MetricQuery) ([]domain.Metric, error) {
	if query.Resolution == "hour" {
		return r.queryRollups(ctx, query)
	}
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
	sqlQuery += ` ORDER BY ts DESC LIMIT ?`
	limit := query.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	args = append(args, limit)

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
	for i, j := 0, len(metrics)-1; i < j; i, j = i+1, j-1 {
		metrics[i], metrics[j] = metrics[j], metrics[i]
	}
	return metrics, rows.Err()
}

func (r *metricRepository) queryRollups(ctx context.Context, query domain.MetricQuery) ([]domain.Metric, error) {
	sqlQuery := `SELECT source_id, name, bucket_ts, avg_value, labels_json FROM metrics_rollup WHERE 1=1`
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
		sqlQuery += ` AND bucket_ts >= ?`
		args = append(args, *query.From)
	}
	if query.To != nil {
		sqlQuery += ` AND bucket_ts <= ?`
		args = append(args, *query.To)
	}
	limit := query.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	sqlQuery += ` ORDER BY bucket_ts DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query rollups: %w", err)
	}
	defer rows.Close()
	var out []domain.Metric
	for rows.Next() {
		var m domain.Metric
		if err := rows.Scan(&m.SourceID, &m.Name, &m.TS, &m.Value, &m.LabelsJSON); err != nil {
			return nil, fmt.Errorf("telemetry: scan rollup: %w", err)
		}
		m.SchemaVersion = "1.0"
		out = append(out, m)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

type checkRepository struct{ db *sql.DB }

func (r *checkRepository) Insert(ctx context.Context, check domain.Check) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO checks (source_id, name, ts, status, meta_json, schema_version)
		VALUES (?, ?, ?, ?, ?, ?)`,
		check.SourceID, check.Name, check.TS, check.Status, check.MetaJSON, check.SchemaVersion)
	if err != nil {
		return fmt.Errorf("telemetry: insert check: %w", err)
	}
	return nil
}

func (r *checkRepository) Query(ctx context.Context, query domain.CheckQuery) ([]domain.Check, error) {
	sqlQuery := `SELECT c.source_id, c.name, c.ts, c.status, c.meta_json, c.schema_version
		FROM checks c WHERE NOT EXISTS (
			SELECT 1 FROM checks newer
			WHERE newer.source_id = c.source_id AND newer.name = c.name AND newer.ts > c.ts
		)`
	var args []any
	if query.SourceID != "" {
		sqlQuery += ` AND c.source_id = ?`
		args = append(args, query.SourceID)
	}
	limit := query.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sqlQuery += ` ORDER BY c.ts DESC LIMIT ?`
	args = append(args, limit)

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
		INSERT OR IGNORE INTO events (source_id, ts, level, message, labels_json, schema_version)
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
	limit := query.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	sqlQuery += ` LIMIT ?`
	args = append(args, limit)

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
