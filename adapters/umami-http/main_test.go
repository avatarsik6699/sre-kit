package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestToNDJSON(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	stats := statsResponse{Pageviews: 120, Visitors: 45, Visits: 60, Bounces: 20, Totaltime: 3600}

	lines := toNDJSON(stats, ts)
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5: %+v", len(lines), lines)
	}

	want := map[string]float64{
		"analytics.pageviews":         120,
		"analytics.visitors":          45,
		"analytics.visits":            60,
		"analytics.bounces":           20,
		"analytics.totaltime_seconds": 3600,
	}
	wantTS := "2024-01-15T10:00:00Z"
	for _, line := range lines {
		if line.Type != "metric" || line.SourceID != "umami-http" {
			t.Fatalf("unexpected envelope: %+v", line)
		}
		if line.Timestamp != wantTS {
			t.Fatalf("got timestamp %q, want %q", line.Timestamp, wantTS)
		}
		wantValue, ok := want[line.Name]
		if !ok {
			t.Fatalf("unexpected metric name %q", line.Name)
		}
		if line.Value != wantValue {
			t.Fatalf("metric %q: got %v, want %v", line.Name, line.Value, wantValue)
		}
		delete(want, line.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing expected metrics: %+v", want)
	}
}

func TestToNDJSON_AllZeroTrafficIsNotAnError(t *testing.T) {
	lines := toNDJSON(statsResponse{}, time.Now())
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
	for _, line := range lines {
		if line.Value != 0 {
			t.Fatalf("expected all-zero values, got %+v", line)
		}
	}
}

// fakeUmami is a minimal in-process stand-in for Umami's REST API, exercising the
// /api/auth/login + /api/websites/{id}/stats endpoints this adapter depends on, mirroring
// beszel-api's httptest.Server technique (I3) rather than requiring a real Umami instance.
type fakeUmami struct {
	wantUsername, wantPassword string
	wantWebsiteID              string
	token                      string
	statsBody                  string
	rejectAuth                 bool
}

func (f *fakeUmami) handler() http.HandlerFunc {
	statsPath := fmt.Sprintf("/api/websites/%s/stats", f.wantWebsiteID)
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth/login":
			var body struct{ Username, Password string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if f.rejectAuth || body.Username != f.wantUsername || body.Password != f.wantPassword {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"message":"Invalid credentials"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(authResponse{Token: f.token})
		case r.URL.Path == statsPath:
			if r.Header.Get("Authorization") != "Bearer "+f.token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, f.statsBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestAuthenticate_Success(t *testing.T) {
	fake := &fakeUmami{wantUsername: "admin", wantPassword: "s3cret", token: "tok-123"}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	token, err := authenticate(&http.Client{}, config{BaseURL: server.URL, Username: "admin", Password: "s3cret"})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if token != "tok-123" {
		t.Fatalf("got token %q", token)
	}
}

func TestAuthenticate_RejectedCredentials(t *testing.T) {
	fake := &fakeUmami{wantUsername: "admin", wantPassword: "s3cret", rejectAuth: true}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	if _, err := authenticate(&http.Client{}, config{BaseURL: server.URL, Username: "admin", Password: "wrong"}); err == nil {
		t.Fatal("expected an error for rejected credentials")
	}
}

func TestFetchStats_Success(t *testing.T) {
	fake := &fakeUmami{
		wantUsername: "admin", wantPassword: "s3cret", wantWebsiteID: "site1", token: "tok",
		statsBody: `{"pageviews":120,"visitors":45,"visits":60,"bounces":20,"totaltime":3600}`,
	}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	since := time.Now().Add(-time.Hour)
	until := time.Now()
	stats, err := fetchStats(&http.Client{}, config{BaseURL: server.URL, WebsiteID: "site1"}, "tok", since, until)
	if err != nil {
		t.Fatalf("fetchStats: %v", err)
	}
	if stats.Pageviews != 120 || stats.Visitors != 45 {
		t.Fatalf("got %+v", stats)
	}
}

func TestFetchStats_QueryParams(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		fmt.Fprint(w, `{"pageviews":0,"visitors":0,"visits":0,"bounces":0,"totaltime":0}`)
	}))
	defer server.Close()

	since := time.UnixMilli(1000)
	until := time.UnixMilli(2000)
	if _, err := fetchStats(&http.Client{}, config{BaseURL: server.URL, WebsiteID: "site1"}, "tok", since, until); err != nil {
		t.Fatalf("fetchStats: %v", err)
	}
	if !strings.Contains(gotQuery, "startAt=1000") || !strings.Contains(gotQuery, "endAt=2000") {
		t.Fatalf("got query %q, want startAt=1000 and endAt=2000", gotQuery)
	}
}

func TestFetchStats_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := fetchStats(&http.Client{}, config{BaseURL: server.URL, WebsiteID: "site1"}, "tok", time.Now(), time.Now()); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestFetchEventCount_Success(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"data":[],"count":7,"page":1}`)
	}))
	defer server.Close()

	count, err := fetchEventCount(&http.Client{}, config{BaseURL: server.URL, WebsiteID: "site1"}, "tok", "signup", time.UnixMilli(1000), time.UnixMilli(2000))
	if err != nil {
		t.Fatalf("fetchEventCount: %v", err)
	}
	if count != 7 {
		t.Fatalf("count = %v, want 7", count)
	}
	if !strings.Contains(gotQuery, "event=signup") {
		t.Fatalf("got query %q, want event=signup", gotQuery)
	}
}

func TestFetchEventCount_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := fetchEventCount(&http.Client{}, config{BaseURL: server.URL, WebsiteID: "site1"}, "tok", "signup", time.Now(), time.Now()); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestEventCountNDJSON(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	line := eventCountNDJSON("signup", 7, ts)
	if line.Type != "metric" || line.SourceID != "umami-http" || line.Name != "analytics.event_count" {
		t.Fatalf("unexpected envelope: %+v", line)
	}
	if line.Value != 7 {
		t.Fatalf("value = %v, want 7", line.Value)
	}
	if line.Labels["event"] != "signup" {
		t.Fatalf("labels = %+v, want event=signup", line.Labels)
	}
}

func TestFetchStats_AllZeroTrafficIsNotAnError(t *testing.T) {
	fake := &fakeUmami{
		wantUsername: "admin", wantPassword: "s3cret", wantWebsiteID: "site1", token: "tok",
		statsBody: `{"pageviews":0,"visitors":0,"visits":0,"bounces":0,"totaltime":0}`,
	}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	stats, err := fetchStats(&http.Client{}, config{BaseURL: server.URL, WebsiteID: "site1"}, "tok", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("fetchStats: %v", err)
	}
	if stats != (statsResponse{}) {
		t.Fatalf("got %+v, want zero value", stats)
	}
}
