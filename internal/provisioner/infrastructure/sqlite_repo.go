package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sre-kit/internal/provisioner/domain"
)

// SQLiteRepository implements domain.Repository against the shared *sql.DB.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository wraps db as a domain.Repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Create(ctx context.Context, run domain.Run) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO provisioning_runs (id, host_id, preset_name, status, step, error_message,
			admin_password_secret_ref, produced_source_id, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.HostID, run.PresetName, run.Status, run.Step, run.ErrorMessage,
		run.AdminPasswordSecretRef, nullableString(run.ProducedSourceID), run.StartedAt, nullableTime(run.FinishedAt))
	if err != nil {
		return fmt.Errorf("provisioner: insert: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Update(ctx context.Context, run domain.Run) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE provisioning_runs SET status = ?, step = ?, error_message = ?,
			admin_password_secret_ref = ?, produced_source_id = ?, finished_at = ?
		WHERE id = ?`,
		run.Status, run.Step, run.ErrorMessage, run.AdminPasswordSecretRef,
		nullableString(run.ProducedSourceID), nullableTime(run.FinishedAt), run.ID)
	if err != nil {
		return fmt.Errorf("provisioner: update: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (domain.Run, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, host_id, preset_name, status, step, error_message, admin_password_secret_ref,
			produced_source_id, started_at, finished_at
		FROM provisioning_runs WHERE id = ?`, id)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Run{}, fmt.Errorf("provisioner: get: %w", err)
	}
	return run, nil
}

func (r *SQLiteRepository) ListByHost(ctx context.Context, hostID string) ([]domain.Run, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, host_id, preset_name, status, step, error_message, admin_password_secret_ref,
			produced_source_id, started_at, finished_at
		FROM provisioning_runs WHERE host_id = ? ORDER BY started_at DESC`, hostID)
	if err != nil {
		return nil, fmt.Errorf("provisioner: list: %w", err)
	}
	defer rows.Close()

	var runs []domain.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("provisioner: scan: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("provisioner: rows: %w", err)
	}
	return runs, nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(scanner rowScanner) (domain.Run, error) {
	var run domain.Run
	var producedSourceID sql.NullString
	var finishedAt sql.NullTime
	if err := scanner.Scan(&run.ID, &run.HostID, &run.PresetName, &run.Status, &run.Step,
		&run.ErrorMessage, &run.AdminPasswordSecretRef, &producedSourceID, &run.StartedAt, &finishedAt); err != nil {
		return domain.Run{}, err
	}
	if producedSourceID.Valid {
		run.ProducedSourceID = producedSourceID.String
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}
	return run, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
