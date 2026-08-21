package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"sre-kit/internal/projects/domain"
)

type SQLiteRepository struct{ db *sql.DB }

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository { return &SQLiteRepository{db: db} }

func (r *SQLiteRepository) Create(ctx context.Context, p domain.Project) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO projects (id,name,slug,description,created_at,updated_at) VALUES (?,?,?,?,?,?)`, p.ID, p.Name, p.Slug, p.Description, p.CreatedAt, p.UpdatedAt)
	return err
}
func (r *SQLiteRepository) Update(ctx context.Context, p domain.Project) error {
	res, err := r.db.ExecContext(ctx, `UPDATE projects SET name=?,slug=?,description=?,updated_at=? WHERE id=?`, p.Name, p.Slug, p.Description, p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func scan(row interface{ Scan(...any) error }) (domain.Project, error) {
	var p domain.Project
	err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (r *SQLiteRepository) Get(ctx context.Context, id string) (domain.Project, error) {
	p, err := scan(r.db.QueryRowContext(ctx, `SELECT id,name,slug,description,created_at,updated_at FROM projects WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	return p, err
}
func (r *SQLiteRepository) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,slug,description,created_at,updated_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Project{}
	for rows.Next() {
		p, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
