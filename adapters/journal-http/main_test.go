package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const fixtureBody = `{"MESSAGE":"connection refused","PRIORITY":"3","_SYSTEMD_UNIT":"nginx.service","__REALTIME_TIMESTAMP":"1705318800000000"}
{"MESSAGE":"heartbeat ok","PRIORITY":"6","_SYSTEMD_UNIT":"sre-kit.service","__REALTIME_TIMESTAMP":"1705318805000000"}
not valid json at all
{"MESSAGE":"debug trace","PRIORITY":"7","SYSLOG_IDENTIFIER":"cron","__REALTIME_TIMESTAMP":"1705318810000000"}
`

func TestParseEntries(t *testing.T) {
	entries, err := parseEntries(strings.NewReader(fixtureBody))
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	if entries[0].Message != "connection refused" || entries[0].SystemdUnit != "nginx.service" {
		t.Fatalf("entry 0: got %+v", entries[0])
	}
}

func TestFilterEntries(t *testing.T) {
	entries, err := parseEntries(strings.NewReader(fixtureBody))
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}

	t.Run("default min_priority 6 drops debug", func(t *testing.T) {
		got := filterEntries(entries, 6)
		if len(got) != 2 {
			t.Fatalf("got %d entries, want 2: %+v", len(got), got)
		}
	})

	t.Run("min_priority 3 keeps only crit-and-above", func(t *testing.T) {
		got := filterEntries(entries, 3)
		if len(got) != 1 || got[0].Message != "connection refused" {
			t.Fatalf("got %+v, want one connection-refused entry", got)
		}
	})

	t.Run("missing/unparsable priority is kept", func(t *testing.T) {
		got := filterEntries([]journalEntry{{Message: "no priority field"}}, 0)
		if len(got) != 1 {
			t.Fatalf("got %d entries, want 1", len(got))
		}
	})
}

func TestFilterByLookback(t *testing.T) {
	entries, err := parseEntries(strings.NewReader(fixtureBody))
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}

	t.Run("keeps entries at or after since", func(t *testing.T) {
		since := time.UnixMicro(1705318805000000) // exactly entries[1]'s timestamp
		got := filterByLookback(entries, since)
		if len(got) != 2 {
			t.Fatalf("got %d entries, want 2: %+v", len(got), got)
		}
	})

	t.Run("since in the future drops everything", func(t *testing.T) {
		got := filterByLookback(entries, time.Now().Add(time.Hour))
		if len(got) != 0 {
			t.Fatalf("got %d entries, want 0", len(got))
		}
	})

	t.Run("missing/unparsable timestamp is kept", func(t *testing.T) {
		got := filterByLookback([]journalEntry{{Message: "no timestamp field"}}, time.Now())
		if len(got) != 1 {
			t.Fatalf("got %d entries, want 1", len(got))
		}
	})
}

func TestToNDJSON(t *testing.T) {
	t.Run("high severity maps to warn", func(t *testing.T) {
		line := toNDJSON(journalEntry{Message: "boom", Priority: "3", SystemdUnit: "nginx.service", RealtimeTimestamp: "1705318800000000"})
		if line.Type != "event" || line.Level != "warn" {
			t.Fatalf("got %+v", line)
		}
		if line.Labels["unit"] != "nginx.service" || line.Labels["priority"] != "3" {
			t.Fatalf("got labels %+v", line.Labels)
		}
		wantTS := time.UnixMicro(1705318800000000).UTC().Format(time.RFC3339)
		if line.Timestamp != wantTS {
			t.Fatalf("got timestamp %q, want %q", line.Timestamp, wantTS)
		}
	})

	t.Run("low severity maps to info", func(t *testing.T) {
		line := toNDJSON(journalEntry{Message: "ok", Priority: "6", SystemdUnit: "sre-kit.service"})
		if line.Level != "info" {
			t.Fatalf("got %+v", line)
		}
	})

	t.Run("falls back to syslog identifier when unit is unset", func(t *testing.T) {
		line := toNDJSON(journalEntry{Message: "cron ran", Priority: "6", SyslogIdentifier: "cron"})
		if line.Labels["unit"] != "cron" {
			t.Fatalf("got labels %+v", line.Labels)
		}
	})
}

func TestFetchEntries_Success(t *testing.T) {
	var gotRange, gotAccept, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		gotAccept = r.Header.Get("Accept")
		gotQuery = r.URL.Query().Get("_SYSTEMD_UNIT")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fixtureBody)
	}))
	defer server.Close()

	cfg := configFor(t, server.URL, "nginx.service")
	client, err := httpClient(cfg)
	if err != nil {
		t.Fatalf("httpClient: %v", err)
	}

	entries, err := fetchEntries(client, cfg)
	if err != nil {
		t.Fatalf("fetchEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if gotAccept != "application/json" {
		t.Fatalf("got Accept %q", gotAccept)
	}
	wantRange := fmt.Sprintf("entries=:-%d:%d", maxEntries, maxEntries)
	if gotRange != wantRange {
		t.Fatalf("got Range %q, want %q", gotRange, wantRange)
	}
	if gotQuery != "nginx.service" {
		t.Fatalf("got _SYSTEMD_UNIT query %q", gotQuery)
	}
}

func TestFetchEntries_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	cfg := configFor(t, server.URL, "")
	client, err := httpClient(cfg)
	if err != nil {
		t.Fatalf("httpClient: %v", err)
	}

	if _, err := fetchEntries(client, cfg); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestFetchEntries_ConnectionRefused(t *testing.T) {
	cfg := config{Host: "127.0.0.1", Port: 1} // nothing listens on port 1
	client, err := httpClient(cfg)
	if err != nil {
		t.Fatalf("httpClient: %v", err)
	}

	if _, err := fetchEntries(client, cfg); err == nil {
		t.Fatal("expected a connection error")
	}
}

func TestHTTPClient_MTLS(t *testing.T) {
	cert, key := generateTestCertPEM(t)
	client, err := httpClient(config{ClientCert: string(cert), ClientKey: string(key)})
	if err != nil {
		t.Fatalf("httpClient: %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || len(transport.TLSClientConfig.Certificates) != 1 {
		t.Fatalf("expected a client certificate to be configured on the transport")
	}
}

func TestHTTPClient_InvalidCert(t *testing.T) {
	if _, err := httpClient(config{ClientCert: "not a cert", ClientKey: "not a key"}); err == nil {
		t.Fatal("expected an error for an invalid client cert/key pair")
	}
}

func configFor(t *testing.T, serverURL, unit string) config {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL %q: %v", serverURL, err)
	}
	host, portStr, err := splitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host:port %q: %v", u.Host, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	return config{Host: host, Port: port, Unit: unit, LookbackSeconds: 120}
}

func splitHostPort(hostport string) (string, string, error) {
	idx := strings.LastIndex(hostport, ":")
	if idx < 0 {
		return "", "", fmt.Errorf("no port in %q", hostport)
	}
	return hostport[:idx], hostport[idx+1:], nil
}

// generateTestCertPEM returns a minimal self-signed cert/key pair, PEM-encoded, for exercising
// httpClient's mTLS configuration path without a real CA.
func generateTestCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}
