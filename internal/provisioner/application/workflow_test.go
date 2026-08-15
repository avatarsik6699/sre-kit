package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sre-kit/internal/provisioner/application"
	"sre-kit/internal/provisioner/domain"
)

type fakeRunRepo struct{ runs map[string]domain.Run }

func newFakeRunRepo() *fakeRunRepo { return &fakeRunRepo{runs: map[string]domain.Run{}} }

func (f *fakeRunRepo) Create(_ context.Context, run domain.Run) error {
	f.runs[run.ID] = run
	return nil
}

func (f *fakeRunRepo) Update(_ context.Context, run domain.Run) error {
	f.runs[run.ID] = run
	return nil
}

func (f *fakeRunRepo) Get(_ context.Context, id string) (domain.Run, error) {
	run, ok := f.runs[id]
	if !ok {
		return domain.Run{}, domain.ErrNotFound
	}
	return run, nil
}

func (f *fakeRunRepo) ListByHost(_ context.Context, hostID string) ([]domain.Run, error) {
	var out []domain.Run
	for _, run := range f.runs {
		if run.HostID == hostID {
			out = append(out, run)
		}
	}
	return out, nil
}

func fakeHosts(conn application.HostConn) application.HostsLookup {
	return func(_ context.Context, _ string) (application.HostConn, error) {
		return conn, nil
	}
}

type fakeSSH struct {
	runCalls []string
	uploaded map[string]string
}

func (f *fakeSSH) RunCommand(_ context.Context, _ application.HostConn, command string) (string, error) {
	f.runCalls = append(f.runCalls, command)
	return "", nil
}

func (f *fakeSSH) UploadFile(_ context.Context, _ application.HostConn, path string, content []byte) error {
	if f.uploaded == nil {
		f.uploaded = map[string]string{}
	}
	f.uploaded[path] = string(content)
	return nil
}

type fakeSecrets struct{ values map[string]string }

func newFakeSecrets() *fakeSecrets { return &fakeSecrets{values: map[string]string{}} }

func (f *fakeSecrets) Put(value string) (string, error) {
	ref := fmt.Sprintf("ref-%d", len(f.values))
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

type sourceCreateCall struct{ Adapter, Config string }

type fakeSourceCreator struct {
	calls    []sourceCreateCall
	failOnce bool
	failed   bool
}

func (f *fakeSourceCreator) Create(_ context.Context, adapterName, configJSON string) (string, error) {
	f.calls = append(f.calls, sourceCreateCall{adapterName, configJSON})
	if f.failOnce && !f.failed {
		f.failed = true
		return "", errors.New("register: simulated failure")
	}
	return "src-1", nil
}

// writePreset writes a minimal preset fixture (manifest.json, docker-compose.yml.tmpl,
// bootstrap.json) under dir/name/, mirroring the on-disk shape application.LoadPreset expects.
func writePreset(t *testing.T, presetsDir, name string, manifest domain.Manifest, compose string, steps []domain.BootstrapStep) {
	t.Helper()
	dir := filepath.Join(presetsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	manifest.Name = name
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestRaw, 0o644); err != nil {
		t.Fatalf("write manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml.tmpl"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write docker-compose.yml.tmpl: %v", err)
	}
	bootstrapRaw, err := json.Marshal(map[string]any{"steps": steps})
	if err != nil {
		t.Fatalf("marshal bootstrap: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bootstrap.json"), bootstrapRaw, 0o644); err != nil {
		t.Fatalf("write bootstrap.json: %v", err)
	}
}

func TestStart_SSHCommandBootstrap_RegistersSource(t *testing.T) {
	presetsDir := t.TempDir()
	writePreset(t, presetsDir, "beszel", domain.Manifest{
		ProducesAdapter: "beszel-api",
		ProducesSourceConfigTemplate: map[string]string{
			"base_url": "http://{{.HostAddress}}:8090",
			"password": "{{.AdminPassword}}",
		},
	}, "services:\n  beszel:\n    image: henrygd/beszel\n", []domain.BootstrapStep{
		{Type: domain.BootstrapSSHCommand, Command: "docker compose exec -T beszel beszel superuser upsert {{.AdminEmail}} {{.AdminPassword}}"},
	})

	ssh := &fakeSSH{}
	sourceCreator := &fakeSourceCreator{}
	svc := application.NewService(newFakeRunRepo(), fakeHosts(application.HostConn{Address: "1.2.3.4"}), ssh, newFakeSecrets(), sourceCreator.Create, presetsDir)

	run, err := svc.Start(context.Background(), "host-1", "beszel")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if run.Status != domain.StatusDone {
		t.Fatalf("Status = %q, want %q (error: %s)", run.Status, domain.StatusDone, run.ErrorMessage)
	}
	if run.ProducedSourceID != "src-1" {
		t.Fatalf("ProducedSourceID = %q, want src-1", run.ProducedSourceID)
	}
	if len(ssh.runCalls) != 2 {
		t.Fatalf("ssh RunCommand calls = %d, want 2 (deploy + bootstrap)", len(ssh.runCalls))
	}
	if len(sourceCreator.calls) != 1 || sourceCreator.calls[0].Adapter != "beszel-api" {
		t.Fatalf("sourceCreator calls = %+v", sourceCreator.calls)
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(sourceCreator.calls[0].Config), &config); err != nil {
		t.Fatalf("unmarshal produced config: %v", err)
	}
	if config["base_url"] != "http://1.2.3.4:8090" {
		t.Fatalf("base_url = %q", config["base_url"])
	}
	if config["password"] == "" {
		t.Fatal("password: expected a generated plaintext password in the produced source config")
	}
}

func TestStart_HTTPCallBootstrap_CapturesTokenAndChangesPassword(t *testing.T) {
	var loginCalls, changeCalls int
	var capturedAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			loginCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"tok-abc"}`))
		case "/api/auth/password":
			changeCalls++
			capturedAuthHeader = r.Header.Get("Authorization")
			body, _ := io.ReadAll(r.Body)
			if !json.Valid(body) {
				t.Errorf("change-password body is not valid JSON: %s", body)
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	presetsDir := t.TempDir()
	writePreset(t, presetsDir, "umami", domain.Manifest{
		ProducesAdapter: "umami-http",
		BaseURLTemplate: server.URL,
		ProducesSourceConfigTemplate: map[string]string{
			"base_url": server.URL,
			"password": "{{.AdminPassword}}",
		},
	}, "services:\n  umami:\n    image: ghcr.io/umami-software/umami\n", []domain.BootstrapStep{
		{
			Type:         domain.BootstrapHTTPCall,
			Method:       http.MethodPost,
			PathTemplate: "/api/auth/login",
			BodyTemplate: `{"username":"admin","password":"umami"}`,
			CaptureField: "token",
			CaptureAs:    "token",
		},
		{
			Type:          domain.BootstrapHTTPCall,
			Method:        http.MethodPost,
			PathTemplate:  "/api/auth/password",
			BodyTemplate:  `{"currentPassword":"umami","newPassword":"{{.AdminPassword}}"}`,
			AuthFromField: "token",
		},
	})

	svc := application.NewService(newFakeRunRepo(), fakeHosts(application.HostConn{Address: "1.2.3.4"}), &fakeSSH{}, newFakeSecrets(), (&fakeSourceCreator{}).Create, presetsDir)

	run, err := svc.Start(context.Background(), "host-1", "umami")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if run.Status != domain.StatusDone {
		t.Fatalf("Status = %q, want %q (error: %s)", run.Status, domain.StatusDone, run.ErrorMessage)
	}
	if loginCalls != 1 || changeCalls != 1 {
		t.Fatalf("loginCalls=%d changeCalls=%d, want 1/1", loginCalls, changeCalls)
	}
	if capturedAuthHeader != "Bearer tok-abc" {
		t.Fatalf("Authorization header = %q, want captured token", capturedAuthHeader)
	}
}

func TestRetry_ResumesFromLastStep_DoesNotRerunBootstrap(t *testing.T) {
	presetsDir := t.TempDir()
	writePreset(t, presetsDir, "beszel", domain.Manifest{
		ProducesAdapter: "beszel-api",
		ProducesSourceConfigTemplate: map[string]string{
			"password": "{{.AdminPassword}}",
		},
	}, "services: {}\n", []domain.BootstrapStep{
		{Type: domain.BootstrapSSHCommand, Command: "bootstrap-command"},
	})

	ssh := &fakeSSH{}
	sourceCreator := &fakeSourceCreator{failOnce: true}
	svc := application.NewService(newFakeRunRepo(), fakeHosts(application.HostConn{Address: "1.2.3.4"}), ssh, newFakeSecrets(), sourceCreator.Create, presetsDir)

	run, err := svc.Start(context.Background(), "host-1", "beszel")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if run.Status != domain.StatusFailed {
		t.Fatalf("Status = %q, want %q", run.Status, domain.StatusFailed)
	}
	if run.Step != domain.StepBootstrap {
		t.Fatalf("Step = %q, want %q (bootstrap should have completed before registration failed)", run.Step, domain.StepBootstrap)
	}
	firstPasswordCall := len(sourceCreator.calls)
	if firstPasswordCall != 1 {
		t.Fatalf("sourceCreator calls before retry = %d, want 1", firstPasswordCall)
	}
	firstAttemptPassword := sourceCreator.calls[0].Config

	retried, err := svc.Retry(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("Retry: %v", err)
	}
	if retried.Status != domain.StatusDone {
		t.Fatalf("Retry Status = %q, want %q (error: %s)", retried.Status, domain.StatusDone, retried.ErrorMessage)
	}
	// Bootstrap must not re-run on retry — only the original run's 2 RunCommand calls (deploy +
	// bootstrap) should exist, none added by the retry.
	if len(ssh.runCalls) != 2 {
		t.Fatalf("ssh RunCommand calls after retry = %d, want 2 (no re-run of deploy/bootstrap)", len(ssh.runCalls))
	}
	if len(sourceCreator.calls) != 2 {
		t.Fatalf("sourceCreator calls after retry = %d, want 2", len(sourceCreator.calls))
	}
	if sourceCreator.calls[1].Config != firstAttemptPassword {
		t.Fatalf("retry used a different admin password than the original bootstrap: %q vs %q", sourceCreator.calls[1].Config, firstAttemptPassword)
	}
}
