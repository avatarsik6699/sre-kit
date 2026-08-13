// Command stub is a fixed-output pull-mode test adapter: it reads (and ignores) its config on
// stdin, emits a handful of NDJSON metric lines on stdout, and exits 0. It exists to exercise the
// full pull-mode pipeline end-to-end (docs/SPEC.md M1: "a stub adapter's test data is visible via
// /api/metrics") without needing a real SSH-connected host.
package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"time"
)

// ndjsonLine is deliberately independent of internal/contract's types — an adapter is any
// language, any process, talking only NDJSON-over-stdio, so it can't import core Go packages.
type ndjsonLine struct {
	Type      string  `json:"type"`
	SourceID  string  `json:"source_id"`
	Name      string  `json:"name"`
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

func main() {
	// Drain stdin (the core writes the source's config JSON here) — the stub doesn't need it, but
	// draining keeps the core's Spawn/Wait sequencing simple (no adapter left blocked on stdin).
	_, _ = io.Copy(io.Discard, os.Stdin)

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	now := time.Now().UTC().Format(time.RFC3339)
	fixtures := []ndjsonLine{
		{Type: "metric", SourceID: "stub", Name: "cpu.usage_percent", Timestamp: now, Value: 12.5},
		{Type: "metric", SourceID: "stub", Name: "mem.usage_percent", Timestamp: now, Value: 41.2},
		{Type: "metric", SourceID: "stub", Name: "disk.usage_percent", Timestamp: now, Value: 63.8},
	}
	encoder := json.NewEncoder(writer)
	for _, line := range fixtures {
		if err := encoder.Encode(line); err != nil {
			log.Fatalf("stub: encode line: %v", err)
		}
	}
}
