// Command journal-http is a pull-mode adapter (docs/changes/08-journal-http-adapter.md): it queries
// a configured host's systemd-journal-gatewayd HTTP API for recent journal entries and emits NDJSON
// event lines on stdout. A non-zero exit (connection failure, TLS handshake failure, non-2xx
// response, unparsable config) is how the core's Runner learns to mark the source `unreachable`
// (docs/SPEC.md §4) — mirrors fail2ban-ssh's semantics: failure to reach the monitored host's
// gatewayd is the condition this adapter reports as unreachable, there is no separate status line.
package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// config is the adapter's stdin payload: the source's config_json with its client_cert/client_key
// fields already resolved from secret_refs to plaintext PEM by the core (see
// internal/platform/secrets.ResolveConfig) — this process never sees a secret_ref.
type config struct {
	Host             string `json:"host"`
	Port             int    `json:"port"`
	HTTPS            bool   `json:"https"`
	ClientCert       string `json:"client_cert"`
	ClientKey        string `json:"client_key"`
	Unit             string `json:"unit"`
	MinPriority      *int   `json:"min_priority"`
	LookbackSeconds  int    `json:"lookback_seconds"`
	ParseJSONMessage bool   `json:"parse_json_message"`
}

// ndjsonLine is deliberately independent of internal/contract's types — an adapter is any
// language, any process, talking only NDJSON-over-stdio, so it can't import core Go packages.
type ndjsonLine struct {
	Type      string            `json:"type"`
	SourceID  string            `json:"source_id"`
	Timestamp string            `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// journalEntry is one gatewayd /entries JSON-per-line response object — only the fields this
// adapter cares about; gatewayd emits many more (_PID, _HOSTNAME, ...) which are ignored.
type journalEntry struct {
	Message           string `json:"MESSAGE"`
	Priority          string `json:"PRIORITY"`
	SystemdUnit       string `json:"_SYSTEMD_UNIT"`
	SyslogIdentifier  string `json:"SYSLOG_IDENTIFIER"`
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"` // microseconds since epoch, as a string
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("journal-http: read config: %v", err)
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("journal-http: parse config: %v", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 19531
	}
	if cfg.LookbackSeconds <= 0 {
		cfg.LookbackSeconds = 120
	}
	minPriority := 6
	if cfg.MinPriority != nil {
		minPriority = *cfg.MinPriority
	}

	client, err := httpClient(cfg)
	if err != nil {
		log.Fatalf("journal-http: build HTTP client: %v", err)
	}

	entries, err := fetchEntries(client, cfg)
	if err != nil {
		log.Fatalf("journal-http: fetch entries from %s: %v", cfg.Host, err)
	}

	since := time.Now().Add(-time.Duration(cfg.LookbackSeconds) * time.Second)
	entries = filterByLookback(entries, since)

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	encoder := json.NewEncoder(writer)
	for _, entry := range filterEntries(entries, minPriority) {
		if err := encoder.Encode(toNDJSON(entry, cfg.ParseJSONMessage)); err != nil {
			log.Fatalf("journal-http: encode line: %v", err)
		}
	}
}

// httpClient builds an *http.Client per cfg, configured for mTLS when both client_cert and
// client_key are present. Host TLS verification is intentionally left at Go's default (unlike the
// SSH adapters' TOFU-skip, gatewayd's HTTPS mode implies the operator already provisioned a real
// certificate for it).
func httpClient(cfg config) (*http.Client, error) {
	transport := &http.Transport{}
	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(cfg.ClientCert), []byte(cfg.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("parse client cert/key: %w", err)
		}
		transport.TLSClientConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}, nil
}

// maxEntries bounds each poll's fetch to the last N journal entries. Time-window filtering
// (lookback_seconds) happens client-side in Go afterward — see the package doc comment on
// fetchEntries for why the Range: realtime= header can't be relied on for this directly.
const maxEntries = 2000

// fetchEntries issues one GET /entries request against cfg's host, requesting the last maxEntries
// entries via Range: entries=:-N:N and optionally filtering server-side to one systemd unit.
//
// This deliberately does not use gatewayd's documented Range: realtime=<since>: time-window syntax
// — live-verified against a real systemd 255 (Ubuntu 24.04) instance that syntax is accepted (200
// OK) but silently ignored, always returning from the start of the journal. Range: entries=:-N:N
// (last N entries) is honored correctly, so this mirrors fail2ban-ssh's "fetch a bounded tail, then
// time-filter in Go" pattern (filterByLookback below) instead.
func fetchEntries(client *http.Client, cfg config) ([]journalEntry, error) {
	scheme := "http"
	if cfg.HTTPS {
		scheme = "https"
	}
	u := url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/entries",
	}
	if cfg.Unit != "" {
		q := u.Query()
		q.Set("_SYSTEMD_UNIT", cfg.Unit)
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Range", fmt.Sprintf("entries=:-%d:%d", maxEntries, maxEntries))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return parseEntries(resp.Body)
}

// filterByLookback keeps entries whose __REALTIME_TIMESTAMP falls at or after since. Entries with a
// missing/unparsable timestamp are kept, same tolerance rule as filterEntries' priority handling —
// a malformed field must never silently drop a real event.
func filterByLookback(entries []journalEntry, since time.Time) []journalEntry {
	var kept []journalEntry
	for _, entry := range entries {
		usec, err := strconv.ParseInt(entry.RealtimeTimestamp, 10, 64)
		if err == nil && time.UnixMicro(usec).Before(since) {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// parseEntries decodes gatewayd's application/json response: one JSON object per line, not a JSON
// array. Lines that fail to decode are skipped rather than erroring, matching fail2ban-ssh's
// tolerance for malformed/unexpected lines.
func parseEntries(body io.Reader) ([]journalEntry, error) {
	var entries []journalEntry
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return entries, nil
}

// filterEntries keeps entries whose PRIORITY is at or more severe than minPriority (lower number =
// more severe). Entries with a missing/unparsable PRIORITY are kept (treated as unfiltered, since
// gatewayd should always set it, but a missing field must never silently drop real events).
func filterEntries(entries []journalEntry, minPriority int) []journalEntry {
	var kept []journalEntry
	for _, entry := range entries {
		priority, err := strconv.Atoi(entry.Priority)
		if err == nil && priority > minPriority {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// toNDJSON converts a parsed journalEntry into the wire event line: "warn" for priority <= 4
// (crit/alert/emerg/err), "info" otherwise (warning/notice/info/debug) — mirrors syslog severity
// convention (lower number = more severe). When parseJSONMessage is enabled, decorateJSONMessage
// may add extra labels and swap in a friendlier Message (docs/SPEC.md's generic structured-log
// support — opt-in, no assumption about which application emitted the log or its field names).
func toNDJSON(entry journalEntry, parseJSONMessage bool) ndjsonLine {
	level := "info"
	if priority, err := strconv.Atoi(entry.Priority); err == nil && priority <= 4 {
		level = "warn"
	}

	unit := entry.SystemdUnit
	if unit == "" {
		unit = entry.SyslogIdentifier
	}

	ts := time.Now().UTC()
	if usec, err := strconv.ParseInt(entry.RealtimeTimestamp, 10, 64); err == nil {
		ts = time.UnixMicro(usec).UTC()
	}

	message := entry.Message
	labels := map[string]string{"unit": unit, "priority": entry.Priority}
	if parseJSONMessage {
		message = decorateJSONMessage(entry.Message, labels)
	}

	return ndjsonLine{
		Type:      "event",
		SourceID:  "journal-http",
		Timestamp: ts.Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Labels:    labels,
	}
}

// decorateJSONMessage attempts to JSON-decode rawMessage as an object. On success, every top-level
// scalar (string/number/bool) field is added to labels (stringified via fmt.Sprint; nested
// objects/arrays are skipped — flat labels only), and a conventional "message"/"msg" key (checked
// case-insensitively), if present, is returned as the friendlier display text in place of the raw
// JSON blob. This is deliberately generic: it doesn't assume any particular application's log
// schema (field names, "kind"/"route"/whatever) — it just surfaces whatever structured fields a
// JSON log line happens to carry. Non-JSON or non-object messages are returned unchanged, so
// disabling parse_json_message (the default) reproduces today's exact behavior, and enabling it
// against a source whose logs aren't JSON is a silent no-op rather than an error.
func decorateJSONMessage(rawMessage string, labels map[string]string) string {
	var fields map[string]any
	if err := json.Unmarshal([]byte(rawMessage), &fields); err != nil {
		return rawMessage
	}

	message := rawMessage
	for key, value := range fields {
		switch v := value.(type) {
		case string:
			labels[key] = v
			if message == rawMessage && (strings.EqualFold(key, "message") || strings.EqualFold(key, "msg")) {
				message = v
			}
		case float64, bool:
			labels[key] = fmt.Sprint(v)
		default:
			// nested object/array — skipped, labels stay flat
		}
	}
	return message
}
