package application_test

import (
	"context"
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
