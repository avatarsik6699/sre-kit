package main

import (
	"context"
	"testing"

	adapterengineapp "sre-kit/internal/adapterengine/application"
)

func TestSourceOutcomeReporterMapsPullSemantics(t *testing.T) {
	tests := []struct {
		name       string
		report     adapterengineapp.PullOutcomeReport
		wantMarked []string
		wantAlerts []string
	}{
		{
			name: "quiet success marks source and resolves connectivity",
			report: adapterengineapp.PullOutcomeReport{
				Outcome: adapterengineapp.PullOutcomeOK,
			},
			wantMarked: []string{"ok"},
			wantAlerts: []string{"ok"},
		},
		{
			name: "emitting success preserves telemetry-derived source status",
			report: adapterengineapp.PullOutcomeReport{
				Outcome:          adapterengineapp.PullOutcomeOK,
				EmittedTelemetry: true,
			},
			wantAlerts: []string{"ok"},
		},
		{
			name: "failure marks unreachable and participates in debounce",
			report: adapterengineapp.PullOutcomeReport{
				Outcome: adapterengineapp.PullOutcomeUnreachable,
			},
			wantMarked: []string{"unreachable"},
			wantAlerts: []string{"unreachable"},
		},
		{
			name: "invalid output marks immediate error",
			report: adapterengineapp.PullOutcomeReport{
				Outcome: adapterengineapp.PullOutcomeError,
			},
			wantMarked: []string{"error"},
			wantAlerts: []string{"error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var marked []string
			var alerts []string
			reporter := sourceOutcomeReporter{
				markSeen: func(_ context.Context, _ string, status string) error {
					marked = append(marked, status)
					return nil
				},
				evaluateSourceStatus: func(_ context.Context, _ string, status string) error {
					alerts = append(alerts, status)
					return nil
				},
			}

			reporter.ReportPullOutcome(context.Background(), "src-1", tt.report)

			assertStatuses(t, "markSeen", marked, tt.wantMarked)
			assertStatuses(t, "evaluateSourceStatus", alerts, tt.wantAlerts)
		})
	}
}

func assertStatuses(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s calls = %v, want %v", name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s calls = %v, want %v", name, got, want)
		}
	}
}
