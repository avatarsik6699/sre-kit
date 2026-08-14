package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"sre-kit/internal/sources/application"
	"sre-kit/internal/sources/domain"
)

type fakeRepo struct {
	sources map[string]domain.Source
}

func newFakeRepo() *fakeRepo { return &fakeRepo{sources: map[string]domain.Source{}} }

func (f *fakeRepo) Create(_ context.Context, source domain.Source) error {
	f.sources[source.ID] = source
	return nil
}

func (f *fakeRepo) Update(_ context.Context, source domain.Source) error {
	if _, ok := f.sources[source.ID]; !ok {
		return domain.ErrNotFound
	}
	f.sources[source.ID] = source
	return nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (domain.Source, error) {
	source, ok := f.sources[id]
	if !ok {
		return domain.Source{}, domain.ErrNotFound
	}
	return source, nil
}

func (f *fakeRepo) List(_ context.Context) ([]domain.Source, error) {
	var all []domain.Source
	for _, source := range f.sources {
		all = append(all, source)
	}
	return all, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := f.sources[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.sources, id)
	return nil
}

func TestCreate_AssignsIDAndDefaults(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	source, err := svc.Create(context.Background(), "host-metrics-ssh", `{"host":"1.2.3.4"}`)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if source.ID == "" {
		t.Fatal("Create: expected a generated ID")
	}
	if !source.Enabled {
		t.Fatal("Create: expected Enabled=true by default")
	}
	if source.LastStatus != domain.StatusUnreachable {
		t.Fatalf("Create: LastStatus = %q, want %q", source.LastStatus, domain.StatusUnreachable)
	}
}

func TestCreate_RejectsEmptyAdapterID(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	if _, err := svc.Create(context.Background(), "", "{}"); err == nil {
		t.Fatal("Create with empty adapter_id: want error, got nil")
	}
}

func TestCreate_RejectsInvalidConfigJSON(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	if _, err := svc.Create(context.Background(), "host-metrics-ssh", `{not json`); err == nil {
		t.Fatal("Create with invalid config JSON: want error, got nil")
	}
}

func TestDisableThenEnable(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	source, err := svc.Create(context.Background(), "uptime-http", "{}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	disabled, err := svc.Disable(context.Background(), source.ID)
	if err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if disabled.Enabled {
		t.Fatal("Disable: expected Enabled=false")
	}

	enabled, err := svc.Enable(context.Background(), source.ID)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("Enable: expected Enabled=true")
	}
}

func TestDelete_UnknownIDReturnsErrNotFound(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	err := svc.Delete(context.Background(), "does-not-exist")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Delete unknown id: got %v, want domain.ErrNotFound", err)
	}
}

func TestOnChange_FiresOnCreateUpdateAndDelete(t *testing.T) {
	type event struct {
		sourceID string
		deleted  bool
	}
	var events []event
	svc := application.NewService(newFakeRepo()).OnChange(func(_ context.Context, source domain.Source, deleted bool) {
		events = append(events, event{sourceID: source.ID, deleted: deleted})
	})

	source, err := svc.Create(context.Background(), "host-metrics-ssh", "{}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Disable(context.Background(), source.ID); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if err := svc.Delete(context.Background(), source.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	want := []event{
		{sourceID: source.ID, deleted: false}, // Create
		{sourceID: source.ID, deleted: false}, // Disable (still an Update, not a delete)
		{sourceID: source.ID, deleted: true},  // Delete
	}
	if len(events) != len(want) {
		t.Fatalf("got %d OnChange events, want %d: %+v", len(events), len(want), events)
	}
	for i, got := range events {
		if got != want[i] {
			t.Fatalf("event %d = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestOnChange_NotCalledWhenMutationFails(t *testing.T) {
	called := false
	svc := application.NewService(newFakeRepo()).OnChange(func(context.Context, domain.Source, bool) {
		called = true
	})

	if _, err := svc.Create(context.Background(), "", "{}"); err == nil {
		t.Fatal("Create with empty adapter_id: want error, got nil")
	}
	if called {
		t.Fatal("OnChange fired despite Create failing validation")
	}
}

func TestMarkSeen_UpdatesStatusAndSeenAt(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	fixedNow := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	application.SetClockForTest(svc, func() time.Time { return fixedNow })

	source, err := svc.Create(context.Background(), "uptime-http", "{}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if source.LastStatus != domain.StatusUnreachable || source.LastSeenAt != nil {
		t.Fatalf("Create: want default unreachable/nil, got status=%q seenAt=%v", source.LastStatus, source.LastSeenAt)
	}

	if err := svc.MarkSeen(context.Background(), source.ID, domain.StatusOK); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}

	sources, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("List: got %d sources, want 1", len(sources))
	}
	got := sources[0]
	if got.LastStatus != domain.StatusOK {
		t.Fatalf("LastStatus = %q, want %q", got.LastStatus, domain.StatusOK)
	}
	if got.LastSeenAt == nil || !got.LastSeenAt.Equal(fixedNow) {
		t.Fatalf("LastSeenAt = %v, want %v", got.LastSeenAt, fixedNow)
	}
}

func TestMarkSeen_EmptyStatusOnlyTouchesSeenAt(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	source, err := svc.Create(context.Background(), "uptime-http", "{}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.MarkSeen(context.Background(), source.ID, ""); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}

	sources, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := sources[0]
	if got.LastStatus != domain.StatusUnreachable {
		t.Fatalf("LastStatus = %q, want unchanged %q", got.LastStatus, domain.StatusUnreachable)
	}
	if got.LastSeenAt == nil {
		t.Fatal("LastSeenAt: want non-nil after MarkSeen")
	}
}

func TestMarkSeen_DoesNotFireOnChange(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	source, err := svc.Create(context.Background(), "uptime-http", "{}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	called := false
	svc.OnChange(func(context.Context, domain.Source, bool) { called = true })

	if err := svc.MarkSeen(context.Background(), source.ID, domain.StatusOK); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}
	if called {
		t.Fatal("OnChange fired on MarkSeen — would needlessly re-trigger scheduler reconciliation on every telemetry ingest")
	}
}

func TestMarkSeen_UnknownIDReturnsErrNotFound(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	if err := svc.MarkSeen(context.Background(), "does-not-exist", domain.StatusOK); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("MarkSeen unknown id: got %v, want ErrNotFound", err)
	}
}

// --- secret_ref resolution (docs/changes/07-source-secret-ref-fix.md) ---

type fakeSecretsStore struct {
	nextID      int
	values      map[string]string
	putCalls    []string
	deleteCalls []string
}

func newFakeSecretsStore() *fakeSecretsStore {
	return &fakeSecretsStore{values: map[string]string{}}
}

func (f *fakeSecretsStore) Put(value string) (string, error) {
	f.nextID++
	ref := fmt.Sprintf("ref-%d", f.nextID)
	f.values[ref] = value
	f.putCalls = append(f.putCalls, value)
	return ref, nil
}

func (f *fakeSecretsStore) Delete(ref string) error {
	delete(f.values, ref)
	f.deleteCalls = append(f.deleteCalls, ref)
	return nil
}

var hostMetricsSSHSchema = json.RawMessage(`{
	"properties": {
		"host": {"type": "string"},
		"secret": {"type": "string", "format": "secret"}
	}
}`)

func fixedSchemaLookup(schema json.RawMessage, err error) application.AdapterConfigSchemaLookup {
	return func(context.Context, string) (json.RawMessage, error) { return schema, err }
}

func configField(t *testing.T, configJSON, field string) string {
	t.Helper()
	var config map[string]string
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		t.Fatalf("parse config %q: %v", configJSON, err)
	}
	return config[field]
}

func TestCreate_ResolvesSecretFieldToRef(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(newFakeRepo(),
		application.WithSecrets(store),
		application.WithAdapterConfigSchemas(fixedSchemaLookup(hostMetricsSSHSchema, nil)),
	)

	source, err := svc.Create(context.Background(), "host-metrics-ssh", `{"host":"1.2.3.4","secret":"hunter2"}`)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(store.putCalls) != 1 || store.putCalls[0] != "hunter2" {
		t.Fatalf("Put calls = %+v, want one call with the plaintext secret", store.putCalls)
	}
	ref := configField(t, source.ConfigJSON, "secret")
	if ref == "hunter2" || ref == "" {
		t.Fatalf("stored config secret field = %q, want a ref, not the plaintext value", ref)
	}
	if configField(t, source.ConfigJSON, "host") != "1.2.3.4" {
		t.Fatalf("stored config host field changed unexpectedly: %s", source.ConfigJSON)
	}
}

func TestCreate_UnknownAdapterLeavesConfigUnchanged(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(newFakeRepo(),
		application.WithSecrets(store),
		application.WithAdapterConfigSchemas(fixedSchemaLookup(nil, fmt.Errorf("adapter %q not installed", "host-metrics-ssh"))),
	)

	source, err := svc.Create(context.Background(), "host-metrics-ssh", `{"secret":"hunter2"}`)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(store.putCalls) != 0 {
		t.Fatalf("Put calls = %+v, want none for an unresolvable adapter schema", store.putCalls)
	}
	if configField(t, source.ConfigJSON, "secret") != "hunter2" {
		t.Fatalf("config = %s, want the plaintext value left untouched", source.ConfigJSON)
	}
}

func TestCreate_NoSecretsWiredLeavesConfigUnchanged(t *testing.T) {
	svc := application.NewService(newFakeRepo())
	source, err := svc.Create(context.Background(), "host-metrics-ssh", `{"secret":"hunter2"}`)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if configField(t, source.ConfigJSON, "secret") != "hunter2" {
		t.Fatalf("config = %s, want the plaintext value left untouched when no SecretsStore is wired", source.ConfigJSON)
	}
}

func TestUpdate_UnchangedSecretFieldIsNotRewrapped(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(newFakeRepo(),
		application.WithSecrets(store),
		application.WithAdapterConfigSchemas(fixedSchemaLookup(hostMetricsSSHSchema, nil)),
	)

	source, err := svc.Create(context.Background(), "host-metrics-ssh", `{"host":"1.2.3.4","secret":"hunter2"}`)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(store.putCalls) != 1 {
		t.Fatalf("Put calls after Create = %+v, want exactly one", store.putCalls)
	}

	// Resubmit the config exactly as returned (the ref, not a new plaintext value) — as an edit
	// flow that round-trips an unmodified field would.
	updated, err := svc.Update(context.Background(), source.ID, &source.ConfigJSON, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(store.putCalls) != 1 {
		t.Fatalf("Put calls after no-op Update = %+v, want still exactly one (unchanged field must not be re-wrapped)", store.putCalls)
	}
	if len(store.deleteCalls) != 0 {
		t.Fatalf("Delete calls = %+v, want none for an unchanged field", store.deleteCalls)
	}
	if configField(t, updated.ConfigJSON, "secret") != configField(t, source.ConfigJSON, "secret") {
		t.Fatalf("ref changed on a no-op update: %s -> %s", source.ConfigJSON, updated.ConfigJSON)
	}
}

func TestUpdate_ChangedSecretFieldRotatesRefAndDeletesOld(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(newFakeRepo(),
		application.WithSecrets(store),
		application.WithAdapterConfigSchemas(fixedSchemaLookup(hostMetricsSSHSchema, nil)),
	)

	source, err := svc.Create(context.Background(), "host-metrics-ssh", `{"host":"1.2.3.4","secret":"hunter2"}`)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	oldRef := configField(t, source.ConfigJSON, "secret")

	newConfig := `{"host":"1.2.3.4","secret":"new-password"}`
	updated, err := svc.Update(context.Background(), source.ID, &newConfig, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(store.putCalls) != 2 || store.putCalls[1] != "new-password" {
		t.Fatalf("Put calls = %+v, want a second call storing the new plaintext value", store.putCalls)
	}
	newRef := configField(t, updated.ConfigJSON, "secret")
	if newRef == oldRef || newRef == "new-password" {
		t.Fatalf("new ref = %q, want a fresh ref distinct from %q and the plaintext value", newRef, oldRef)
	}
	if len(store.deleteCalls) != 1 || store.deleteCalls[0] != oldRef {
		t.Fatalf("Delete calls = %+v, want exactly one deleting the superseded ref %q", store.deleteCalls, oldRef)
	}
}
