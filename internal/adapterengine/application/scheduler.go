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
	runner *Runner

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewScheduler wires a Scheduler to the Runner it drives.
func NewScheduler(runner *Runner) *Scheduler {
	return &Scheduler{runner: runner, cancels: map[string]context.CancelFunc{}}
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
	_, _ = s.runner.RunOnce(ctx, job.SourceID, job.Command, job.Args, job.Config)
	// Per-invocation errors (spawn failure, non-zero exit, misbehaving-adapter auto-disable) are
	// Runner's concern; the scheduler's job is only to keep firing on interval. Surfacing
	// per-invocation status to an operator view is deferred to M4's Sources/Dashboard UI.
}
