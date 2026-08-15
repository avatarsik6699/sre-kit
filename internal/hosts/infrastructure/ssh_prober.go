package infrastructure

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"sre-kit/internal/hosts/application"
)

// SSHProber implements application.ConnectionProber against a real SSH connection. It deliberately
// never rejects a connection based on the presented host key — it only observes and reports the
// fingerprint (ssh.FingerprintSHA256, the same format `ssh-keygen -lf` prints); the trust decision
// (pin on first connect, refuse on mismatch) is application-level policy, kept out of this package
// so it stays unit-testable without a real network dial (docs/SPEC.md §12.4).
type SSHProber struct{}

// NewSSHProber constructs an SSHProber.
func NewSSHProber() *SSHProber { return &SSHProber{} }

// Probe dials address:port as user with privateKeyPEM, records the presented host key's SHA256
// fingerprint, and — on a successful connect — runs `docker compose version` to determine
// DockerAvailable. A failed docker probe does not fail the overall connection check: a host can be
// validly added before Docker is installed on it.
func (p *SSHProber) Probe(ctx context.Context, address string, port int, user string, privateKeyPEM string) (application.ProbeResult, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return application.ProbeResult{}, fmt.Errorf("hosts: parse ssh key: %w", err)
	}

	var fingerprint string
	clientConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			fingerprint = ssh.FingerprintSHA256(key)
			return nil // accept-and-observe; trust decision belongs to application.Service.CheckConnection
		},
		Timeout: 10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", address, port), clientConfig)
	if err != nil {
		return application.ProbeResult{}, fmt.Errorf("hosts: dial %s:%d: %w", address, port, err)
	}
	defer client.Close()

	dockerAvailable := probeDocker(client)

	return application.ProbeResult{HostKeyFingerprint: fingerprint, DockerAvailable: dockerAvailable}, nil
}

// probeDocker runs `docker compose version` over a fresh session; any error (missing binary,
// permission denied, non-zero exit) just means "not available yet," not a probe failure.
func probeDocker(client *ssh.Client) bool {
	session, err := client.NewSession()
	if err != nil {
		return false
	}
	defer session.Close()
	return session.Run("docker compose version") == nil
}
