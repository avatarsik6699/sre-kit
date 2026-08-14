package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const fixtureLog = `2024-01-15 10:20:00,001 fail2ban.filter        [12345]: INFO    [sshd] Found 203.0.113.9
2024-01-15 10:23:45,123 fail2ban.actions        [12345]: NOTICE  [sshd] Ban 203.0.113.9
2024-01-15 10:24:10,456 fail2ban.actions        [12345]: NOTICE  [nginx-http-auth] Ban 198.51.100.7
2024-01-15 10:25:12,789 fail2ban.actions        [12345]: NOTICE  [sshd] Unban 203.0.113.9
not a fail2ban line at all
`

func TestParseLog(t *testing.T) {
	events := parseLog(fixtureLog)
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3: %+v", len(events), events)
	}

	want := []banEvent{
		{Timestamp: mustParse(t, "2024-01-15 10:23:45"), Jail: "sshd", Action: "Ban", IP: "203.0.113.9"},
		{Timestamp: mustParse(t, "2024-01-15 10:24:10"), Jail: "nginx-http-auth", Action: "Ban", IP: "198.51.100.7"},
		{Timestamp: mustParse(t, "2024-01-15 10:25:12"), Jail: "sshd", Action: "Unban", IP: "203.0.113.9"},
	}
	for i, w := range want {
		got := events[i]
		if !got.Timestamp.Equal(w.Timestamp) || got.Jail != w.Jail || got.Action != w.Action || got.IP != w.IP {
			t.Fatalf("event %d: got %+v, want %+v", i, got, w)
		}
	}
}

func TestParseLog_NoMatches(t *testing.T) {
	events := parseLog("nothing here\nstill nothing\n")
	if len(events) != 0 {
		t.Fatalf("got %d events, want 0", len(events))
	}
}

func TestFilterEvents(t *testing.T) {
	all := parseLog(fixtureLog)

	t.Run("time window excludes older events", func(t *testing.T) {
		since := mustParse(t, "2024-01-15 10:24:00")
		got := filterEvents(all, since, "")
		if len(got) != 2 {
			t.Fatalf("got %d events, want 2: %+v", len(got), got)
		}
	})

	t.Run("jail filter", func(t *testing.T) {
		since := mustParse(t, "2024-01-15 00:00:00")
		got := filterEvents(all, since, "nginx-http-auth")
		if len(got) != 1 || got[0].IP != "198.51.100.7" {
			t.Fatalf("got %+v, want one nginx-http-auth event", got)
		}
	})

	t.Run("empty jail keeps all jails", func(t *testing.T) {
		since := mustParse(t, "2024-01-15 00:00:00")
		got := filterEvents(all, since, "")
		if len(got) != len(all) {
			t.Fatalf("got %d events, want %d", len(got), len(all))
		}
	})
}

func TestToNDJSON(t *testing.T) {
	t.Run("ban", func(t *testing.T) {
		line := toNDJSON(banEvent{Timestamp: mustParse(t, "2024-01-15 10:23:45"), Jail: "sshd", Action: "Ban", IP: "203.0.113.9"})
		if line.Type != "event" || line.Level != "warn" {
			t.Fatalf("got %+v", line)
		}
		if line.Labels["jail"] != "sshd" || line.Labels["ip"] != "203.0.113.9" || line.Labels["action"] != "ban" {
			t.Fatalf("got labels %+v", line.Labels)
		}
	})

	t.Run("unban", func(t *testing.T) {
		line := toNDJSON(banEvent{Timestamp: mustParse(t, "2024-01-15 10:25:12"), Jail: "sshd", Action: "Unban", IP: "203.0.113.9"})
		if line.Level != "info" || line.Labels["action"] != "unban" {
			t.Fatalf("got %+v", line)
		}
	})
}

func TestSSHAuthMethod(t *testing.T) {
	t.Run("password", func(t *testing.T) {
		method, err := sshAuthMethod(config{AuthMethod: "password", Secret: "hunter2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if method == nil {
			t.Fatal("expected a non-nil AuthMethod")
		}
	})

	t.Run("unknown auth_method", func(t *testing.T) {
		if _, err := sshAuthMethod(config{AuthMethod: "carrier-pigeon"}); err == nil {
			t.Fatal("expected an error for an unknown auth_method")
		}
	})
}

func TestDialAndTailLog_Success(t *testing.T) {
	addr := startFakeSSHServer(t, fakeSSHServerOptions{
		password: "s3cret",
		output:   fixtureLog,
	})
	cfg := dialConfigFor(t, addr, "password", "s3cret")

	client, err := dial(cfg)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	output, err := tailLog(client)
	if err != nil {
		t.Fatalf("tailLog: %v", err)
	}
	if len(parseLog(output)) != 3 {
		t.Fatalf("got %d events from remote output, want 3", len(parseLog(output)))
	}
}

func TestDial_AuthFailure(t *testing.T) {
	addr := startFakeSSHServer(t, fakeSSHServerOptions{
		password:   "s3cret",
		rejectAuth: true,
	})
	cfg := dialConfigFor(t, addr, "password", "wrong-password")

	if _, err := dial(cfg); err == nil {
		t.Fatal("expected an auth failure error")
	}
}

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return ts.UTC()
}

func dialConfigFor(t *testing.T, addr, authMethod, secret string) config {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host:port %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	return config{Host: host, Port: port, Username: "test", AuthMethod: authMethod, Secret: secret}
}

type fakeSSHServerOptions struct {
	password   string
	output     string
	rejectAuth bool
}

// startFakeSSHServer runs a minimal in-process SSH server (docs/changes/06-fail2ban-ssh-adapter.md
// I3: exercise the connect/auth-failure path without a real host, same technique as change-02's
// I3) that accepts one connection, authenticates by password, and on any "exec" request writes
// opts.output and exits 0.
func startFakeSSHServer(t *testing.T, opts fakeSSHServerOptions) string {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if opts.rejectAuth || string(password) != opts.password {
				return nil, fmt.Errorf("fake ssh server: authentication rejected")
			}
			return nil, nil
		},
	}
	serverConfig.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_, chans, reqs, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			// Auth failure (or any handshake error) — nothing more to serve.
			return
		}
		go ssh.DiscardRequests(reqs)
		for newChannel := range chans {
			if newChannel.ChannelType() != "session" {
				_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				return
			}
			go func() {
				defer channel.Close()
				for req := range requests {
					if req.Type != "exec" {
						if req.WantReply {
							_ = req.Reply(false, nil)
						}
						continue
					}
					_, _ = channel.Write([]byte(opts.output))
					if req.WantReply {
						_ = req.Reply(true, nil)
					}
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					return
				}
			}()
		}
	}()

	return listener.Addr().String()
}
