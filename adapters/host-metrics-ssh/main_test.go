package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"strconv"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestParseSample(t *testing.T) {
	cases := []struct {
		name           string
		output         string
		wantErr        bool
		cpu, mem, disk float64
	}{
		{name: "valid", output: "12.50 41.20 63.80\n", cpu: 12.50, mem: 41.20, disk: 63.80},
		{name: "extra whitespace", output: "  1   2   3  \n", cpu: 1, mem: 2, disk: 3},
		{name: "wrong field count", output: "1.00 2.00\n", wantErr: true},
		{name: "non numeric", output: "a b c\n", wantErr: true},
		{name: "empty", output: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cpu, mem, disk, err := parseSample(tc.output)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cpu != tc.cpu || mem != tc.mem || disk != tc.disk {
				t.Fatalf("got (%v, %v, %v), want (%v, %v, %v)", cpu, mem, disk, tc.cpu, tc.mem, tc.disk)
			}
		})
	}
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

	t.Run("private_key", func(t *testing.T) {
		method, err := sshAuthMethod(config{AuthMethod: "private_key", Secret: string(newTestRSAKeyPEM(t))})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if method == nil {
			t.Fatal("expected a non-nil AuthMethod")
		}
	})

	t.Run("private_key with malformed PEM", func(t *testing.T) {
		if _, err := sshAuthMethod(config{AuthMethod: "private_key", Secret: "not a key"}); err == nil {
			t.Fatal("expected an error for malformed PEM")
		}
	})

	t.Run("unknown auth_method", func(t *testing.T) {
		if _, err := sshAuthMethod(config{AuthMethod: "carrier-pigeon"}); err == nil {
			t.Fatal("expected an error for an unknown auth_method")
		}
	})
}

func TestDialAndSample_Success(t *testing.T) {
	addr := startFakeSSHServer(t, fakeSSHServerOptions{
		password: "s3cret",
		output:   "12.34 56.78 90.12\n",
	})
	cfg := dialConfigFor(t, addr, "password", "s3cret")

	client, err := dial(cfg)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	cpu, mem, disk, err := sample(client)
	if err != nil {
		t.Fatalf("sample: %v", err)
	}
	if cpu != 12.34 || mem != 56.78 || disk != 90.12 {
		t.Fatalf("got (%v, %v, %v), want (12.34, 56.78, 90.12)", cpu, mem, disk)
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

func newTestRSAKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

type fakeSSHServerOptions struct {
	password   string
	output     string
	rejectAuth bool
}

// startFakeSSHServer runs a minimal in-process SSH server (docs/changes/02-host-metrics-ssh-adapter.md
// I3: exercise the connect/auth-failure path without a real host) that accepts one connection,
// authenticates by password, and on any "exec" request writes opts.output and exits 0.
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
