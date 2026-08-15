// Package infrastructure implements the Host repository port against SQLite, plus the real SSH
// connection prober.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sre-kit/internal/hosts/domain"
)

// SQLiteRepository implements domain.Repository against the shared *sql.DB.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository wraps db as a domain.Repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Create(ctx context.Context, host domain.Host) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO hosts (id, label, address, ssh_port, ssh_user, ssh_key_secret_ref,
			host_key_fingerprint, docker_available, last_connected_at, last_status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		host.ID, host.Label, host.Address, host.SSHPort, host.SSHUser, host.SSHKeySecretRef,
		host.HostKeyFingerprint, host.DockerAvailable, nullableTime(host.LastConnectedAt), host.LastStatus, host.CreatedAt)
	if err != nil {
		return fmt.Errorf("hosts: insert: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Update(ctx context.Context, host domain.Host) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE hosts SET label = ?, address = ?, ssh_port = ?, ssh_user = ?, ssh_key_secret_ref = ?,
			host_key_fingerprint = ?, docker_available = ?, last_connected_at = ?, last_status = ?
		WHERE id = ?`,
		host.Label, host.Address, host.SSHPort, host.SSHUser, host.SSHKeySecretRef,
		host.HostKeyFingerprint, host.DockerAvailable, nullableTime(host.LastConnectedAt), host.LastStatus, host.ID)
	if err != nil {
		return fmt.Errorf("hosts: update: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (domain.Host, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, label, address, ssh_port, ssh_user, ssh_key_secret_ref, host_key_fingerprint,
			docker_available, last_connected_at, last_status, created_at
		FROM hosts WHERE id = ?`, id)
	host, err := scanHost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Host{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Host{}, fmt.Errorf("hosts: get: %w", err)
	}
	return host, nil
}

func (r *SQLiteRepository) List(ctx context.Context) ([]domain.Host, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, label, address, ssh_port, ssh_user, ssh_key_secret_ref, host_key_fingerprint,
			docker_available, last_connected_at, last_status, created_at
		FROM hosts ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("hosts: list: %w", err)
	}
	defer rows.Close()

	var hosts []domain.Host
	for rows.Next() {
		host, err := scanHost(rows)
		if err != nil {
			return nil, fmt.Errorf("hosts: scan: %w", err)
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hosts: rows: %w", err)
	}
	return hosts, nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("hosts: delete: %w", err)
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

func scanHost(scanner rowScanner) (domain.Host, error) {
	var host domain.Host
	var lastConnectedAt sql.NullTime
	if err := scanner.Scan(&host.ID, &host.Label, &host.Address, &host.SSHPort, &host.SSHUser,
		&host.SSHKeySecretRef, &host.HostKeyFingerprint, &host.DockerAvailable, &lastConnectedAt,
		&host.LastStatus, &host.CreatedAt); err != nil {
		return domain.Host{}, err
	}
	if lastConnectedAt.Valid {
		host.LastConnectedAt = &lastConnectedAt.Time
	}
	return host, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
