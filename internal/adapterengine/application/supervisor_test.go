package application_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sre-kit/internal/adapterengine/application"
)

// fakeStreamSource simulates a long-lived stream-mode adapter: Scan blocks until a line arrives on
// linesCh or doneCh is closed (simulating process exit).
type fakeStreamSource struct {
	linesCh chan string
	doneCh  chan struct{}
	text    string
	waitErr error
}

func newFakeStreamSource() *fakeStreamSource {
	return &fakeStreamSource{linesCh: make(chan string), doneCh: make(chan struct{})}
}

func (f *fakeStreamSource) Scan() bool {
	select {
	case line, ok := <-f.linesCh:
		if !ok {
			return false
		}
		f.text = line
		return true
	case <-f.doneCh:
		return false
	}
}
func (f *fakeStreamSource) Text() string { return f.text }
func (f *fakeStreamSource) Err() error   { return nil }
func (f *fakeStreamSource) Wait() error  { return f.waitErr }

type streamSpawner struct {
	calls int32

	mu      sync.Mutex
	sources []*fakeStreamSource
}

func (s *streamSpawner) Spawn(context.Context, string, []string, []byte) (application.LineSource, error) {
	atomic.AddInt32(&s.calls, 1)
	source := newFakeStreamSource()
	s.mu.Lock()
	s.sources = append(s.sources, source)
	s.mu.Unlock()
	return source, nil
}

func (s *streamSpawner) spawnedSources() []*fakeStreamSource {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*fakeStreamSource(nil), s.sources...)
}

func TestSupervisor_IngestsValidStreamLines(t *testing.T) {
	spawner := &streamSpawner{}
	ingestor := &fakeIngestor{}
	supervisor := application.NewSupervisor(spawner, ingestor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	supervisor.Supervise(ctx, application.StreamJob{SourceID: "src-1", Command: "stub", HeartbeatInterval: 500 * time.Millisecond})
	time.Sleep(10 * time.Millisecond) // let Spawn happen

	sources := spawner.spawnedSources()
	if len(sources) != 1 {
		t.Fatalf("got %d spawns, want 1", len(sources))
	}
	sources[0].linesCh <- `{"type":"metric","source_id":"src-1","name":"cpu.usage_percent","timestamp":"2026-08-13T12:00:00Z","value":1}`
	sources[0].linesCh <- `{"type":"event","source_id":"src-1","timestamp":"2026-08-13T12:00:00Z","level":"info","message":"tick"}`
	time.Sleep(20 * time.Millisecond)

	supervisor.Stop("src-1")

	metrics, _, events := ingestor.counts()
	if metrics != 1 || events != 1 {
		t.Fatalf("got %d metrics, %d events, want 1 of each", metrics, events)
	}
}

func TestSupervisor_StopHaltsFurtherSpawns(t *testing.T) {
	spawner := &streamSpawner{}
	supervisor := application.NewSupervisor(spawner, &fakeIngestor{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	supervisor.Supervise(ctx, application.StreamJob{SourceID: "src-1", Command: "stub", HeartbeatInterval: time.Second})
	time.Sleep(10 * time.Millisecond)
	supervisor.Stop("src-1")

	afterStop := atomic.LoadInt32(&spawner.calls)
	time.Sleep(30 * time.Millisecond)
	if got := atomic.LoadInt32(&spawner.calls); got != afterStop {
		t.Fatalf("spawns after Stop: got %d, want unchanged at %d", got, afterStop)
	}
}

func TestSupervisor_MissedHeartbeatTriggersRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ~1s backoff-timing test in short mode")
	}
	spawner := &streamSpawner{}
	supervisor := application.NewSupervisor(spawner, &fakeIngestor{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Never send a line on any spawned source's linesCh, so every attempt times out on its
	// 20ms heartbeat window; backoff before the first retry is ~1s (application.go's
	// supervisorInitialBackoff), so wait a bit past that for the second spawn.
	supervisor.Supervise(ctx, application.StreamJob{SourceID: "src-1", Command: "stub", HeartbeatInterval: 20 * time.Millisecond})
	time.Sleep(1200 * time.Millisecond)
	supervisor.Stop("src-1")

	if got := atomic.LoadInt32(&spawner.calls); got < 2 {
		t.Fatalf("got %d spawns, want at least 2 (initial + restart after missed heartbeat)", got)
	}
}
