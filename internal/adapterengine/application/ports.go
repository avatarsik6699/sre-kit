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

// PullOutcome describes the source-level result of one complete pull invocation. It deliberately
// does not expose subprocess errors outside adapterengine: consumers only need the stable
// connectivity semantics defined by docs/SPEC.md §6.
type PullOutcome string

const (
	PullOutcomeOK          PullOutcome = "ok"
	PullOutcomeUnreachable PullOutcome = "unreachable"
	PullOutcomeError       PullOutcome = "error"
)

// PullOutcomeReport carries the normalized result of one pull. EmittedTelemetry lets the
// composition root avoid overwriting the finer Source status already derived while ingesting a
// check, while a successful quiet adapter can still be marked seen and healthy.
type PullOutcomeReport struct {
	Outcome          PullOutcome
	EmittedTelemetry bool
}

// PullOutcomeReporter receives one report after every completed pull invocation. The scheduler
// treats reporting as best-effort: telemetry collection must continue even if status persistence
// or alert evaluation temporarily fails.
type PullOutcomeReporter interface {
	ReportPullOutcome(ctx context.Context, sourceID string, report PullOutcomeReport)
}
