package application_test

import (
	"context"
	"errors"
	"testing"

	"sre-kit/internal/hosts/application"
	"sre-kit/internal/hosts/domain"
)

type fakeRepo struct {
	hosts map[string]domain.Host
}

func newFakeRepo() *fakeRepo { return &fakeRepo{hosts: map[string]domain.Host{}} }

func (f *fakeRepo) Create(_ context.Context, host domain.Host) error {
	f.hosts[host.ID] = host
	return nil
}

func (f *fakeRepo) Update(_ context.Context, host domain.Host) error {
	if _, ok := f.hosts[host.ID]; !ok {
		return domain.ErrNotFound
	}
	f.hosts[host.ID] = host
	return nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (domain.Host, error) {
	host, ok := f.hosts[id]
	if !ok {
		return domain.Host{}, domain.ErrNotFound
	}
	return host, nil
}

func (f *fakeRepo) List(_ context.Context) ([]domain.Host, error) {
	var all []domain.Host
	for _, host := range f.hosts {
		all = append(all, host)
	}
	return all, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := f.hosts[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.hosts, id)
	return nil
}

type fakeSecrets struct {
	values map[string]string
}

func newFakeSecrets() *fakeSecrets { return &fakeSecrets{values: map[string]string{}} }

func (f *fakeSecrets) Put(value string) (string, error) {
	ref := "ref-" + value
	f.values[ref] = value
	return ref, nil
}

func (f *fakeSecrets) Get(ref string) (string, error) {
	value, ok := f.values[ref]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (f *fakeSecrets) Delete(ref string) error {
	delete(f.values, ref)
	return nil
}

type fakeProber struct {
	result application.ProbeResult
	err    error
	calls  int
}

func (f *fakeProber) Probe(_ context.Context, _ string, _ int, _ string, _ string) (application.ProbeResult, error) {
	f.calls++
	return f.result, f.err
}

func TestCreate_StoresKeyAsSecretRefAndDefaults(t *testing.T) {
	secrets := newFakeSecrets()
	svc := application.NewService(newFakeRepo(), secrets)
	host, err := svc.Create(context.Background(), "prod-vps", "1.2.3.4", 0, "operator", "PEM-DATA")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if host.ID == "" {
		t.Fatal("Create: expected a generated ID")
	}
	if host.SSHPort != 22 {
		t.Fatalf("Create: SSHPort = %d, want default 22", host.SSHPort)
	}
	if host.SSHKeySecretRef == "" || host.SSHKeySecretRef == "PEM-DATA" {
		t.Fatalf("Create: expected the key to be stored as a secret_ref, got %q", host.SSHKeySecretRef)
	}
	if got, _ := secrets.Get(host.SSHKeySecretRef); got != "PEM-DATA" {
		t.Fatalf("Create: secret store holds %q, want the plaintext key", got)
	}
	if host.LastStatus != domain.StatusUnreachable {
		t.Fatalf("Create: LastStatus = %q, want %q", host.LastStatus, domain.StatusUnreachable)
	}
}

func TestCreate_RejectsMissingFields(t *testing.T) {
	svc := application.NewService(newFakeRepo(), newFakeSecrets())
	cases := []struct {
		name, address, user, key string
	}{
		{"no address", "", "operator", "PEM"},
		{"no user", "1.2.3.4", "", "PEM"},
		{"no key", "1.2.3.4", "operator", ""},
	}
	for _, c := range cases {
		if _, err := svc.Create(context.Background(), "label", c.address, 22, c.user, c.key); err == nil {
			t.Fatalf("%s: want error, got nil", c.name)
		}
	}
}

func TestCheckConnection_PinsFingerprintOnFirstConnect(t *testing.T) {
	secrets := newFakeSecrets()
	prober := &fakeProber{result: application.ProbeResult{HostKeyFingerprint: "SHA256:abc", DockerAvailable: true}}
	svc := application.NewService(newFakeRepo(), secrets, application.WithProber(prober))

	host, err := svc.Create(context.Background(), "", "1.2.3.4", 22, "operator", "PEM")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.CheckConnection(context.Background(), host.ID)
	if err != nil {
		t.Fatalf("CheckConnection: %v", err)
	}
	if updated.HostKeyFingerprint != "SHA256:abc" {
		t.Fatalf("HostKeyFingerprint = %q, want pinned value", updated.HostKeyFingerprint)
	}
	if !updated.DockerAvailable {
		t.Fatal("DockerAvailable = false, want true")
	}
	if updated.LastStatus != domain.StatusOK {
		t.Fatalf("LastStatus = %q, want %q", updated.LastStatus, domain.StatusOK)
	}
	if updated.LastConnectedAt == nil {
		t.Fatal("LastConnectedAt not set")
	}
}

func TestCheckConnection_RefusesFingerprintMismatch(t *testing.T) {
	secrets := newFakeSecrets()
	prober := &fakeProber{result: application.ProbeResult{HostKeyFingerprint: "SHA256:abc"}}
	svc := application.NewService(newFakeRepo(), secrets, application.WithProber(prober))

	host, err := svc.Create(context.Background(), "", "1.2.3.4", 22, "operator", "PEM")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.CheckConnection(context.Background(), host.ID); err != nil {
		t.Fatalf("first CheckConnection: %v", err)
	}

	prober.result = application.ProbeResult{HostKeyFingerprint: "SHA256:different"}
	_, err = svc.CheckConnection(context.Background(), host.ID)
	if !errors.Is(err, domain.ErrHostKeyMismatch) {
		t.Fatalf("second CheckConnection: err = %v, want ErrHostKeyMismatch", err)
	}
}

func TestDelete_RemovesStoredKey(t *testing.T) {
	secrets := newFakeSecrets()
	svc := application.NewService(newFakeRepo(), secrets)
	host, err := svc.Create(context.Background(), "", "1.2.3.4", 22, "operator", "PEM")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Delete(context.Background(), host.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := secrets.Get(host.SSHKeySecretRef); err == nil {
		t.Fatal("Delete: expected the stored key to be removed")
	}
}
