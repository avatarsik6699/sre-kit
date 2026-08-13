// Package infrastructure implements the Source repository port against SQLite.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sre-kit/internal/sources/domain"
)

// SQLiteRepository implements domain.Repository against the shared *sql.DB.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository wraps db as a domain.Repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Create(ctx context.Context, source domain.Source) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sources (id, adapter_name, config_json, enabled, last_status, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		source.ID, source.AdapterName, source.ConfigJSON, source.Enabled, source.LastStatus, nullableTime(source.LastSeenAt))
	if err != nil {
		return fmt.Errorf("sources: insert: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Update(ctx context.Context, source domain.Source) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE sources SET adapter_name = ?, config_json = ?, enabled = ?, last_status = ?, last_seen_at = ?
		WHERE id = ?`,
		source.AdapterName, source.ConfigJSON, source.Enabled, source.LastStatus, nullableTime(source.LastSeenAt), source.ID)
	if err != nil {
		return fmt.Errorf("sources: update: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (domain.Source, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, adapter_name, config_json, enabled, last_status, last_seen_at
		FROM sources WHERE id = ?`, id)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Source{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Source{}, fmt.Errorf("sources: get: %w", err)
	}
	return source, nil
}

func (r *SQLiteRepository) List(ctx context.Context) ([]domain.Source, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, adapter_name, config_json, enabled, last_status, last_seen_at
		FROM sources ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sources: list: %w", err)
	}
	defer rows.Close()

	var sources []domain.Source
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, fmt.Errorf("sources: scan: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sources: rows: %w", err)
	}
	return sources, nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sources: delete: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSource(scanner rowScanner) (domain.Source, error) {
	var source domain.Source
	var lastSeenAt sql.NullTime
	if err := scanner.Scan(&source.ID, &source.AdapterName, &source.ConfigJSON, &source.Enabled, &source.LastStatus, &lastSeenAt); err != nil {
		return domain.Source{}, err
	}
	if lastSeenAt.Valid {
		source.LastSeenAt = &lastSeenAt.Time
	}
	return source, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
