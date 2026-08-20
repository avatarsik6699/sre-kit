package application_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"sre-kit/internal/adapterengine/application"
)

// countingSpawner returns a fresh empty fakeLineSource on every Spawn call and counts invocations.
type countingSpawner struct{ calls int32 }

func (c *countingSpawner) Spawn(context.Context, string, []string, []byte) (application.LineSource, error) {
	atomic.AddInt32(&c.calls, 1)
	return &fakeLineSource{}, nil
}

type outcomeRecorder struct {
	reports chan application.PullOutcomeReport
}

func (r *outcomeRecorder) ReportPullOutcome(_ context.Context, _ string, report application.PullOutcomeReport) {
	r.reports <- report
}

type cancellationSpawner struct{}

func (cancellationSpawner) Spawn(ctx context.Context, _ string, _ []string, _ []byte) (application.LineSource, error) {
	return &cancellationLineSource{ctx: ctx}, nil
}

type cancellationLineSource struct {
	ctx context.Context
}

func (*cancellationLineSource) Scan() bool   { return false }
func (*cancellationLineSource) Text() string { return "" }
func (*cancellationLineSource) Err() error   { return nil }
func (s *cancellationLineSource) Wait() error {
	<-s.ctx.Done()
	return s.ctx.Err()
}

func TestScheduler_FiresImmediatelyAndOnInterval(t *testing.T) {
	spawner := &countingSpawner{}
	runner := application.NewRunner(spawner, &fakeIngestor{}, nil)
	scheduler := application.NewScheduler(runner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.Schedule(ctx, application.PullJob{SourceID: "src-1", Command: "stub", Interval: 20 * time.Millisecond})

	time.Sleep(70 * time.Millisecond)
	scheduler.Cancel("src-1")

	calls := atomic.LoadInt32(&spawner.calls)
	if calls < 2 {
		t.Fatalf("got %d spawns in 70ms at 20ms interval, want at least 2 (immediate + ticks)", calls)
	}
}

func TestScheduler_CancelStopsFurtherInvocations(t *testing.T) {
	spawner := &countingSpawner{}
	runner := application.NewRunner(spawner, &fakeIngestor{}, nil)
	scheduler := application.NewScheduler(runner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.Schedule(ctx, application.PullJob{SourceID: "src-1", Command: "stub", Interval: 10 * time.Millisecond})
	time.Sleep(15 * time.Millisecond)
	scheduler.Cancel("src-1")
	afterCancel := atomic.LoadInt32(&spawner.calls)

	time.Sleep(40 * time.Millisecond)
	if got := atomic.LoadInt32(&spawner.calls); got != afterCancel {
		t.Fatalf("spawns after Cancel: got %d, want unchanged at %d", got, afterCancel)
	}
}

func TestScheduler_ReportsEveryPullOutcome(t *testing.T) {
	tests := []struct {
		name    string
		spawn   *fakeSpawner
		want    application.PullOutcome
		emitted bool
	}{
		{
			name:  "quiet success",
			spawn: &fakeSpawner{source: &fakeLineSource{}},
			want:  application.PullOutcomeOK,
		},
		{
			name: "success with telemetry",
			spawn: &fakeSpawner{source: &fakeLineSource{lines: []string{
				`{"type":"metric","source_id":"adapter","name":"cpu.usage_percent","timestamp":"2026-08-20T12:00:00Z","value":1}`,
			}}},
			want:    application.PullOutcomeOK,
			emitted: true,
		},
		{
			name:  "subprocess failure",
			spawn: &fakeSpawner{err: errors.New("dial failed")},
			want:  application.PullOutcomeUnreachable,
		},
		{
			name: "invalid output",
			spawn: &fakeSpawner{source: &fakeLineSource{lines: []string{
				`{"type":"bogus"}`,
			}}},
			want: application.PullOutcomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &outcomeRecorder{reports: make(chan application.PullOutcomeReport, 1)}
			runner := application.NewRunner(tt.spawn, &fakeIngestor{}, nil)
			scheduler := application.NewScheduler(runner, reporter)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			scheduler.Schedule(ctx, application.PullJob{SourceID: "src-1", Command: "adapter", Interval: time.Hour})

			select {
			case report := <-reporter.reports:
				if report.Outcome != tt.want || report.EmittedTelemetry != tt.emitted {
					t.Fatalf("report = %+v, want outcome=%s emitted=%v", report, tt.want, tt.emitted)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for pull outcome")
			}
			scheduler.Cancel("src-1")
		})
	}
}

func TestScheduler_DoesNotReportOwnerCancellationAsUnreachable(t *testing.T) {
	reporter := &outcomeRecorder{reports: make(chan application.PullOutcomeReport, 1)}
	runner := application.NewRunner(cancellationSpawner{}, &fakeIngestor{}, nil)
	scheduler := application.NewScheduler(runner, reporter)
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Schedule(ctx, application.PullJob{SourceID: "src-1", Command: "adapter", Interval: time.Hour})

	cancel()
	time.Sleep(20 * time.Millisecond)

	select {
	case report := <-reporter.reports:
		t.Fatalf("owner cancellation reported as pull outcome: %+v", report)
	default:
	}
}
