// supervisor.go keeps stream-mode adapter subprocesses alive: it restarts on crash or on a missed
// heartbeat window, with exponential backoff between restarts (docs/SPEC.md §4).
package application

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"sre-kit/internal/contract"
)

const (
	supervisorInitialBackoff   = 1 * time.Second
	supervisorMaxBackoff       = 60 * time.Second
	supervisorDefaultHeartbeat = 30 * time.Second
)

// StreamJob describes one stream-mode adapter subprocess to keep alive.
type StreamJob struct {
	SourceID          string
	Command           string
	Args              []string
	Config            []byte
	HeartbeatInterval time.Duration
}

// Supervisor keeps a stream-mode adapter subprocess running for as long as its source is enabled.
// Invalid NDJSON lines are skipped (not auto-disabled) — the 10-consecutive-invalid-lines rule is
// pull-mode-specific (Runner); a mid-stream bad line here doesn't necessarily mean the adapter
// itself is misbehaving the way it does for a discrete pull invocation.
type Supervisor struct {
	spawner  Spawner
	ingestor TelemetryIngestor
	now      func() time.Time

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewSupervisor wires a Supervisor to its Spawner and TelemetryIngestor.
func NewSupervisor(spawner Spawner, ingestor TelemetryIngestor) *Supervisor {
	return &Supervisor{spawner: spawner, ingestor: ingestor, now: time.Now, cancels: map[string]context.CancelFunc{}}
}

// Supervise starts (or restarts) job's keep-alive loop under ctx and returns immediately; the loop
// runs in a background goroutine until ctx is cancelled or Stop(job.SourceID) is called.
func (s *Supervisor) Supervise(ctx context.Context, job StreamJob) {
	jobCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	if existing, ok := s.cancels[job.SourceID]; ok {
		existing()
	}
	s.cancels[job.SourceID] = cancel
	s.mu.Unlock()

	go s.runLoop(jobCtx, job)
}

// Stop cancels the keep-alive loop for sourceID, if running, and lets its subprocess exit via
// context cancellation.
func (s *Supervisor) Stop(sourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.cancels[sourceID]; ok {
		cancel()
		delete(s.cancels, sourceID)
	}
}

func (s *Supervisor) runLoop(ctx context.Context, job StreamJob) {
	backoff := supervisorInitialBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		err := s.runOnce(ctx, job)
		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if err == nil {
			backoff = supervisorInitialBackoff
		} else if backoff < supervisorMaxBackoff {
			backoff *= 2
			if backoff > supervisorMaxBackoff {
				backoff = supervisorMaxBackoff
			}
		}
	}
}

// runOnce spawns job's subprocess under a child context (so a heartbeat timeout or process exit
// can be scoped to this attempt without tearing down the whole Supervise loop) and reads lines
// until it exits, errors, or misses a heartbeat window.
func (s *Supervisor) runOnce(ctx context.Context, job StreamJob) error {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	source, err := s.spawner.Spawn(attemptCtx, job.Command, job.Args, job.Config)
	if err != nil {
		return fmt.Errorf("adapterengine: spawn: %w", err)
	}

	lines := make(chan string)
	scanDone := make(chan error, 1)
	go func() {
		for source.Scan() {
			lines <- source.Text()
		}
		scanDone <- source.Err()
	}()

	heartbeat := job.HeartbeatInterval
	if heartbeat <= 0 {
		heartbeat = supervisorDefaultHeartbeat
	}
	timer := time.NewTimer(heartbeat)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case line := <-lines:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(heartbeat)
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			raw := []byte(trimmed)
			if err := contract.ValidateLine(raw); err != nil {
				log.Printf("adapterengine: source %s: invalid stream line: %v", job.SourceID, err)
				continue
			}
			if err := ingestLine(ctx, s.ingestor, job.SourceID, raw, s.now().UTC()); err != nil {
				log.Printf("adapterengine: source %s: ingest failed: %v", job.SourceID, err)
			}
		case err := <-scanDone:
			_ = source.Wait()
			return err
		case <-timer.C:
			return fmt.Errorf("adapterengine: source %s missed heartbeat window (%s)", job.SourceID, heartbeat)
		}
	}
}
