package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStatusForCode(t *testing.T) {
	cases := []struct {
		code, expect int
		want         string
	}{
		{200, 200, "ok"},
		{200, 201, "critical"},
		{500, 200, "critical"},
		{404, 404, "ok"},
	}
	for _, tc := range cases {
		if got := statusForCode(tc.code, tc.expect); got != tc.want {
			t.Errorf("statusForCode(%d, %d) = %q, want %q", tc.code, tc.expect, got, tc.want)
		}
	}
}

func TestStatusForCertExpiry(t *testing.T) {
	cases := []struct {
		name          string
		daysRemaining int
		warnDays      int
		want          string
	}{
		{"plenty of time", 90, 14, "ok"},
		{"just past warn threshold", 15, 14, "ok"},
		{"at warn threshold", 14, 14, "warn"},
		{"about to expire", 1, 14, "warn"},
		{"already expired", -1, 14, "critical"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusForCertExpiry(tc.daysRemaining, tc.warnDays); got != tc.want {
				t.Errorf("statusForCertExpiry(%d, %d) = %q, want %q", tc.daysRemaining, tc.warnDays, got, tc.want)
			}
		})
	}
}

func TestDaysUntil_FixtureCerts(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		offset     time.Duration
		wantStatus string
	}{
		{"expires in 90 days", 90 * 24 * time.Hour, "ok"},
		{"expires in 5 days", 5 * 24 * time.Hour, "warn"},
		{"expired yesterday", -24 * time.Hour, "critical"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cert := newTestCert(t, now.Add(tc.offset))
			days := daysUntil(cert.NotAfter, now)
			if got := statusForCertExpiry(days, 14); got != tc.wantStatus {
				t.Errorf("cert expiring %s: got status %q, want %q (days=%d)", tc.name, got, tc.wantStatus, days)
			}
		})
	}
}

func TestHTTPCheckLine_StatusCodeMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()

	cfg := config{URL: server.URL, ExpectStatus: http.StatusTeapot}
	cfg.applyDefaults()
	line, tlsState := httpCheckLine(cfg, "2026-08-13T12:00:00Z")
	if line.Status != "ok" {
		t.Fatalf("expected ok status for matching code, got %q (meta: %v)", line.Status, line.Meta)
	}
	if tlsState != nil {
		t.Fatalf("expected nil TLS state for a plain HTTP server")
	}

	cfg.ExpectStatus = http.StatusOK
	line, _ = httpCheckLine(cfg, "2026-08-13T12:00:00Z")
	if line.Status != "critical" {
		t.Fatalf("expected critical status for mismatched code, got %q", line.Status)
	}
}

func TestHTTPCheckLine_ConnectionFailure(t *testing.T) {
	cfg := config{URL: "http://127.0.0.1:1"}
	cfg.applyDefaults()
	line, tlsState := httpCheckLine(cfg, "2026-08-13T12:00:00Z")
	if line.Status != "critical" {
		t.Fatalf("expected critical status on connection failure, got %q", line.Status)
	}
	if tlsState != nil {
		t.Fatalf("expected nil TLS state on connection failure")
	}
	if _, ok := line.Meta["error"]; !ok {
		t.Fatalf("expected an \"error\" meta field, got %v", line.Meta)
	}
}

func TestHTTPCheckLine_TLSState(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config{URL: server.URL, ExpectStatus: http.StatusOK}
	cfg.applyDefaults()

	line, tlsState := httpCheckLineWithClient(cfg, "2026-08-13T12:00:00Z", server.Client())
	if line.Status != "ok" {
		t.Fatalf("expected ok status, got %q (meta: %v)", line.Status, line.Meta)
	}
	if tlsState == nil || len(tlsState.PeerCertificates) == 0 {
		t.Fatalf("expected a populated TLS state with peer certificates")
	}
}

func TestTCPCheckLine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	line := tcpCheckLine(mustHostPort(t, server.URL), 2*time.Second, "2026-08-13T12:00:00Z")
	if line.Status != "ok" {
		t.Fatalf("expected ok status for a reachable TCP target, got %q", line.Status)
	}

	line = tcpCheckLine("127.0.0.1:1", 2*time.Second, "2026-08-13T12:00:00Z")
	if line.Status != "critical" {
		t.Fatalf("expected critical status for an unreachable TCP target, got %q", line.Status)
	}
}

func mustHostPort(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := parseURLHost(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	return u
}

func newTestCert(t *testing.T, notAfter time.Time) *x509.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}
