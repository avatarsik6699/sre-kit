package application_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"sre-kit/internal/adapterengine/application"
)

type fakeLineSource struct {
	lines   []string
	idx     int
	waitErr error
}

func (f *fakeLineSource) Scan() bool {
	if f.idx >= len(f.lines) {
		return false
	}
	f.idx++
	return true
}
func (f *fakeLineSource) Text() string { return f.lines[f.idx-1] }
func (f *fakeLineSource) Err() error   { return nil }
func (f *fakeLineSource) Wait() error  { return f.waitErr }

type fakeSpawner struct {
	source *fakeLineSource
	err    error
}

func (f *fakeSpawner) Spawn(context.Context, string, []string, []byte) (application.LineSource, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.source, nil
}

// fakeIngestor is shared by runner and supervisor tests; Supervisor ingests on a background
// goroutine, so counts are mutex-guarded even though Runner's tests only ever touch it
// synchronously.
type fakeIngestor struct {
	mu      sync.Mutex
	metrics int
	checks  int
	events  int
}

func (f *fakeIngestor) IngestMetric(context.Context, string, string, time.Time, float64, map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metrics++
	return nil
}
func (f *fakeIngestor) IngestCheck(context.Context, string, string, time.Time, string, map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	return nil
}
func (f *fakeIngestor) IngestEvent(context.Context, string, time.Time, string, string, map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events++
	return nil
}

func (f *fakeIngestor) counts() (metrics, checks, events int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.metrics, f.checks, f.events
}

func TestRunOnce_IngestsValidLinesByType(t *testing.T) {
	source := &fakeLineSource{lines: []string{
		`{"type":"metric","source_id":"src-1","name":"cpu.usage_percent","timestamp":"2026-08-13T12:00:00Z","value":42.5}`,
		`{"type":"check","source_id":"src-1","name":"tls-expiry","timestamp":"2026-08-13T12:00:00Z","status":"ok"}`,
		`{"type":"event","source_id":"src-1","timestamp":"2026-08-13T12:00:00Z","level":"info","message":"tick"}`,
	}}
	ingestor := &fakeIngestor{}
	runner := application.NewRunner(&fakeSpawner{source: source}, ingestor, nil)

	result, err := runner.RunOnce(context.Background(), "src-1", "stub", nil, []byte("{}"))
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.LinesProcessed != 3 || result.InvalidLines != 0 {
		t.Fatalf("result = %+v, want 3 processed, 0 invalid", result)
	}
	if ingestor.metrics != 1 || ingestor.checks != 1 || ingestor.events != 1 {
		t.Fatalf("ingestor calls = %+v, want 1 of each", ingestor)
	}
}

func TestRunOnce_SkipsBlankLines(t *testing.T) {
	source := &fakeLineSource{lines: []string{
		"",
		`{"type":"metric","source_id":"src-1","name":"cpu.usage_percent","timestamp":"2026-08-13T12:00:00Z","value":1}`,
		"   ",
	}}
	ingestor := &fakeIngestor{}
	runner := application.NewRunner(&fakeSpawner{source: source}, ingestor, nil)

	result, err := runner.RunOnce(context.Background(), "src-1", "stub", nil, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.LinesProcessed != 1 {
		t.Fatalf("LinesProcessed = %d, want 1", result.LinesProcessed)
	}
}

func TestRunOnce_AutoDisablesAfterTenConsecutiveInvalidLines(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = `{"type":"bogus"}`
	}
	source := &fakeLineSource{lines: lines}
	ingestor := &fakeIngestor{}
	disabledID := ""
	disable := func(_ context.Context, sourceID string) error {
		disabledID = sourceID
		return nil
	}
	runner := application.NewRunner(&fakeSpawner{source: source}, ingestor, disable)

	result, err := runner.RunOnce(context.Background(), "src-1", "stub", nil, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !result.AutoDisabled {
		t.Fatal("result.AutoDisabled = false, want true after 10 consecutive invalid lines")
	}
	if disabledID != "src-1" {
		t.Fatalf("disable called with %q, want %q", disabledID, "src-1")
	}
}

func TestRunOnce_InvalidStreakResetsOnAValidLine(t *testing.T) {
	lines := []string{
		`{"type":"bogus"}`, `{"type":"bogus"}`, `{"type":"bogus"}`,
		`{"type":"metric","source_id":"src-1","name":"cpu.usage_percent","timestamp":"2026-08-13T12:00:00Z","value":1}`,
		`{"type":"bogus"}`, `{"type":"bogus"}`,
	}
	source := &fakeLineSource{lines: lines}
	ingestor := &fakeIngestor{}
	runner := application.NewRunner(&fakeSpawner{source: source}, ingestor, nil)

	result, err := runner.RunOnce(context.Background(), "src-1", "stub", nil, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.AutoDisabled {
		t.Fatal("result.AutoDisabled = true, want false — a valid line should reset the streak")
	}
	if result.InvalidLines != 5 {
		t.Fatalf("InvalidLines = %d, want 5", result.InvalidLines)
	}
}

func TestRunOnce_SpawnErrorPropagates(t *testing.T) {
	spawnErr := errors.New("boom")
	runner := application.NewRunner(&fakeSpawner{err: spawnErr}, &fakeIngestor{}, nil)

	_, err := runner.RunOnce(context.Background(), "src-1", "stub", nil, nil)
	if err == nil {
		t.Fatal("RunOnce with a spawn error: want error, got nil")
	}
}

func TestRunOnce_SubprocessExitErrorPropagates(t *testing.T) {
	source := &fakeLineSource{waitErr: errors.New("credential=must-not-be-logged")}
	runner := application.NewRunner(&fakeSpawner{source: source}, &fakeIngestor{}, nil)

	_, err := runner.RunOnce(context.Background(), "src-1", "stub", nil, nil)
	if err == nil {
		t.Fatal("RunOnce with a subprocess exit error: want error, got nil")
	}
	if got := application.PullFailureClassOf(err); got != application.PullFailureSubprocess {
		t.Fatalf("failure class = %q, want %q", got, application.PullFailureSubprocess)
	}
	if strings.Contains(err.Error(), "credential") {
		t.Fatalf("error leaked raw subprocess stderr: %q", err)
	}
}
