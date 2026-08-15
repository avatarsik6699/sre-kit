// Package domain holds the ProvisioningRun workflow record and the Preset shape, per
// docs/SPEC.md §12. A Run is a job/workflow record, not a CRUD entity like Host/Source —
// deliberately separate from internal/hosts so a retry can resume from Step instead of restarting.
package domain

import (
	"context"
	"time"

	"sre-kit/internal/platform/apierror"
)

// Status values for Run.Status, per docs/SPEC.md §3/§12.2.
const (
	StatusPending       = "pending"
	StatusDeploying     = "deploying"
	StatusBootstrapping = "bootstrapping"
	StatusRegistering   = "registering"
	StatusDone          = "done"
	StatusFailed        = "failed"
)

// Step values for Run.Step — the last successfully completed step, so Retry knows where to resume
// (docs/SPEC.md §12.2/§12.4's partial-failure requirement).
const (
	StepDeploy    = "deploy"
	StepBootstrap = "bootstrap"
	StepRegister  = "register"
)

// Run is one provisioning workflow execution: Host + preset -> deployed stack -> bootstrapped
// admin account -> registered Source.
type Run struct {
	ID                     string
	HostID                 string
	PresetName             string
	Status                 string
	Step                   string
	ErrorMessage           string
	AdminPasswordSecretRef string
	ProducedSourceID       string
	StartedAt              time.Time
	FinishedAt             *time.Time
}

// ErrNotFound is returned when a lookup by ID finds no matching run.
var ErrNotFound = apierror.NotFound("provisioning run not found")

// ErrNotRetryable is returned by Retry when the target run isn't in a failed state.
var ErrNotRetryable = apierror.Invalid("only a failed provisioning run can be retried")

// Repository is the persistence port for Run, implemented by internal/provisioner/infrastructure.
type Repository interface {
	Create(ctx context.Context, run Run) error
	Update(ctx context.Context, run Run) error
	Get(ctx context.Context, id string) (Run, error)
	ListByHost(ctx context.Context, hostID string) ([]Run, error)
}
