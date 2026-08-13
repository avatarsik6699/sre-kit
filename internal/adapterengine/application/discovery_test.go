package application_test

import (
	"os"
	"path/filepath"
	"testing"

	"sre-kit/internal/adapterengine/application"
)

func TestListInstalled_MissingDirReturnsEmpty(t *testing.T) {
	got, err := application.ListInstalled(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d adapters, want 0", len(got))
	}
}

func TestListInstalled_ParsesValidManifest(t *testing.T) {
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, "stub")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifest := `{"name":"stub","version":"0.1.0","mode":"pull","emits":["metric"],"config_schema":{}}`
	if err := os.WriteFile(filepath.Join(adapterDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := application.ListInstalled(dir)
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(got) != 1 || got[0].Manifest.Name != "stub" {
		t.Fatalf("got %+v, want one adapter named stub", got)
	}
}

func TestListInstalled_SkipsDirsWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "not-an-adapter"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got, err := application.ListInstalled(dir)
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d adapters, want 0", len(got))
	}
}

func TestListInstalled_RejectsInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, "broken")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adapterDir, "manifest.json"), []byte(`{"mode":"pull"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := application.ListInstalled(dir); err == nil {
		t.Fatal("ListInstalled with a nameless manifest: want error, got nil")
	}
}
