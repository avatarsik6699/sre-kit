// Command umami-http is a pull-mode adapter (docs/changes/11-umami-http-adapter.md): it
// authenticates against a configured Umami instance's REST API, fetches aggregate traffic stats
// for one website over the poll's lookback window, and emits NDJSON metric lines on stdout. A
// non-zero exit (connection failure, login rejected, non-2xx response, unparsable config) is how
// the core's Runner learns to mark the source `unreachable` (docs/SPEC.md §4) — a successful poll
// with all-zero traffic counts (no visitors in the window) is not a failure, it just emits metric
// lines with value 0.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// config is the adapter's stdin payload: the source's config_json with its "password" field
// already resolved from a secret_ref to the plaintext Umami account password by the core (see
// internal/platform/secrets.ResolveConfig) — this process never sees a secret_ref.
type config struct {
	BaseURL         string   `json:"base_url"`
	WebsiteID       string   `json:"website_id"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	LookbackSeconds int      `json:"lookback_seconds"`
	TrackedEvents   []string `json:"tracked_events"`
}

// ndjsonLine is deliberately independent of internal/contract's types — an adapter is any
// language, any process, talking only NDJSON-over-stdio, so it can't import core Go packages.
type ndjsonLine struct {
	Type      string            `json:"type"`
	SourceID  string            `json:"source_id"`
	Name      string            `json:"name"`
	Timestamp string            `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// authResponse is Umami's /api/auth/login success shape.
type authResponse struct {
	Token string `json:"token"`
}

// statsResponse is Umami's /api/websites/{id}/stats success shape — plain integers, not
// {value: ...}-wrapped fields (verified against Umami's current v3 API docs).
type statsResponse struct {
	Pageviews float64 `json:"pageviews"`
	Visitors  float64 `json:"visitors"`
	Visits    float64 `json:"visits"`
	Bounces   float64 `json:"bounces"`
	Totaltime float64 `json:"totaltime"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("umami-http: read config: %v", err)
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("umami-http: parse config: %v", err)
	}
	if cfg.LookbackSeconds <= 0 {
		cfg.LookbackSeconds = 3600
	}

	client := &http.Client{Timeout: 10 * time.Second}

	token, err := authenticate(client, cfg)
	if err != nil {
		log.Fatalf("umami-http: authenticate against %s: %v", cfg.BaseURL, err)
	}

	now := time.Now().UTC()
	since := now.Add(-time.Duration(cfg.LookbackSeconds) * time.Second)
	stats, err := fetchStats(client, cfg, token, since, now)
	if err != nil {
		log.Fatalf("umami-http: fetch stats: %v", err)
	}

	lines := toNDJSON(stats, now)
	for _, eventName := range cfg.TrackedEvents {
		count, err := fetchEventCount(client, cfg, token, eventName, since, now)
		if err != nil {
			log.Fatalf("umami-http: fetch event count for %q: %v", eventName, err)
		}
		lines = append(lines, eventCountNDJSON(eventName, count, now))
	}

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	encoder := json.NewEncoder(writer)
	for _, line := range lines {
		if err := encoder.Encode(line); err != nil {
			log.Fatalf("umami-http: encode line: %v", err)
		}
	}
}

// authenticate performs Umami's /api/auth/login and returns the bearer token. Re-authenticates on
// every run (stateless pull-mode subprocess, no token caching between invocations — same precedent
// as journal-http/beszel-api's stateless design).
func authenticate(client *http.Client, cfg config) (string, error) {
	body, err := json.Marshal(map[string]string{"username": cfg.Username, "password": cfg.Password})
	if err != nil {
		return "", fmt.Errorf("encode request body: %w", err)
	}
	u := strings.TrimRight(cfg.BaseURL, "/") + "/api/auth/login"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var auth authResponse
	if err := json.Unmarshal(respBody, &auth); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if auth.Token == "" {
		return "", fmt.Errorf("response had no token")
	}
	return auth.Token, nil
}

// fetchStats issues one GET /api/websites/{id}/stats request for the [since, until] window.
func fetchStats(client *http.Client, cfg config, token string, since, until time.Time) (statsResponse, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	q := url.Values{}
	q.Set("startAt", fmt.Sprintf("%d", since.UnixMilli()))
	q.Set("endAt", fmt.Sprintf("%d", until.UnixMilli()))
	q.Set("unit", "hour")
	q.Set("timezone", "UTC")
	reqURL := fmt.Sprintf("%s/api/websites/%s/stats?%s", base, url.PathEscape(cfg.WebsiteID), q.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return statsResponse{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return statsResponse{}, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return statsResponse{}, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statsResponse{}, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var stats statsResponse
	if err := json.Unmarshal(respBody, &stats); err != nil {
		return statsResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return stats, nil
}

// eventsResponse is Umami's GET /api/websites/{id}/events success shape — `count` is the total
// number of matching events for the filtered window, regardless of pageSize (verified against
// Umami's current v3 API docs).
type eventsResponse struct {
	Count float64 `json:"count"`
}

// fetchEventCount issues one GET /api/websites/{id}/events request filtered to eventName, returning
// the total count of matching events in [since, until]. Generic by design: sre-kit has no notion of
// which event names matter — the caller (a source's tracked_events config) decides that per
// deployment, per docs/SPEC.md's adapter-config-is-user-owned convention.
func fetchEventCount(client *http.Client, cfg config, token, eventName string, since, until time.Time) (float64, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	q := url.Values{}
	q.Set("startAt", fmt.Sprintf("%d", since.UnixMilli()))
	q.Set("endAt", fmt.Sprintf("%d", until.UnixMilli()))
	q.Set("event", eventName)
	q.Set("pageSize", "1")
	reqURL := fmt.Sprintf("%s/api/websites/%s/events?%s", base, url.PathEscape(cfg.WebsiteID), q.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var events eventsResponse
	if err := json.Unmarshal(respBody, &events); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return events.Count, nil
}

// eventCountNDJSON converts one tracked event's count into a wire metric line, labeled by event
// name so an arbitrary number of tracked events all share one metric name
// ("analytics.event_count") distinguished by label, matching docs/SPEC.md §4's Metric shape.
func eventCountNDJSON(eventName string, count float64, ts time.Time) ndjsonLine {
	return ndjsonLine{
		Type:      "metric",
		SourceID:  "umami-http",
		Name:      "analytics.event_count",
		Timestamp: ts.Format(time.RFC3339),
		Value:     count,
		Labels:    map[string]string{"event": eventName},
	}
}

// toNDJSON converts stats into wire metric lines, all stamped at ts (the poll time) — a snapshot
// of the lookback window's totals, not a historical backfill (see main.go's doc comment on why
// this adapter doesn't call Umami's /pageviews time-series endpoint).
func toNDJSON(stats statsResponse, ts time.Time) []ndjsonLine {
	timestamp := ts.Format(time.RFC3339)
	return []ndjsonLine{
		{Type: "metric", SourceID: "umami-http", Name: "analytics.pageviews", Timestamp: timestamp, Value: stats.Pageviews},
		{Type: "metric", SourceID: "umami-http", Name: "analytics.visitors", Timestamp: timestamp, Value: stats.Visitors},
		{Type: "metric", SourceID: "umami-http", Name: "analytics.visits", Timestamp: timestamp, Value: stats.Visits},
		{Type: "metric", SourceID: "umami-http", Name: "analytics.bounces", Timestamp: timestamp, Value: stats.Bounces},
		{Type: "metric", SourceID: "umami-http", Name: "analytics.totaltime_seconds", Timestamp: timestamp, Value: stats.Totaltime},
	}
}
