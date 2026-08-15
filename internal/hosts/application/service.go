// Package application holds the Host use-cases: Create, CheckConnection, List, Get, Delete.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sre-kit/internal/hosts/domain"
	"sre-kit/internal/platform/apierror"
)

// SecretsStore is the narrow port onto internal/platform/secrets.Store this service needs to store
// the user-pasted SSH private key and resolve it back to plaintext for a connection check — mirrors
// internal/sources/application's own narrow secrets port (docs/SPEC.md §12.1: reuses the existing
// secret_ref mechanism, no new secrets machinery).
type SecretsStore interface {
	Put(value string) (string, error)
	Get(ref string) (string, error)
	Delete(ref string) error
}

// ProbeResult is what a connection probe observes about a host — deliberately dumb: it reports the
// presented host key fingerprint and docker availability, but does not itself decide whether to
// trust the fingerprint. That trust decision (pin on first connect, refuse on mismatch) is
// application-level (docs/SPEC.md §12.4), kept out of the infrastructure prober so the TOFU policy
// lives in one place and is unit-testable without a real network dial.
type ProbeResult struct {
	HostKeyFingerprint string
	DockerAvailable    bool
}

// ConnectionProber is the port onto the real SSH dial + docker probe, implemented by
// internal/hosts/infrastructure against golang.org/x/crypto/ssh.
type ConnectionProber interface {
	Probe(ctx context.Context, address string, port int, user string, privateKeyPEM string) (ProbeResult, error)
}

// Service implements the Host use-cases against a domain.Repository.
type Service struct {
	repo    domain.Repository
	secrets SecretsStore
	prober  ConnectionProber
	now     func() time.Time
}

// Option configures optional Service dependencies.
type Option func(*Service)

// WithProber wires the connection prober used by CheckConnection. Without it, CheckConnection
// returns an error — a Service under test that doesn't exercise connection checks doesn't need one.
func WithProber(prober ConnectionProber) Option {
	return func(s *Service) { s.prober = prober }
}

// NewService wires a Service to its repository port, secrets store, and any Options.
func NewService(repo domain.Repository, secrets SecretsStore, opts ...Option) *Service {
	s := &Service{repo: repo, secrets: secrets, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create validates and persists a new Host. sshKeyPEM is the plaintext private key the user pasted
// into the Host setup form; it is stored via the existing secret_ref mechanism (docs/SPEC.md §3)
// and never persisted or returned as plaintext again. The host starts with no pinned
// HostKeyFingerprint — CheckConnection pins it on first successful connect.
func (s *Service) Create(ctx context.Context, label, address string, sshPort int, sshUser, sshKeyPEM string) (domain.Host, error) {
	if address == "" {
		return domain.Host{}, apierror.Invalid("address is required")
	}
	if sshUser == "" {
		return domain.Host{}, apierror.Invalid("ssh_user is required")
	}
	if sshKeyPEM == "" {
		return domain.Host{}, apierror.Invalid("ssh_key is required")
	}
	if sshPort == 0 {
		sshPort = 22
	}
	ref, err := s.secrets.Put(sshKeyPEM)
	if err != nil {
		return domain.Host{}, fmt.Errorf("hosts: store ssh key: %w", err)
	}

	host := domain.Host{
		ID:              uuid.NewString(),
		Label:           label,
		Address:         address,
		SSHPort:         sshPort,
		SSHUser:         sshUser,
		SSHKeySecretRef: ref,
		LastStatus:      domain.StatusUnreachable,
		CreatedAt:       s.now(),
	}
	if err := s.repo.Create(ctx, host); err != nil {
		return domain.Host{}, fmt.Errorf("hosts: create: %w", err)
	}
	return host, nil
}

// Get returns the host identified by id.
func (s *Service) Get(ctx context.Context, id string) (domain.Host, error) {
	return s.repo.Get(ctx, id)
}

// List returns every configured host.
func (s *Service) List(ctx context.Context) ([]domain.Host, error) {
	hosts, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("hosts: list: %w", err)
	}
	return hosts, nil
}

// Delete removes a host permanently, along with its stored SSH key.
func (s *Service) Delete(ctx context.Context, id string) error {
	host, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("hosts: delete: %w", err)
	}
	if host.SSHKeySecretRef != "" {
		_ = s.secrets.Delete(host.SSHKeySecretRef) // best-effort cleanup
	}
	return nil
}

// CheckConnection dials id's host, pins its host key fingerprint on first successful connect, and
// refuses to proceed if a later connect presents a different fingerprint (docs/SPEC.md §12.4 — this
// is the one place in the codebase that requires real host-key verification rather than the TOFU
// exemption docs/KNOWN_GOTCHAS.md documents for the two existing read-only SSH adapters). On
// success, updates DockerAvailable/LastConnectedAt/LastStatus; on a mismatch, LastStatus becomes
// StatusError and the fingerprint is left untouched.
func (s *Service) CheckConnection(ctx context.Context, id string) (domain.Host, error) {
	host, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Host{}, err
	}
	if s.prober == nil {
		return domain.Host{}, fmt.Errorf("hosts: no connection prober configured")
	}
	keyPEM, err := s.secrets.Get(host.SSHKeySecretRef)
	if err != nil {
		return domain.Host{}, fmt.Errorf("hosts: resolve ssh key: %w", err)
	}

	result, err := s.prober.Probe(ctx, host.Address, host.SSHPort, host.SSHUser, keyPEM)
	if err != nil {
		host.LastStatus = domain.StatusUnreachable
		_ = s.repo.Update(ctx, host)
		return domain.Host{}, fmt.Errorf("hosts: probe %s: %w", host.Address, err)
	}

	if host.HostKeyFingerprint == "" {
		host.HostKeyFingerprint = result.HostKeyFingerprint
	} else if host.HostKeyFingerprint != result.HostKeyFingerprint {
		host.LastStatus = domain.StatusError
		_ = s.repo.Update(ctx, host)
		return domain.Host{}, domain.ErrHostKeyMismatch
	}

	host.DockerAvailable = result.DockerAvailable
	connectedAt := s.now()
	host.LastConnectedAt = &connectedAt
	host.LastStatus = domain.StatusOK
	if err := s.repo.Update(ctx, host); err != nil {
		return domain.Host{}, fmt.Errorf("hosts: update after probe: %w", err)
	}
	return host, nil
}
