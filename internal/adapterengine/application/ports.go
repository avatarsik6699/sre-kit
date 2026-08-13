// Package application holds the adapter engine's use-cases (Runner, Scheduler, Supervisor) and the
// ports they depend on, per docs/STACK.md's "ports, not direct imports" cross-module rule:
// adapterengine defines the abstraction it needs here; the composition root (cmd/server/main.go)
// wires internal/telemetry/application.Service (and internal/sources/application.Service.Disable)
// in as the concrete implementation, so neither module imports the other.
package application

import (
	"context"
	"time"
)

// TelemetryIngestor is the port the adapter engine writes validated NDJSON data through. Every
// method uses only stdlib types so any type with matching methods (e.g.
// internal/telemetry/application.Service) satisfies it structurally, without importing this
// package.
type TelemetryIngestor interface {
	IngestMetric(ctx context.Context, sourceID, name string, ts time.Time, value float64, labels map[string]string) error
	IngestCheck(ctx context.Context, sourceID, name string, ts time.Time, status string, meta map[string]any) error
	IngestEvent(ctx context.Context, sourceID string, ts time.Time, level, message string, labels map[string]string) error
}

// SourceDisabler disables a source — used when a pull invocation trips the auto-disable rule
// (10 consecutive invalid NDJSON lines, docs/SPEC.md §4). A func type rather than an interface so
// main.go can adapt internal/sources/application.Service.Disable directly with a closure.
type SourceDisabler func(ctx context.Context, sourceID string) error
