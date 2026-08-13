package infrastructure_test

import (
	"context"
	"testing"

	"sre-kit/internal/adapterengine/infrastructure"
)

func TestProcessSpawner_SpawnReadsStdoutLines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-subprocess test in short mode")
	}

	spawner := infrastructure.NewProcessSpawner()
	source, err := spawner.Spawn(context.Background(), "sh", []string{"-c", "cat; echo done >&2"}, []byte("line one\nline two\n"))
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	var got []string
	for source.Scan() {
		got = append(got, source.Text())
	}
	if err := source.Err(); err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if err := source.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if len(got) != 2 || got[0] != "line one" || got[1] != "line two" {
		t.Fatalf("got %v, want [line one, line two]", got)
	}
}

func TestProcessSpawner_WaitReturnsErrorOnNonZeroExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-subprocess test in short mode")
	}

	spawner := infrastructure.NewProcessSpawner()
	source, err := spawner.Spawn(context.Background(), "sh", []string{"-c", "echo oops >&2; exit 1"}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for source.Scan() {
	}
	if err := source.Wait(); err == nil {
		t.Fatal("Wait after non-zero exit: want error, got nil")
	}
}
