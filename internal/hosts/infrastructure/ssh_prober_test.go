package infrastructure_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strconv"
	"testing"

	"golang.org/x/crypto/ssh"

	"sre-kit/internal/hosts/infrastructure"
)

type fakeServerOptions struct {
	acceptExitCode int // exit code every exec command returns
}

// startFakeSSHServer runs a minimal in-process SSH server accepting any public key, serving every
// exec request with opts.acceptExitCode — mirrors adapters/host-metrics-ssh/main_test.go's
// startFakeSSHServer, generalized to public-key auth and multiple sequential sessions (a real probe
// opens more than one session: the connection itself, then a docker-probe session).
func startFakeSSHServer(t *testing.T, opts fakeServerOptions) string {
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
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil // accept any key — this fake server only exists to exercise the client side
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
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(opts.acceptExitCode)}))
					return
				}
			}()
		}
	}()

	return listener.Addr().String()
}

func TestProbe_ReportsFingerprintAndDockerAvailability(t *testing.T) {
	addr := startFakeSSHServer(t, fakeServerOptions{acceptExitCode: 0})
	host, port := splitAddr(t, addr)

	prober := infrastructure.NewSSHProber()
	result, err := prober.Probe(context.Background(), host, port, "operator", string(newTestED25519KeyPEM(t)))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.HostKeyFingerprint == "" {
		t.Fatal("HostKeyFingerprint is empty")
	}
	if !result.DockerAvailable {
		t.Fatal("DockerAvailable = false, want true (fake server returns exit 0 for any command)")
	}
}

func TestProbe_DockerUnavailableOnNonZeroExit(t *testing.T) {
	addr := startFakeSSHServer(t, fakeServerOptions{acceptExitCode: 1})
	host, port := splitAddr(t, addr)

	prober := infrastructure.NewSSHProber()
	result, err := prober.Probe(context.Background(), host, port, "operator", string(newTestED25519KeyPEM(t)))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.DockerAvailable {
		t.Fatal("DockerAvailable = true, want false on a non-zero exit")
	}
}

func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host:port %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	return host, port
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
