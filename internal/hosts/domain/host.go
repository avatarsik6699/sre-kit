// Package domain holds the Host entity, its repository port, and its typed domain errors.
// Per docs/SPEC.md §12: a Host is an SSH-reachable machine sre-kit is authorized to deploy
// Docker-based observability tools to. Deliberately separate from internal/sources — a Host is
// infrastructure internal/provisioner mutates, not part of the read-only Source/Metric/Check/Event
// telemetry chain.
package domain

import (
	"context"
	"time"

	"sre-kit/internal/platform/apierror"
)

// Status values for Host.LastStatus — same vocabulary as sources.LastStatus (docs/SPEC.md §3).
const (
	StatusOK          = "ok"
	StatusUnreachable = "unreachable"
	StatusError       = "error"
)

// Host is a configured SSH+Docker deploy target (docs/SPEC.md §3 hosts table). ID is a
// core-generated UUID, same rationale as sources.ID.
type Host struct {
	ID                 string
	Label              string
	Address            string
	SSHPort            int
	SSHUser            string
	SSHKeySecretRef    string
	HostKeyFingerprint string // pinned on first successful connect; empty until then
	DockerAvailable    bool
	LastConnectedAt    *time.Time
	LastStatus         string
	CreatedAt          time.Time
}

// ErrNotFound is returned when a lookup by ID finds no matching host.
var ErrNotFound = apierror.NotFound("host not found")

// ErrHostKeyMismatch is returned by CheckConnection when a host presents a different key than the
// one pinned on first connect — the provisioner refuses to proceed rather than silently trust a
// possibly-MITM'd connection (docs/SPEC.md §12.4).
var ErrHostKeyMismatch = apierror.Conflict("host key does not match the fingerprint pinned on first connect")

// Repository is the persistence port for Host, implemented by internal/hosts/infrastructure.
type Repository interface {
	Create(ctx context.Context, host Host) error
	Update(ctx context.Context, host Host) error
	Get(ctx context.Context, id string) (Host, error)
	List(ctx context.Context) ([]Host, error)
	Delete(ctx context.Context, id string) error
}
