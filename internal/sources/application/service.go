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

// SecretsStore is the narrow port onto internal/platform/secrets.Store this service needs to
// resolve config_schema "format": "secret" fields into refs before persisting — mirrors
// internal/alertrouter/application's own narrow secrets port (Put/Delete only), not a direct
// dependency on the secrets package.
type SecretsStore interface {
	Put(value string) (string, error)
	Delete(ref string) error
}

// AdapterConfigSchemaLookup resolves an installed adapter's config_schema by adapter name, so
// Create/Update know which config fields are secrets. Backed by adapterengine's ListInstalled at
// the composition root (cmd/server/main.go) — sources/application deliberately doesn't import
// adapterengine directly (ports, not direct imports; mirrors ChangeHook above).
type AdapterConfigSchemaLookup func(ctx context.Context, adapterName string) (json.RawMessage, error)

// Service implements the Source use-cases against a domain.Repository.
type Service struct {
	repo         domain.Repository
	onChange     ChangeHook
	now          func() time.Time // overridable in tests
	secrets      SecretsStore
	schemaLookup AdapterConfigSchemaLookup
}

// Option configures optional Service dependencies not required by every caller/test.
type Option func(*Service)

// WithSecrets wires the secrets store Create/Update use to resolve "format": "secret" config
// fields into refs. Without it, config is persisted as given (e.g. in tests that don't exercise
// secret handling).
func WithSecrets(store SecretsStore) Option {
	return func(s *Service) { s.secrets = store }
}

// WithAdapterConfigSchemas wires the lookup Create/Update use to find which config fields a given
// adapter marks as secrets.
func WithAdapterConfigSchemas(lookup AdapterConfigSchemaLookup) Option {
	return func(s *Service) { s.schemaLookup = lookup }
}

// NewService wires a Service to its repository port and any Options.
func NewService(repo domain.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// resolveSecrets rewrites configJSON so every config_schema field marked "format": "secret" holds
// a secret_ref instead of a plaintext value, per docs/SPEC.md §3 ("config_json only ever stores a
// secret_ref, never a plaintext value") — mirrors alertrouter's CreateChannel/UpdateChannel
// pattern (secrets.Put before persisting), generalized to any adapter's schema.
//
// oldConfigJSON is the source's currently-persisted config ("" on Create): a secret field whose
// incoming value is byte-identical to what's already stored there is left untouched — this is
// what stops a value that's already a ref (round-tripped by an edit flow that resends it
// unchanged) from being wrapped into a second ref pointing at the ref string itself. When a
// secret field's value does change, the previous value is deleted from the store if it was
// itself a ref (best-effort; a no-op if it wasn't).
//
// Both a nil SecretsStore/AdapterConfigSchemaLookup (not wired) and an unresolvable adapter name
// leave configJSON untouched rather than erroring — a source may reference an adapter that isn't
// installed yet (mirrors cmd/server/main.go's reconcileSchedule tolerating the same case), and
// that must not block creating/editing the source.
func (s *Service) resolveSecrets(ctx context.Context, adapterName, newConfigJSON, oldConfigJSON string) (string, error) {
	if s.secrets == nil || s.schemaLookup == nil {
		return newConfigJSON, nil
	}
	schema, err := s.schemaLookup(ctx, adapterName)
	if err != nil || len(schema) == 0 {
		return newConfigJSON, nil
	}
	var doc struct {
		Properties map[string]struct {
			Format string `json:"format"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return newConfigJSON, nil
	}

	var newConfig map[string]json.RawMessage
	if err := json.Unmarshal([]byte(newConfigJSON), &newConfig); err != nil {
		return "", fmt.Errorf("sources: parse config: %w", err)
	}
	var oldConfig map[string]json.RawMessage
	if oldConfigJSON != "" {
		_ = json.Unmarshal([]byte(oldConfigJSON), &oldConfig) // best-effort: malformed old config just means nothing looks "unchanged"
	}

	changed := false
	for name, prop := range doc.Properties {
		if prop.Format != "secret" {
			continue
		}
		raw, ok := newConfig[name]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil || value == "" {
			continue
		}
		oldRaw, hadOld := oldConfig[name]
		if hadOld && string(oldRaw) == string(raw) {
			continue // unchanged from what's already stored
		}

		ref, err := s.secrets.Put(value)
		if err != nil {
			return "", fmt.Errorf("sources: store secret field %q: %w", name, err)
		}
		encoded, err := json.Marshal(ref)
		if err != nil {
			return "", fmt.Errorf("sources: encode secret ref for field %q: %w", name, err)
		}
		if hadOld {
			var oldRef string
			if json.Unmarshal(oldRaw, &oldRef) == nil && oldRef != "" {
				_ = s.secrets.Delete(oldRef) // best-effort cleanup of the superseded ref
			}
		}
		newConfig[name] = encoded
		changed = true
	}
	if !changed {
		return newConfigJSON, nil
	}
	resolved, err := json.Marshal(newConfig)
	if err != nil {
		return "", fmt.Errorf("sources: encode resolved config: %w", err)
	}
	return string(resolved), nil
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
	resolvedConfig, err := s.resolveSecrets(ctx, adapterName, configJSON, "")
	if err != nil {
		return domain.Source{}, fmt.Errorf("sources: resolve secrets: %w", err)
	}

	source := domain.Source{
		ID:          uuid.NewString(),
		AdapterName: adapterName,
		ConfigJSON:  resolvedConfig,
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
		resolvedConfig, err := s.resolveSecrets(ctx, source.AdapterName, *configJSON, source.ConfigJSON)
		if err != nil {
			return domain.Source{}, fmt.Errorf("sources: resolve secrets: %w", err)
		}
		source.ConfigJSON = resolvedConfig
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
