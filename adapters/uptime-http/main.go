// Command uptime-http is a pull-mode adapter (docs/SPEC.md M3): probes a configured HTTP(S) or
// TCP target and emits NDJSON check lines on stdout. Unlike host-metrics-ssh, a down/unreachable
// *target* is the exact condition this adapter exists to observe, so it is reported as a normal
// `critical`-status check line (exit 0) rather than a non-zero process exit — a non-zero exit
// would make the core mark the whole *source* `unreachable` (docs/SPEC.md §4/§6), which has its
// own debounced alert semantics separate from a check's `status_is` alert rule and would prevent
// an alert rule targeting this check's name from ever firing while the target is down. A non-zero
// exit is reserved for genuine adapter-level failures (unparsuable config, missing "url").
package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// config is the adapter's stdin payload: the source's config_json with basic_auth_secret already
// resolved from a secret_ref to a plaintext "username:password" pair by the core (see
// internal/platform/secrets.ResolveConfig) — this process never sees a secret_ref.
type config struct {
	URL               string `json:"url"`
	Method            string `json:"method"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
	ExpectStatus      int    `json:"expect_status"`
	BasicAuthSecret   string `json:"basic_auth_secret"`
	TLSExpiryWarnDays int    `json:"tls_expiry_warn_days"`
}

func (c *config) applyDefaults() {
	if c.Method == "" {
		c.Method = http.MethodGet
	}
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 10
	}
	if c.ExpectStatus <= 0 {
		c.ExpectStatus = 200
	}
	if c.TLSExpiryWarnDays <= 0 {
		c.TLSExpiryWarnDays = 14
	}
}

// checkLine mirrors contract.schema.json's "check" variant. Deliberately independent of
// internal/contract's types — an adapter is any language, any process, talking only
// NDJSON-over-stdio, so it can't import core Go packages.
type checkLine struct {
	Type      string                 `json:"type"`
	SourceID  string                 `json:"source_id"`
	Name      string                 `json:"name"`
	Timestamp string                 `json:"timestamp"`
	Status    string                 `json:"status"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("uptime-http: read config: %v", err)
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("uptime-http: parse config: %v", err)
	}
	if cfg.URL == "" {
		log.Fatalf("uptime-http: config: \"url\" is required")
	}
	cfg.applyDefaults()

	target, err := url.Parse(cfg.URL)
	if err != nil {
		log.Fatalf("uptime-http: parse url %q: %v", cfg.URL, err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var lines []checkLine

	switch target.Scheme {
	case "tcp":
		lines = append(lines, tcpCheckLine(target.Host, time.Duration(cfg.TimeoutSeconds)*time.Second, now))
	case "http", "https":
		httpLine, tlsState := httpCheckLine(cfg, now)
		lines = append(lines, httpLine)
		if target.Scheme == "https" && tlsState != nil && len(tlsState.PeerCertificates) > 0 {
			lines = append(lines, tlsExpiryCheckLine(tlsState.PeerCertificates[0].NotAfter, time.Now(), cfg.TLSExpiryWarnDays, now))
		}
	default:
		log.Fatalf("uptime-http: config: unsupported url scheme %q (want http, https, or tcp)", target.Scheme)
	}

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	encoder := json.NewEncoder(writer)
	for _, line := range lines {
		if err := encoder.Encode(line); err != nil {
			log.Fatalf("uptime-http: encode line: %v", err)
		}
	}
}

// parseURLHost returns rawURL's host:port, e.g. for turning an httptest.Server's http:// URL into
// a tcp:// dial target in tests.
func parseURLHost(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

// tcpCheckLine dials address and reports "ok" on connect success, "critical" otherwise.
func tcpCheckLine(address string, timeout time.Duration, ts string) checkLine {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return checkLine{
			Type: "check", SourceID: "uptime-http", Name: "uptime.tcp", Timestamp: ts,
			Status: "critical", Meta: map[string]interface{}{"error": err.Error()},
		}
	}
	_ = conn.Close()
	return checkLine{Type: "check", SourceID: "uptime-http", Name: "uptime.tcp", Timestamp: ts, Status: "ok"}
}

// httpCheckLine performs the configured HTTP request against the default (standard TLS
// verification) client and classifies the outcome.
func httpCheckLine(cfg config, ts string) (checkLine, *tls.ConnectionState) {
	client := &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	return httpCheckLineWithClient(cfg, ts, client)
}

// httpCheckLineWithClient is httpCheckLine with an injectable *http.Client, so tests can pass an
// httptest.Server's own client (which trusts that server's self-signed cert) without weakening
// the production default's TLS verification. It returns the completed TLS connection state (nil
// if the request failed before/without a TLS handshake, or the target isn't https) so the caller
// can derive a separate tls.cert_expiry check without a second connection.
func httpCheckLineWithClient(cfg config, ts string, client *http.Client) (checkLine, *tls.ConnectionState) {
	req, err := http.NewRequest(cfg.Method, cfg.URL, nil)
	if err != nil {
		return checkLine{
			Type: "check", SourceID: "uptime-http", Name: "uptime.http", Timestamp: ts,
			Status: "critical", Meta: map[string]interface{}{"error": err.Error()},
		}, nil
	}
	if cfg.BasicAuthSecret != "" {
		user, pass, ok := strings.Cut(cfg.BasicAuthSecret, ":")
		if ok {
			req.SetBasicAuth(user, pass)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return checkLine{
			Type: "check", SourceID: "uptime-http", Name: "uptime.http", Timestamp: ts,
			Status: "critical", Meta: map[string]interface{}{"error": err.Error()},
		}, nil
	}
	defer resp.Body.Close()

	return checkLine{
		Type: "check", SourceID: "uptime-http", Name: "uptime.http", Timestamp: ts,
		Status: statusForCode(resp.StatusCode, cfg.ExpectStatus),
		Meta:   map[string]interface{}{"status_code": resp.StatusCode},
	}, resp.TLS
}

// statusForCode is the pure status-code -> check-status mapping.
func statusForCode(code, expect int) string {
	if code == expect {
		return "ok"
	}
	return "critical"
}

// tlsExpiryCheckLine builds the tls.cert_expiry check line from a certificate's NotAfter.
func tlsExpiryCheckLine(notAfter, now time.Time, warnDays int, ts string) checkLine {
	days := daysUntil(notAfter, now)
	return checkLine{
		Type: "check", SourceID: "uptime-http", Name: "tls.cert_expiry", Timestamp: ts,
		Status: statusForCertExpiry(days, warnDays),
		Meta:   map[string]interface{}{"days_remaining": days, "not_after": notAfter.UTC().Format(time.RFC3339)},
	}
}

// daysUntil is the pure days-remaining calculation, floored (a cert expiring in 23h59m counts as
// 0 days remaining, not 1).
func daysUntil(notAfter, now time.Time) int {
	return int(notAfter.Sub(now).Hours() / 24)
}

// statusForCertExpiry is the pure days-remaining -> check-status mapping.
func statusForCertExpiry(daysRemaining, warnDays int) string {
	switch {
	case daysRemaining < 0:
		return "critical"
	case daysRemaining <= warnDays:
		return "warn"
	default:
		return "ok"
	}
}
