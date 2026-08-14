package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEscapeFilterValue(t *testing.T) {
	got := escapeFilterValue(`sys"1\2`)
	want := `sys\"1\\2`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFreshTimestamp(t *testing.T) {
	since := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	t.Run("nil record", func(t *testing.T) {
		if _, ok := freshTimestamp(nil, since); ok {
			t.Fatal("expected ok=false for a nil record")
		}
	})

	t.Run("fresh record", func(t *testing.T) {
		record := &statsRecord{Created: "2024-01-15 10:05:00.123Z"}
		ts, ok := freshTimestamp(record, since)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if ts != "2024-01-15T10:05:00Z" {
			t.Fatalf("got %q", ts)
		}
	})

	t.Run("stale record", func(t *testing.T) {
		record := &statsRecord{Created: "2024-01-15 09:00:00.000Z"}
		if _, ok := freshTimestamp(record, since); ok {
			t.Fatal("expected ok=false for a stale record")
		}
	})

	t.Run("unparsable timestamp", func(t *testing.T) {
		record := &statsRecord{Created: "not a timestamp"}
		if _, ok := freshTimestamp(record, since); ok {
			t.Fatal("expected ok=false for an unparsable timestamp")
		}
	})
}

func TestToNDJSON(t *testing.T) {
	since := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	system := &statsRecord{
		Created: "2024-01-15 10:05:00.000Z",
		Stats:   json.RawMessage(`{"cpu":12.5,"mp":40.2,"dp":55.0,"la":[0.8,0.5,0.3]}`),
	}
	containers := &statsRecord{
		Created: "2024-01-15 10:05:00.000Z",
		Stats:   json.RawMessage(`[{"n":"web","c":3.1,"m":128.5},{"n":"db","c":9.4,"m":512.0}]`),
	}

	lines := toNDJSON(system, containers, since)

	wantNames := map[string]bool{
		"cpu.usage_percent": false, "mem.usage_percent": false, "disk.usage_percent": false, "load.avg_1m": false,
	}
	containerLines := 0
	for _, line := range lines {
		if _, ok := wantNames[line.Name]; ok {
			wantNames[line.Name] = true
			continue
		}
		if line.Name == "container.cpu_percent" || line.Name == "container.mem_mib" {
			containerLines++
			if line.Labels["container"] == "" {
				t.Fatalf("container metric missing container label: %+v", line)
			}
		}
	}
	for name, seen := range wantNames {
		if !seen {
			t.Fatalf("missing expected host metric %q in %+v", name, lines)
		}
	}
	if containerLines != 4 { // 2 containers * 2 metrics each
		t.Fatalf("got %d container metric lines, want 4", containerLines)
	}
}

func TestToNDJSON_StaleRecordsProduceNothing(t *testing.T) {
	since := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	stale := &statsRecord{Created: "2024-01-15 09:00:00.000Z", Stats: json.RawMessage(`{"cpu":1}`)}

	lines := toNDJSON(stale, stale, since)
	if len(lines) != 0 {
		t.Fatalf("got %d lines, want 0: %+v", len(lines), lines)
	}
}

func TestToNDJSON_NilRecordsProduceNothing(t *testing.T) {
	lines := toNDJSON(nil, nil, time.Now())
	if len(lines) != 0 {
		t.Fatalf("got %d lines, want 0: %+v", len(lines), lines)
	}
}

// fakePocketBase is a minimal in-process stand-in for Beszel's PocketBase API, exercising the
// auth-with-password + records list endpoints this adapter depends on, mirroring journal-http's
// httptest.Server technique (I3) rather than requiring a real Beszel instance.
type fakePocketBase struct {
	wantEmail, wantPassword string
	wantAuthCollection      string // defaults to "_superusers" if empty, see handler()
	token                   string
	systemStatsBody         string
	containerStatsBody      string
	rejectAuth              bool
}

func (f *fakePocketBase) handler() http.HandlerFunc {
	wantCollection := f.wantAuthCollection
	if wantCollection == "" {
		wantCollection = "_superusers"
	}
	authPath := fmt.Sprintf("/api/collections/%s/auth-with-password", wantCollection)
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == authPath:
			var body struct{ Identity, Password string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if f.rejectAuth || body.Identity != f.wantEmail || body.Password != f.wantPassword {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(pocketbaseError{Status: 400, Message: "Failed to authenticate."})
				return
			}
			_ = json.NewEncoder(w).Encode(authResponse{Token: f.token})
		case r.URL.Path == "/api/collections/system_stats/records":
			if r.Header.Get("Authorization") != f.token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, f.systemStatsBody)
		case r.URL.Path == "/api/collections/container_stats/records":
			if r.Header.Get("Authorization") != f.token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, f.containerStatsBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestAuthenticate_Success(t *testing.T) {
	fake := &fakePocketBase{wantEmail: "admin@example.com", wantPassword: "s3cret", token: "tok-123"}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	token, err := authenticate(&http.Client{}, config{BaseURL: server.URL, Email: "admin@example.com", Password: "s3cret", AuthCollection: "_superusers"})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if token != "tok-123" {
		t.Fatalf("got token %q", token)
	}
}

func TestAuthenticate_RejectedCredentials(t *testing.T) {
	fake := &fakePocketBase{wantEmail: "admin@example.com", wantPassword: "s3cret", rejectAuth: true}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	if _, err := authenticate(&http.Client{}, config{BaseURL: server.URL, Email: "admin@example.com", Password: "wrong", AuthCollection: "_superusers"}); err == nil {
		t.Fatal("expected an error for rejected credentials")
	}
}

func TestAuthenticate_NonDefaultCollection(t *testing.T) {
	fake := &fakePocketBase{wantEmail: "viewer@example.com", wantPassword: "s3cret", wantAuthCollection: "users", token: "tok-456"}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	token, err := authenticate(&http.Client{}, config{BaseURL: server.URL, Email: "viewer@example.com", Password: "s3cret", AuthCollection: "users"})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if token != "tok-456" {
		t.Fatalf("got token %q", token)
	}
}

func TestFetchLatestRecord_EmptyResultIsNotAnError(t *testing.T) {
	fake := &fakePocketBase{
		wantEmail: "a@b.com", wantPassword: "p", token: "tok",
		systemStatsBody: `{"items":[]}`,
	}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	record, err := fetchLatestSystemStats(&http.Client{}, config{BaseURL: server.URL, SystemID: "sys1"}, "tok")
	if err != nil {
		t.Fatalf("fetchLatestSystemStats: %v", err)
	}
	if record != nil {
		t.Fatalf("got %+v, want nil for an empty items list", record)
	}
}

func TestFetchLatestRecord_Success(t *testing.T) {
	fake := &fakePocketBase{
		wantEmail: "a@b.com", wantPassword: "p", token: "tok",
		systemStatsBody: `{"items":[{"created":"2024-01-15 10:05:00.000Z","stats":{"cpu":12.5,"mp":40.2,"dp":55.0,"la":[0.8]}}]}`,
	}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	record, err := fetchLatestSystemStats(&http.Client{}, config{BaseURL: server.URL, SystemID: "sys1"}, "tok")
	if err != nil {
		t.Fatalf("fetchLatestSystemStats: %v", err)
	}
	if record == nil {
		t.Fatal("expected a non-nil record")
	}
	var stats systemStats
	if err := json.Unmarshal(record.Stats, &stats); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
	if stats.CPU != 12.5 {
		t.Fatalf("got cpu %v", stats.CPU)
	}
}

func TestFetchLatestRecord_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := fetchLatestSystemStats(&http.Client{}, config{BaseURL: server.URL, SystemID: "sys1"}, "tok"); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}
