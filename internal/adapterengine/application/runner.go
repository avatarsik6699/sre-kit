// runner.go implements pull-mode adapter invocation: spawn the subprocess, parse its NDJSON
// stdout, validate every line against contract.schema.json, ingest valid lines, and auto-disable
// the source after too many consecutive invalid lines (docs/SPEC.md §4).
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"sre-kit/internal/contract"
)

// PullFailureClass is the secret-safe diagnostic category for a failed pull. Raw adapter stderr
// may contain target-specific or sensitive content, so scheduler logs use only this class.
type PullFailureClass string

const (
	PullFailureSpawn         PullFailureClass = "spawn"
	PullFailureRead          PullFailureClass = "read"
	PullFailureSubprocess    PullFailureClass = "subprocess"
	PullFailureIngest        PullFailureClass = "ingest"
	PullFailureInvalidOutput PullFailureClass = "invalid_output"
	PullFailureUnknown       PullFailureClass = "unknown"
)

type pullRunError struct {
	class PullFailureClass
	err   error
}

func (e *pullRunError) Error() string { return "adapterengine: pull failed: " + string(e.class) }
func (e *pullRunError) Unwrap() error { return e.err }

func newPullRunError(class PullFailureClass, err error) error {
	return &pullRunError{class: class, err: err}
}

// PullFailureClassOf extracts the stable category without exposing the wrapped adapter error.
func PullFailureClassOf(err error) PullFailureClass {
	var pullErr *pullRunError
	if errors.As(err, &pullErr) {
		return pullErr.class
	}
	return PullFailureUnknown
}

// maxConsecutiveInvalidLines auto-disables a source after this many bad NDJSON lines in a row.
const maxConsecutiveInvalidLines = 10

// LineSource streams NDJSON lines from a running adapter subprocess and reports the exit error.
// Implemented by internal/adapterengine/infrastructure.Subprocess; kept as its own interface here
// so Runner is unit-testable without a real subprocess.
type LineSource interface {
	Scan() bool
	Text() string
	Err() error
	Wait() error
}

// Spawner starts an adapter subprocess in pull mode: config is written to stdin, NDJSON lines are
// read from stdout.
type Spawner interface {
	Spawn(ctx context.Context, command string, args []string, config []byte) (LineSource, error)
}

// Runner drives one pull-mode adapter invocation.
type Runner struct {
	spawner  Spawner
	ingestor TelemetryIngestor
	disable  SourceDisabler
	now      func() time.Time // overridable in tests
}

// NewRunner wires a Runner to its Spawner, TelemetryIngestor, and SourceDisabler.
func NewRunner(spawner Spawner, ingestor TelemetryIngestor, disable SourceDisabler) *Runner {
	return &Runner{spawner: spawner, ingestor: ingestor, disable: disable, now: time.Now}
}

// RunResult summarizes one pull invocation.
type RunResult struct {
	LinesProcessed int
	InvalidLines   int
	AutoDisabled   bool
}

// ndjsonLine is the union of every field a contract entity may carry, used to route a
// schema-validated line to the right Ingest* call without duplicating contract.schema.json's
// shape in a second type per entity.
type ndjsonLine struct {
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	Value   float64           `json:"value"`
	Labels  map[string]string `json:"labels"`
	Status  string            `json:"status"`
	Meta    map[string]any    `json:"meta"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
}

// RunOnce spawns command with args, feeds it config on stdin, and processes its NDJSON stdout
// lines until the process exits. sourceID scopes ingestion and (on misbehavior) the disable call
// — it is never taken from the line itself. Per docs/SPEC.md §3, the persisted timestamp is always
// stamped by the core at ingest time (r.now()), never trusted from the adapter/remote host.
func (r *Runner) RunOnce(ctx context.Context, sourceID string, command string, args []string, config []byte) (RunResult, error) {
	lines, err := r.spawner.Spawn(ctx, command, args, config)
	if err != nil {
		return RunResult{}, newPullRunError(PullFailureSpawn, err)
	}

	var result RunResult
	consecutiveInvalid := 0
	for lines.Scan() {
		raw := []byte(strings.TrimSpace(lines.Text()))
		if len(raw) == 0 {
			continue
		}
		if err := contract.ValidateLine(raw); err != nil {
			result.InvalidLines++
			consecutiveInvalid++
			if consecutiveInvalid >= maxConsecutiveInvalidLines {
				result.AutoDisabled = true
				if r.disable != nil {
					_ = r.disable(ctx, sourceID)
				}
				break
			}
			continue
		}
		consecutiveInvalid = 0
		result.LinesProcessed++
		if err := ingestLine(ctx, r.ingestor, sourceID, raw, r.now().UTC()); err != nil {
			return result, newPullRunError(PullFailureIngest, err)
		}
	}
	if err := lines.Err(); err != nil {
		return result, newPullRunError(PullFailureRead, err)
	}
	if err := lines.Wait(); err != nil && !result.AutoDisabled {
		return result, newPullRunError(PullFailureSubprocess, err)
	}
	return result, nil
}

// ingestLine decodes an already-schema-validated NDJSON line and routes it to the matching
// TelemetryIngestor method, stamping ts as the persisted timestamp (docs/SPEC.md §3: ts is always
// stamped by the core, never trusted from the adapter/remote host). Shared by Runner (pull mode)
// and Supervisor (stream mode) so the routing logic isn't duplicated per execution mode.
func ingestLine(ctx context.Context, ingestor TelemetryIngestor, sourceID string, raw []byte, ts time.Time) error {
	var line ndjsonLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return fmt.Errorf("decode already-validated line: %w", err)
	}
	switch line.Type {
	case "metric":
		return ingestor.IngestMetric(ctx, sourceID, line.Name, ts, line.Value, line.Labels)
	case "check":
		return ingestor.IngestCheck(ctx, sourceID, line.Name, ts, line.Status, line.Meta)
	case "event":
		return ingestor.IngestEvent(ctx, sourceID, ts, line.Level, line.Message, line.Labels)
	default:
		// "alert" and any future contract-additive type are core-generated, not ingested from an
		// adapter's pull/stream output.
		return nil
	}
}
