package domain

import (
	"context"
	"time"
)

// Check is a discrete status snapshot, e.g. a TLS-expiry check.
type Check struct {
	SourceID      string
	Name          string
	TS            time.Time
	Status        string
	MetaJSON      string
	SchemaVersion string
}

// CheckQuery filters CheckRepository.Query. Zero-value fields are unfiltered ("any").
type CheckQuery struct {
	SourceID string
	Limit    int
}

// CheckRepository is the persistence port for Check, implemented by
// internal/telemetry/infrastructure.
type CheckRepository interface {
	Insert(ctx context.Context, check Check) error
	Query(ctx context.Context, query CheckQuery) ([]Check, error)
}
