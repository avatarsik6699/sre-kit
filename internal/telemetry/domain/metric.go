// Package domain holds the Metric, Check, and Event entities and their repository ports, per
// docs/SPEC.md §3.
package domain

import (
	"context"
	"time"
)

// Metric is one time-series data point, e.g. cpu.usage_percent.
type Metric struct {
	SourceID      string
	Name          string
	TS            time.Time
	Value         float64
	LabelsJSON    string
	SchemaVersion string
}

// MetricQuery filters MetricRepository.Query. Zero-value fields are unfiltered ("any").
type MetricQuery struct {
	SourceID   string
	Name       string
	From       *time.Time
	To         *time.Time
	Limit      int
	Resolution string
}

// MetricRepository is the persistence port for Metric, implemented by
// internal/telemetry/infrastructure.
type MetricRepository interface {
	Insert(ctx context.Context, metric Metric) error
	Query(ctx context.Context, query MetricQuery) ([]Metric, error)
}
