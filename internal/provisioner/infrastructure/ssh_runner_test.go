package infrastructure_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"sre-kit/internal/provisioner/application"
	"sre-kit/internal/provisioner/infrastructure"
)

// startFakeSSHServer runs a minimal in-process SSH server accepting any public key. Every exec
// request writes back opts.output, echoes stdin (if any) into lastStdin, and exits 0.
type fakeServerState struct {
	lastStdin string
}

func startFakeSSHServer(t *testing.T, output string) (addr string, state *fakeServerState, hostKeyFingerprint string) {
	t.Helper()
	state = &fakeServerState{}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}
	fingerprint := ssh.FingerprintSHA256(hostSigner.PublicKey())

	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
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
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				_, chans, reqs, err := ssh.NewServerConn(conn, serverConfig)
				if err != nil {
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
							if req.WantReply {
								_ = req.Reply(true, nil)
							}
							stdin, _ := io.ReadAll(channel)
							if len(stdin) > 0 {
								state.lastStdin = string(stdin)
							}
							_, _ = channel.Write([]byte(output))
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							return
						}
					}()
				}
			}()
		}
	}()

	return listener.Addr().String(), state, fingerprint
}

func newTestED25519KeyPEM(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
}

func connFor(t *testing.T, addr, fingerprint string) application.HostConn {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host:port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return application.HostConn{
		Address:             host,
		Port:                port,
		User:                "operator",
		PrivateKeyPEM:       string(newTestED25519KeyPEM(t)),
		ExpectedFingerprint: fingerprint,
	}
}

func TestRunCommand_ReturnsOutput(t *testing.T) {
	addr, _, fingerprint := startFakeSSHServer(t, "hello\n")
	runner := infrastructure.NewSSHRunner()

	output, err := runner.RunCommand(context.Background(), connFor(t, addr, fingerprint), "docker compose up -d")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if output != "hello\n" {
		t.Fatalf("output = %q, want %q", output, "hello\n")
	}
}

func TestRunCommand_RefusesWithoutPinnedFingerprint(t *testing.T) {
	addr, _, _ := startFakeSSHServer(t, "hello\n")
	runner := infrastructure.NewSSHRunner()

	if _, err := runner.RunCommand(context.Background(), connFor(t, addr, ""), "echo hi"); err == nil {
		t.Fatal("expected an error when ExpectedFingerprint is empty")
	}
}

func TestRunCommand_RefusesOnFingerprintMismatch(t *testing.T) {
	addr, _, _ := startFakeSSHServer(t, "hello\n")
	runner := infrastructure.NewSSHRunner()

	if _, err := runner.RunCommand(context.Background(), connFor(t, addr, "SHA256:not-the-real-one"), "echo hi"); err == nil {
		t.Fatal("expected an error on fingerprint mismatch")
	}
}

func TestUploadFile_WritesContentAsStdin(t *testing.T) {
	addr, state, fingerprint := startFakeSSHServer(t, "")
	runner := infrastructure.NewSSHRunner()

	content := []byte("services:\n  beszel:\n    image: henrygd/beszel\n")
	if err := runner.UploadFile(context.Background(), connFor(t, addr, fingerprint), ".sre-kit/run-1/docker-compose.yml", content); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if !strings.Contains(state.lastStdin, "henrygd/beszel") {
		t.Fatalf("server did not receive uploaded content via stdin, got %q", state.lastStdin)
	}
}
