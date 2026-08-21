package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	rawRetention = 30 * 24 * time.Hour
)

// Maintenance rolls raw metrics into UTC hour buckets and applies the bounded retention policy.
// It never removes Source configuration, projects, alerts, ingestion credentials or secrets.
type Maintenance struct {
	db  *sql.DB
	now func() time.Time
}

func NewMaintenance(db *sql.DB) *Maintenance { return &Maintenance{db: db, now: time.Now} }

func (m *Maintenance) Run(ctx context.Context) (err error) {
	started := m.now().UTC()
	result, e := m.db.ExecContext(ctx, `INSERT INTO maintenance_runs (started_at,status) VALUES (?, 'running')`, started)
	if e != nil {
		return fmt.Errorf("telemetry maintenance: start: %w", e)
	}
	runID, _ := result.LastInsertId()
	tx, e := m.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			_, _ = m.db.ExecContext(context.Background(), `UPDATE maintenance_runs SET finished_at=?,status='error',error_message=? WHERE id=?`, m.now().UTC(), err.Error(), runID)
		}
	}()
	rawCutoff := started.Add(-rawRetention)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO metrics_rollup (source_id,name,labels_json,bucket_ts,min_value,max_value,avg_value,sample_count)
		SELECT source_id,name,labels_json,substr(CAST(ts AS TEXT),1,13) || ':00:00Z',MIN(value),MAX(value),AVG(value),COUNT(*)
		FROM metrics WHERE ts < ? GROUP BY source_id,name,labels_json,substr(CAST(ts AS TEXT),1,13)
		ON CONFLICT(source_id,name,labels_json,bucket_ts) DO UPDATE SET
		min_value=excluded.min_value,max_value=excluded.max_value,avg_value=excluded.avg_value,sample_count=excluded.sample_count`, rawCutoff)
	if err != nil {
		return fmt.Errorf("telemetry maintenance: roll up: %w", err)
	}
	rawDeleted := int64(0)
	for _, table := range []string{"metrics", "checks", "events"} {
		res, execErr := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE ts < ?`, rawCutoff)
		if execErr != nil {
			return fmt.Errorf("telemetry maintenance: prune %s: %w", table, execErr)
		}
		n, _ := res.RowsAffected()
		rawDeleted += n
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM metrics_rollup WHERE bucket_ts < ?`, started.AddDate(0, -13, 0))
	if err != nil {
		return fmt.Errorf("telemetry maintenance: prune rollups: %w", err)
	}
	rollupsDeleted, _ := res.RowsAffected()
	res, err = tx.ExecContext(ctx, `DELETE FROM ingestion_batches WHERE received_at < ?`, rawCutoff)
	if err != nil {
		return fmt.Errorf("telemetry maintenance: prune ingestion batches: %w", err)
	}
	batchesDeleted, _ := res.RowsAffected()
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("telemetry maintenance: commit: %w", err)
	}
	_, err = m.db.ExecContext(ctx, `UPDATE maintenance_runs SET finished_at=?,status='ok',raw_deleted=?,rollups_deleted=?,batches_deleted=? WHERE id=?`, m.now().UTC(), rawDeleted, rollupsDeleted, batchesDeleted, runID)
	return err
}
