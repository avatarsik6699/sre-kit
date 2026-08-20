// scheduler.go invokes pull-mode adapters on a per-source interval (docs/SPEC.md §4).
package application

import (
	"context"
	"sync"
	"time"
)

// PullJob describes one pull-mode adapter to invoke on a fixed interval.
type PullJob struct {
	SourceID string
	Command  string
	Args     []string
	Config   []byte
	Interval time.Duration
}

// Scheduler runs each scheduled PullJob's Runner.RunOnce on its own ticker, one goroutine per
// source, until the job is cancelled or the Scheduler is shut down.
type Scheduler struct {
	runner          *Runner
	reporter        PullOutcomeReporter
	failureReporter PullFailureReporter

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// PullFailureReporter receives only a stable failure class, never raw stderr or resolved config.
type PullFailureReporter interface {
	ReportPullFailure(ctx context.Context, sourceID string, class PullFailureClass)
}

// NewScheduler wires a Scheduler to the Runner it drives and, optionally, the source-outcome
// reporter that keeps connectivity state current after every pull.
func NewScheduler(runner *Runner, reporter ...PullOutcomeReporter) *Scheduler {
	s := &Scheduler{runner: runner, cancels: map[string]context.CancelFunc{}}
	if len(reporter) > 0 {
		s.reporter = reporter[0]
		if failureReporter, ok := reporter[0].(PullFailureReporter); ok {
			s.failureReporter = failureReporter
		}
	}
	return s
}

// Schedule starts (or restarts, if already scheduled) job under ctx. RunOnce fires immediately,
// then again every job.Interval, until ctx is cancelled or Cancel(job.SourceID) is called.
func (s *Scheduler) Schedule(ctx context.Context, job PullJob) {
	jobCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	if existing, ok := s.cancels[job.SourceID]; ok {
		existing()
	}
	s.cancels[job.SourceID] = cancel
	s.mu.Unlock()

	go s.runLoop(jobCtx, job)
}

// Cancel stops the scheduled job for sourceID, if any.
func (s *Scheduler) Cancel(sourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.cancels[sourceID]; ok {
		cancel()
		delete(s.cancels, sourceID)
	}
}

// Shutdown cancels every scheduled job.
func (s *Scheduler) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sourceID, cancel := range s.cancels {
		cancel()
		delete(s.cancels, sourceID)
	}
}

func (s *Scheduler) runLoop(ctx context.Context, job PullJob) {
	s.invoke(ctx, job)

	interval := job.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.invoke(ctx, job)
		}
	}
}

func (s *Scheduler) invoke(ctx context.Context, job PullJob) {
	result, err := s.runner.RunOnce(ctx, job.SourceID, job.Command, job.Args, job.Config)
	// Cancel/disable/shutdown owns this context. Its cancellation is not a target connectivity
	// outcome and must not race a disabled or deleted Source back to "unreachable".
	if s.reporter == nil || ctx.Err() != nil {
		return
	}
	if s.failureReporter != nil {
		switch {
		case result.AutoDisabled || result.InvalidLines > 0:
			s.failureReporter.ReportPullFailure(ctx, job.SourceID, PullFailureInvalidOutput)
		case err != nil:
			s.failureReporter.ReportPullFailure(ctx, job.SourceID, PullFailureClassOf(err))
		}
	}

	report := PullOutcomeReport{EmittedTelemetry: result.LinesProcessed > 0}
	switch {
	case result.AutoDisabled || result.InvalidLines > 0:
		report.Outcome = PullOutcomeError
	case err != nil:
		report.Outcome = PullOutcomeUnreachable
	default:
		report.Outcome = PullOutcomeOK
	}
	s.reporter.ReportPullOutcome(ctx, job.SourceID, report)
}
