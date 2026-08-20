// Command fail2ban-ssh is a pull-mode adapter (docs/SPEC.md M6): it SSHes into a configured host,
// reads fail2ban's default log file (/var/log/fail2ban.log) for recent ban/unban activity, and
// emits NDJSON event lines on stdout. A non-zero exit (connect/auth failure, unparsable config) is
// how the core's Runner learns to mark the source `unreachable` (docs/SPEC.md §4) — mirrors
// host-metrics-ssh's semantics: SSH failure to the monitored host itself is the condition this
// adapter reports as unreachable, there is no separate status line.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// config is the adapter's stdin payload: the source's config_json with its "secret" field already
// resolved from a secret_ref to the plaintext SSH password or PEM-encoded private key by the core
// (see internal/platform/secrets.ResolveConfig) — this process never sees a secret_ref.
type config struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	AuthMethod      string `json:"auth_method"`
	Secret          string `json:"secret"`
	Jail            string `json:"jail"`
	LookbackSeconds int    `json:"lookback_seconds"`
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

// banEvent is one parsed ban/unban occurrence from fail2ban's log.
type banEvent struct {
	Timestamp time.Time
	Jail      string
	Action    string // "Ban" | "Unban"
	IP        string
}

// tailLogScript prints the last logLines of fail2ban's default log file to stdout. Filtering by
// time window and jail happens in Go (see filterEvents) rather than remotely, so the parsing logic
// stays testable against fixture text without depending on remote awk/date quirks.
const logLines = 2000

// logLineRE matches fail2ban.actions NOTICE lines for a ban/unban, e.g.:
//
//	2024-01-15 10:23:45,123 fail2ban.actions        [12345]: NOTICE  [sshd] Ban 192.168.1.100
//	2024-01-15 10:25:12,456 fail2ban.actions        [12345]: NOTICE  [sshd] Unban 192.168.1.100
var logLineRE = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}),\d+.*NOTICE\s+\[([\w.-]+)\]\s+(Ban|Unban)\s+([0-9a-fA-F:.]+)`,
)

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("fail2ban-ssh: read config: %v", err)
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("fail2ban-ssh: parse config: %v", err)
	}
	if cfg.LookbackSeconds <= 0 {
		cfg.LookbackSeconds = 120
	}

	client, err := dial(cfg)
	if err != nil {
		log.Fatalf("fail2ban-ssh: connect to %s: %v", cfg.Host, err)
	}
	defer client.Close()

	output, err := tailLog(client)
	if err != nil {
		log.Fatalf("fail2ban-ssh: read fail2ban log on %s: %v", cfg.Host, err)
	}

	now := time.Now().UTC()
	since := now.Add(-time.Duration(cfg.LookbackSeconds) * time.Second)
	events := filterEvents(parseLog(output), since, cfg.Jail)

	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	encoder := json.NewEncoder(writer)
	for _, ev := range events {
		if err := encoder.Encode(toNDJSON(ev)); err != nil {
			log.Fatalf("fail2ban-ssh: encode line: %v", err)
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

// tailLog opens one session on client and returns the tail of fail2ban's default log file.
func tailLog(client *ssh.Client) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("open session: %w", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf("tail -n %d /var/log/fail2ban.log", logLines)
	output, err := session.Output(cmd)
	if err != nil {
		return "", fmt.Errorf("run %q: %w", cmd, err)
	}
	return string(output), nil
}

// parseLog parses every ban/unban NOTICE line out of raw fail2ban.log text, skipping lines that
// don't match (other log levels, multi-line context, etc.) rather than erroring.
func parseLog(raw string) []banEvent {
	var events []banEvent
	for _, line := range strings.Split(raw, "\n") {
		match := logLineRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		ts, err := time.Parse("2006-01-02 15:04:05", match[1])
		if err != nil {
			continue
		}
		events = append(events, banEvent{
			Timestamp: ts.UTC(),
			Jail:      match[2],
			Action:    match[3],
			IP:        match[4],
		})
	}
	return events
}

// filterEvents keeps events at or after since, optionally restricted to one jail (empty = all).
func filterEvents(events []banEvent, since time.Time, jail string) []banEvent {
	var kept []banEvent
	for _, ev := range events {
		if ev.Timestamp.Before(since) {
			continue
		}
		if jail != "" && ev.Jail != jail {
			continue
		}
		kept = append(kept, ev)
	}
	return kept
}

// toNDJSON converts a parsed banEvent into the wire event line: "warn" for a ban, "info" for an
// unban (docs/changes/archive/06-fail2ban-ssh-adapter.md I2).
func toNDJSON(ev banEvent) ndjsonLine {
	level := "info"
	verb := "unbanned"
	if ev.Action == "Ban" {
		level = "warn"
		verb = "banned"
	}
	return ndjsonLine{
		Type:      "event",
		SourceID:  "fail2ban-ssh",
		Timestamp: ev.Timestamp.Format(time.RFC3339),
		Level:     level,
		Message:   fmt.Sprintf("fail2ban %s %s in jail %s", verb, ev.IP, ev.Jail),
		Labels:    map[string]string{"jail": ev.Jail, "ip": ev.IP, "action": strings.ToLower(ev.Action)},
	}
}
