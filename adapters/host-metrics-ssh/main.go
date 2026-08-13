// Command host-metrics-ssh is a pull-mode adapter (docs/SPEC.md M2): it SSHes into a configured
// host, samples CPU/RAM/disk usage over the remote shell, and emits NDJSON metric lines on
// stdout. A non-zero exit (connect/auth failure, sampling failure) is how the core's Runner
// learns to mark the source `unreachable` (docs/SPEC.md §4) — there is no separate status line.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// config is the adapter's stdin payload: the source's config_json with its "secret" field already
// resolved from a secret_ref to the plaintext SSH password or PEM-encoded private key by the core
// (see internal/platform/secrets.ResolveConfig) — this process never sees a secret_ref.
type config struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	AuthMethod string `json:"auth_method"`
	Secret     string `json:"secret"`
}

// ndjsonLine is deliberately independent of internal/contract's types — an adapter is any
// language, any process, talking only NDJSON-over-stdio, so it can't import core Go packages.
type ndjsonLine struct {
	Type      string  `json:"type"`
	SourceID  string  `json:"source_id"`
	Name      string  `json:"name"`
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// sampleScript prints "cpu_pct mem_pct disk_pct\n" to stdout on the remote host. CPU usage needs
// two /proc/stat samples a moment apart to compute a delta; the 1-second sleep happens remotely,
// inside the single exec, rather than as two separate SSH round-trips.
const sampleScript = `
read cpu user nice system idle iowait irq softirq steal rest < /proc/stat
idle1=$((idle + iowait))
total1=$((user + nice + system + idle + iowait + irq + softirq + steal))
sleep 1
read cpu user nice system idle iowait irq softirq steal rest < /proc/stat
idle2=$((idle + iowait))
total2=$((user + nice + system + idle + iowait + irq + softirq + steal))
idle_delta=$((idle2 - idle1))
total_delta=$((total2 - total1))
cpu_pct=$(awk -v idle="$idle_delta" -v total="$total_delta" 'BEGIN { if (total > 0) printf "%.2f", (1 - idle/total) * 100; else print "0" }')
mem_total=$(awk '/MemTotal/{print $2}' /proc/meminfo)
mem_avail=$(awk '/MemAvailable/{print $2}' /proc/meminfo)
mem_pct=$(awk -v total="$mem_total" -v avail="$mem_avail" 'BEGIN { if (total > 0) printf "%.2f", (1 - avail/total) * 100; else print "0" }')
disk_pct=$(df -P / | awk 'NR==2{gsub(/%/,"",$5); print $5}')
printf '%s %s %s\n' "$cpu_pct" "$mem_pct" "$disk_pct"
`

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("host-metrics-ssh: read config: %v", err)
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("host-metrics-ssh: parse config: %v", err)
	}

	client, err := dial(cfg)
	if err != nil {
		log.Fatalf("host-metrics-ssh: connect to %s: %v", cfg.Host, err)
	}
	defer client.Close()

	cpuPct, memPct, diskPct, err := sample(client)
	if err != nil {
		log.Fatalf("host-metrics-ssh: sample %s: %v", cfg.Host, err)
	}

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	now := time.Now().UTC().Format(time.RFC3339)
	encoder := json.NewEncoder(writer)
	for _, line := range []ndjsonLine{
		{Type: "metric", SourceID: "host-metrics-ssh", Name: "cpu.usage_percent", Timestamp: now, Value: cpuPct},
		{Type: "metric", SourceID: "host-metrics-ssh", Name: "mem.usage_percent", Timestamp: now, Value: memPct},
		{Type: "metric", SourceID: "host-metrics-ssh", Name: "disk.usage_percent", Timestamp: now, Value: diskPct},
	} {
		if err := encoder.Encode(line); err != nil {
			log.Fatalf("host-metrics-ssh: encode line: %v", err)
		}
	}
}

// dial opens an SSH connection per cfg. Host key verification is intentionally skipped in v1 —
// see docs/KNOWN_GOTCHAS.md — there is no known_hosts store or first-connection TOFU pinning yet.
func dial(cfg config) (*ssh.Client, error) {
	auth, err := sshAuthMethod(cfg)
	if err != nil {
		return nil, err
	}
	port := cfg.Port
	if port == 0 {
		port = 22
	}
	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, port), &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
}

// sshAuthMethod builds the ssh.AuthMethod for cfg.AuthMethod from cfg.Secret, which by this point
// is always the resolved plaintext password or PEM private key, never a secret_ref.
func sshAuthMethod(cfg config) (ssh.AuthMethod, error) {
	switch cfg.AuthMethod {
	case "password":
		return ssh.Password(cfg.Secret), nil
	case "private_key":
		signer, err := ssh.ParsePrivateKey([]byte(cfg.Secret))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("unknown auth_method %q", cfg.AuthMethod)
	}
}

// sample opens one session on client, runs sampleScript, and parses its output.
func sample(client *ssh.Client) (cpuPct, memPct, diskPct float64, err error) {
	session, err := client.NewSession()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("open session: %w", err)
	}
	defer session.Close()

	output, err := session.Output(sampleScript)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("run sample script: %w", err)
	}
	return parseSample(string(output))
}

// parseSample parses sampleScript's "cpu_pct mem_pct disk_pct" stdout line.
func parseSample(output string) (cpuPct, memPct, diskPct float64, err error) {
	fields := strings.Fields(output)
	if len(fields) != 3 {
		return 0, 0, 0, fmt.Errorf("expected 3 fields, got %d (output: %q)", len(fields), output)
	}
	values := make([]float64, len(fields))
	for i, field := range fields {
		v, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("parse field %d (%q): %w", i, field, err)
		}
		values[i] = v
	}
	return values[0], values[1], values[2], nil
}
