// Package application holds the Source use-cases: Create, Update, Enable, Disable, Delete, List.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sre-kit/internal/platform/apierror"
	"sre-kit/internal/sources/domain"
)

// ChangeHook is invoked after a source is created, updated (including enable/disable), or
// deleted, so the composition root can keep the adapter engine's scheduler in sync with source
// state. A func type, not an interface — sources/application doesn't import adapterengine
// directly (ports, not direct imports; mirrors adapterengine's own SourceDisabler).
type ChangeHook func(ctx context.Context, source domain.Source, deleted bool)

// Service implements the Source use-cases against a domain.Repository.
type Service struct {
	repo     domain.Repository
	onChange ChangeHook
	now      func() time.Time // overridable in tests
}

// NewService wires a Service to its repository port.
func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// OnChange registers hook to run after every successful Create/Update/Delete. Returns Service for
// chaining at the composition root.
func (s *Service) OnChange(hook ChangeHook) *Service {
	s.onChange = hook
	return s
}

func (s *Service) notifyChange(ctx context.Context, source domain.Source, deleted bool) {
	if s.onChange != nil {
		s.onChange(ctx, source, deleted)
	}
}

// Create validates adapterName/configJSON, assigns a new UUID, and persists a new Source with
// Enabled=true and LastStatus="unreachable" until the adapter engine reports otherwise.
func (s *Service) Create(ctx context.Context, adapterName string, configJSON string) (domain.Source, error) {
	if adapterName == "" {
		return domain.Source{}, apierror.Invalid("adapter_id is required")
	}
	if configJSON == "" {
		configJSON = "{}"
	}
	if !json.Valid([]byte(configJSON)) {
		return domain.Source{}, apierror.Invalid("config must be valid JSON")
	}

	source := domain.Source{
		ID:          uuid.NewString(),
		AdapterName: adapterName,
		ConfigJSON:  configJSON,
		Enabled:     true,
		LastStatus:  domain.StatusUnreachable,
	}
	if err := s.repo.Create(ctx, source); err != nil {
		return domain.Source{}, fmt.Errorf("sources: create: %w", err)
	}
	s.notifyChange(ctx, source, false)
	return source, nil
}

// Update patches an existing source's config, enabled state, or both. Passing nil for a field
// leaves it unchanged.
func (s *Service) Update(ctx context.Context, id string, configJSON *string, enabled *bool) (domain.Source, error) {
	source, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Source{}, err
	}
	if configJSON != nil {
		if !json.Valid([]byte(*configJSON)) {
			return domain.Source{}, apierror.Invalid("config must be valid JSON")
		}
		source.ConfigJSON = *configJSON
	}
	if enabled != nil {
		source.Enabled = *enabled
	}
	if err := s.repo.Update(ctx, source); err != nil {
		return domain.Source{}, fmt.Errorf("sources: update: %w", err)
	}
	s.notifyChange(ctx, source, false)
	return source, nil
}

// Enable sets Enabled=true on the source identified by id.
func (s *Service) Enable(ctx context.Context, id string) (domain.Source, error) {
	enabled := true
	return s.Update(ctx, id, nil, &enabled)
}

// Disable sets Enabled=false on the source identified by id.
func (s *Service) Disable(ctx context.Context, id string) (domain.Source, error) {
	enabled := false
	return s.Update(ctx, id, nil, &enabled)
}

// Delete removes a source permanently.
func (s *Service) Delete(ctx context.Context, id string) error {
	source, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("sources: delete: %w", err)
	}
	s.notifyChange(ctx, source, true)
	return nil
}

// MarkSeen updates id's last_seen_at to now and, when status is non-empty, its last_status.
// Deliberately bypasses OnChange — called on every telemetry ingest (internal/telemetry/
// application.Service), so re-triggering the adapter engine's schedule-reconcile hook on every
// check/metric/event would be wasteful and pointless (config/enabled state hasn't changed).
// Signature intentionally uses only stdlib types so it structurally satisfies
// telemetry/application.SourceStatusUpdater without either package importing the other.
func (s *Service) MarkSeen(ctx context.Context, id string, status string) error {
	source, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	seenAt := s.now()
	source.LastSeenAt = &seenAt
	if status != "" {
		source.LastStatus = status
	}
	if err := s.repo.Update(ctx, source); err != nil {
		return fmt.Errorf("sources: mark seen: %w", err)
	}
	return nil
}

// List returns every configured source.
func (s *Service) List(ctx context.Context) ([]domain.Source, error) {
	sources, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("sources: list: %w", err)
	}
	return sources, nil
}
