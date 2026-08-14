package domain_test

import (
	"testing"

	"sre-kit/internal/alertrouter/domain"
)

func TestEvaluateMetricCondition(t *testing.T) {
	cases := []struct {
		name      string
		condition string
		threshold string
		value     float64
		want      bool
		wantErr   bool
	}{
		{"greater-than true", ">", "90", 95, true, false},
		{"greater-than false", ">", "90", 50, false, false},
		{"less-than true", "<", "10", 5, true, false},
		{"equals true", "=", "42", 42, true, false},
		{"bad threshold", ">", "not-a-number", 1, false, true},
		{"status_is unsupported", "status_is", "ok", 1, false, true},
		{"unknown condition", "~", "1", 1, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.EvaluateMetricCondition(tc.condition, tc.threshold, tc.value)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("got = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEvaluateCheckCondition(t *testing.T) {
	cases := []struct {
		name      string
		condition string
		threshold string
		status    string
		want      bool
		wantErr   bool
	}{
		{"status_is match", "status_is", "critical", "critical", true, false},
		{"status_is no match", "status_is", "critical", "ok", false, false},
		{"comparison unsupported", ">", "1", "ok", false, true},
		{"unknown condition", "~", "ok", "ok", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.EvaluateCheckCondition(tc.condition, tc.threshold, tc.status)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("got = %v, want %v", got, tc.want)
			}
		})
	}
}
