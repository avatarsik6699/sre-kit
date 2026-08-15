// Package application holds the provisioning workflow: render a preset's compose file, deploy it
// over SSH, bootstrap the tool's admin account, and register the result as a Source. Every
// dependency is a narrow local port (ports, not direct imports — docs/STACK.md convention); the
// composition root (cmd/server/main.go) wires concrete implementations in.
package application

import "context"

// HostConn is a host's resolved SSH connection info — address/port/user plus the private key
// already resolved to plaintext (never a secret_ref past this point, same rule as
// secrets.ResolveConfig's contract for adapter subprocess config). ExpectedFingerprint is the
// fingerprint pinned by internal/hosts's CheckConnection (docs/SPEC.md §12.1/§12.4) — SSHRunner
// must refuse to connect if the presented key doesn't match, and refuse outright if this is empty
// (the host has never been through a successful check-connection, so there is nothing to verify
// against — deploying to an unverified host is exactly the risk §12.4 exists to prevent).
type HostConn struct {
	ID                  string
	Address             string
	Port                int
	User                string
	PrivateKeyPEM       string
	ExpectedFingerprint string
}

// HostsLookup resolves a Host's SSH connection info by ID. A func type (not an interface) — mirrors
// sources/application's AdapterConfigSchemaLookup, backed by a closure over internal/hosts's
// Service.Get at the composition root; internal/provisioner deliberately doesn't import
// internal/hosts directly.
type HostsLookup func(ctx context.Context, hostID string) (HostConn, error)

// SSHRunner is the narrow port onto a real SSH session, implemented by
// internal/provisioner/infrastructure against golang.org/x/crypto/ssh — the first core-side (not
// adapter-subprocess-side) SSH client (docs/SPEC.md §12.2).
type SSHRunner interface {
	// RunCommand runs command on conn's host and returns combined stdout+stderr. A non-zero exit
	// is returned as an error.
	RunCommand(ctx context.Context, conn HostConn, command string) (output string, err error)
	// UploadFile writes content to path on conn's host, creating parent directories as needed.
	UploadFile(ctx context.Context, conn HostConn, path string, content []byte) error
}

// SecretsStore is the narrow port onto internal/platform/secrets.Store this service needs to store
// a generated admin password and resolve it back to plaintext on a resumed retry — the same
// Put(value)->ref / Get(ref) mechanism every other adapter secret uses (docs/SPEC.md §3), with no
// new secrets machinery.
type SecretsStore interface {
	Put(value string) (string, error)
	Get(ref string) (string, error)
}

// SourceCreator registers a provisioned instance as a Source — a func type (not an interface),
// backed by a closure over sources/application.Service.Create at the composition root
// (docs/SPEC.md §12.2: no parallel source-creation path).
type SourceCreator func(ctx context.Context, adapterName string, configJSON string) (sourceID string, err error)
