package infrastructure

import (
	"context"
	"database/sql"
)

type BatchRepository struct{ db *sql.DB }

func NewBatchRepository(db *sql.DB) *BatchRepository { return &BatchRepository{db: db} }
func (r *BatchRepository) Reserve(ctx context.Context, sourceID, key string, count int) (bool, error) {
	res, err := r.db.ExecContext(ctx, `INSERT OR IGNORE INTO ingestion_batches (source_id,idempotency_key,record_count) VALUES (?,?,?)`, sourceID, key, count)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
func (r *BatchRepository) Release(ctx context.Context, sourceID, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ingestion_batches WHERE source_id=? AND idempotency_key=?`, sourceID, key)
	return err
}
