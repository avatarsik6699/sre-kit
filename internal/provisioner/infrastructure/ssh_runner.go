// Package infrastructure implements the provisioner's SSHRunner port against a real SSH
// connection — the first core-side (not adapter-subprocess-side) user of golang.org/x/crypto/ssh
// (docs/SPEC.md §12.2).
package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"sre-kit/internal/provisioner/application"
)

// SSHRunner implements application.SSHRunner. Unlike internal/hosts/infrastructure.SSHProber
// (which only observes a host key so the application layer can decide whether to trust it),
// SSHRunner enforces the trust decision itself: every dial requires conn.ExpectedFingerprint to be
// set and to match the presented key, because this package runs write-capable commands
// (docs/SPEC.md §12.4) — there is no equivalent of the read-only adapters' accepted TOFU exemption
// here.
type SSHRunner struct{}

// NewSSHRunner constructs an SSHRunner.
func NewSSHRunner() *SSHRunner { return &SSHRunner{} }

func (r *SSHRunner) dial(conn application.HostConn) (*ssh.Client, error) {
	if conn.ExpectedFingerprint == "" {
		return nil, fmt.Errorf("provisioner: host %s has no pinned host-key fingerprint — run check-connection before provisioning", conn.Address)
	}
	signer, err := ssh.ParsePrivateKey([]byte(conn.PrivateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("provisioner: parse ssh key: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: conn.User,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			if got := ssh.FingerprintSHA256(key); got != conn.ExpectedFingerprint {
				return fmt.Errorf("provisioner: host key fingerprint mismatch: got %s, want %s (possible MITM — re-run check-connection to confirm)", got, conn.ExpectedFingerprint)
			}
			return nil
		},
		Timeout: 10 * time.Second,
	}

	port := conn.Port
	if port == 0 {
		port = 22
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", conn.Address, port), clientConfig)
	if err != nil {
		return nil, fmt.Errorf("provisioner: dial %s:%d: %w", conn.Address, port, err)
	}
	return client, nil
}

// RunCommand runs command on conn's host over a fresh session and returns combined stdout+stderr.
// A non-zero exit is returned as an error (with the captured output included, for diagnosability).
func (r *SSHRunner) RunCommand(_ context.Context, conn application.HostConn, command string) (string, error) {
	client, err := r.dial(conn)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("provisioner: open session: %w", err)
	}
	defer session.Close()

	var output bytes.Buffer
	session.Stdout = &output
	session.Stderr = &output
	if err := session.Run(command); err != nil {
		return output.String(), fmt.Errorf("provisioner: run %q: %w (output: %s)", command, err, output.String())
	}
	return output.String(), nil
}

// UploadFile writes content to path on conn's host, creating parent directories as needed. Avoids
// depending on an SFTP library: the file is streamed as the remote command's stdin
// (`mkdir -p <dir> && cat > <path>`) rather than embedded in a command-line string, avoiding
// shell-quoting failure modes for arbitrary file content.
func (r *SSHRunner) UploadFile(_ context.Context, conn application.HostConn, path string, content []byte) error {
	client, err := r.dial(conn)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("provisioner: open session: %w", err)
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(content)
	var output bytes.Buffer
	session.Stderr = &output
	command := fmt.Sprintf("mkdir -p %q && cat > %q", parentDir(path), path)
	if err := session.Run(command); err != nil {
		return fmt.Errorf("provisioner: upload %s: %w (output: %s)", path, err, output.String())
	}
	return nil
}

// parentDir returns the directory portion of a slash-separated remote path, "." if path has no
// slash. Deliberately not path/filepath (that's host-OS-aware; remote paths are always POSIX here).
func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
