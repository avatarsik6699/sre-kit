// Command beszel-api is a pull-mode adapter (docs/changes/09-beszel-api-adapter.md): it
// authenticates against a configured Beszel instance's PocketBase-backed HTTP API, fetches the
// latest host and per-container stats for one system, and emits NDJSON metric lines on stdout. A
// non-zero exit (connection failure, auth rejected, non-2xx response, unparsable config) is how the
// core's Runner learns to mark the source `unreachable` (docs/SPEC.md §4) — mirrors the other
// adapters' semantics: a successful query that simply has no fresh data yet is not a failure, it
// just emits no metric lines this poll.
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
// already resolved from a secret_ref to the plaintext Beszel account password by the core (see
// internal/platform/secrets.ResolveConfig) — this process never sees a secret_ref.
type config struct {
	BaseURL         string `json:"base_url"`
	SystemID        string `json:"system_id"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	LookbackSeconds int    `json:"lookback_seconds"`
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

// authResponse is PocketBase's auth-with-password success shape.
type authResponse struct {
	Token string `json:"token"`
}

// pocketbaseError is PocketBase's standard error response shape ({status, message, data}).
type pocketbaseError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// recordsResponse is PocketBase's list-records response shape.
type recordsResponse struct {
	Items []statsRecord `json:"items"`
}

// statsRecord is one system_stats or container_stats record.
type statsRecord struct {
	Created string          `json:"created"`
	Stats   json.RawMessage `json:"stats"`
}

// systemStats is system_stats.stats' shape: cpu% (cpu), mem% (mp), disk% (dp), load averages (la).
type systemStats struct {
	CPU float64   `json:"cpu"`
	Mem float64   `json:"mp"`
	Dsk float64   `json:"dp"`
	LA  []float64 `json:"la"`
}

// containerStat is one entry of container_stats.stats' array shape: name (n), cpu% (c), mem MiB (m).
type containerStat struct {
	Name string  `json:"n"`
	CPU  float64 `json:"c"`
	Mem  float64 `json:"m"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("beszel-api: read config: %v", err)
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("beszel-api: parse config: %v", err)
	}
	if cfg.LookbackSeconds <= 0 {
		cfg.LookbackSeconds = 120
	}

	client := &http.Client{Timeout: 10 * time.Second}

	token, err := authenticate(client, cfg)
	if err != nil {
		log.Fatalf("beszel-api: authenticate against %s: %v", cfg.BaseURL, err)
	}

	since := time.Now().Add(-time.Duration(cfg.LookbackSeconds) * time.Second)

	system, err := fetchLatestSystemStats(client, cfg, token)
	if err != nil {
		log.Fatalf("beszel-api: fetch system_stats: %v", err)
	}
	containers, err := fetchLatestContainerStats(client, cfg, token)
	if err != nil {
		log.Fatalf("beszel-api: fetch container_stats: %v", err)
	}

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	encoder := json.NewEncoder(writer)

	for _, line := range toNDJSON(system, containers, since) {
		if err := encoder.Encode(line); err != nil {
			log.Fatalf("beszel-api: encode line: %v", err)
		}
	}
}

// authenticate performs PocketBase's auth-with-password against the "users" collection and returns
// the bearer token. Re-authenticates on every run (stateless pull-mode subprocess, no token caching
// between invocations — same precedent as journal-http's stateless design).
func authenticate(client *http.Client, cfg config) (string, error) {
	body, err := json.Marshal(map[string]string{"identity": cfg.Email, "password": cfg.Password})
	if err != nil {
		return "", fmt.Errorf("encode request body: %w", err)
	}
	u := strings.TrimRight(cfg.BaseURL, "/") + "/api/collections/users/auth-with-password"
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
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, describeError(respBody))
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

// describeError extracts PocketBase's {message} field from an error response body for a more
// useful log line; falls back to the raw body when it doesn't match the expected shape.
func describeError(body []byte) string {
	var pbErr pocketbaseError
	if err := json.Unmarshal(body, &pbErr); err == nil && pbErr.Message != "" {
		return pbErr.Message
	}
	return string(body)
}

// fetchLatestSystemStats fetches the single most recent system_stats record for cfg.SystemID.
func fetchLatestSystemStats(client *http.Client, cfg config, token string) (*statsRecord, error) {
	return fetchLatestRecord(client, cfg, token, "system_stats")
}

// fetchLatestContainerStats fetches the single most recent container_stats record for cfg.SystemID.
func fetchLatestContainerStats(client *http.Client, cfg config, token string) (*statsRecord, error) {
	return fetchLatestRecord(client, cfg, token, "container_stats")
}

// fetchLatestRecord issues one GET /api/collections/{collection}/records request, filtered to
// cfg.SystemID and sorted newest-first, returning the single latest record (nil if the collection
// has no matching records yet — not an error, just "no data this poll").
func fetchLatestRecord(client *http.Client, cfg config, token, collection string) (*statsRecord, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	q := url.Values{}
	q.Set("filter", fmt.Sprintf(`system="%s"`, escapeFilterValue(cfg.SystemID)))
	q.Set("sort", "-created")
	q.Set("perPage", "1")
	reqURL := fmt.Sprintf("%s/api/collections/%s/records?%s", base, collection, q.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, describeError(respBody))
	}

	var records recordsResponse
	if err := json.Unmarshal(respBody, &records); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(records.Items) == 0 {
		return nil, nil
	}
	return &records.Items[0], nil
}

// escapeFilterValue escapes a value for embedding inside a PocketBase filter string literal.
func escapeFilterValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

// toNDJSON converts the latest system/container stats records into wire metric lines. Records
// older than since (the lookback window) are treated as stale and dropped — not an error, just no
// fresh signal this poll, same as a nil record (collection had no matching rows yet).
func toNDJSON(system, containers *statsRecord, since time.Time) []ndjsonLine {
	var lines []ndjsonLine

	if ts, ok := freshTimestamp(system, since); ok {
		var stats systemStats
		if err := json.Unmarshal(system.Stats, &stats); err == nil {
			lines = append(lines,
				ndjsonLine{Type: "metric", SourceID: "beszel-api", Name: "cpu.usage_percent", Timestamp: ts, Value: stats.CPU},
				ndjsonLine{Type: "metric", SourceID: "beszel-api", Name: "mem.usage_percent", Timestamp: ts, Value: stats.Mem},
				ndjsonLine{Type: "metric", SourceID: "beszel-api", Name: "disk.usage_percent", Timestamp: ts, Value: stats.Dsk},
			)
			if len(stats.LA) > 0 {
				lines = append(lines, ndjsonLine{Type: "metric", SourceID: "beszel-api", Name: "load.avg_1m", Timestamp: ts, Value: stats.LA[0]})
			}
		}
	}

	if ts, ok := freshTimestamp(containers, since); ok {
		var entries []containerStat
		if err := json.Unmarshal(containers.Stats, &entries); err == nil {
			for _, c := range entries {
				labels := map[string]string{"container": c.Name}
				lines = append(lines,
					ndjsonLine{Type: "metric", SourceID: "beszel-api", Name: "container.cpu_percent", Timestamp: ts, Value: c.CPU, Labels: labels},
					ndjsonLine{Type: "metric", SourceID: "beszel-api", Name: "container.mem_mib", Timestamp: ts, Value: c.Mem, Labels: labels},
				)
			}
		}
	}

	return lines
}

// freshTimestamp reports whether record is non-nil and its Created timestamp parses and falls at
// or after since, returning that timestamp in RFC3339 form for the NDJSON line.
func freshTimestamp(record *statsRecord, since time.Time) (string, bool) {
	if record == nil {
		return "", false
	}
	// PocketBase's default Created format: "2006-01-02 15:04:05.000Z".
	ts, err := time.Parse("2006-01-02 15:04:05.000Z", record.Created)
	if err != nil {
		return "", false
	}
	if ts.Before(since) {
		return "", false
	}
	return ts.UTC().Format(time.RFC3339), true
}
