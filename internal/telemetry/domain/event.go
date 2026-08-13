package domain

import (
	"context"
	"time"
)

// Event is a discrete log/occurrence, e.g. a fail2ban ban.
type Event struct {
	SourceID      string
	TS            time.Time
	Level         string
	Message       string
	LabelsJSON    string
	SchemaVersion string
}

// EventQuery filters EventRepository.Query. Zero-value fields are unfiltered ("any"); Limit <= 0
// means "no limit".
type EventQuery struct {
	SourceID string
	Limit    int
}

// EventRepository is the persistence port for Event, implemented by
// internal/telemetry/infrastructure.
type EventRepository interface {
	Insert(ctx context.Context, event Event) error
	Query(ctx context.Context, query EventQuery) ([]Event, error)
}
