package application_test

import (
	"context"
	"errors"
	"testing"

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
