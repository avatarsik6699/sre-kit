package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	adapterenginehttp "sre-kit/internal/adapterengine/interfaces/http"
)

func TestListAdapters_ReturnsInstalledManifests(t *testing.T) {
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, "stub")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifest := `{"name":"stub","version":"0.1.0","mode":"pull","emits":["metric"],"config_schema":{}}`
	if err := os.WriteFile(filepath.Join(adapterDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mux := http.NewServeMux()
	adapterenginehttp.NewHandlers(dir).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/adapters", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0]["name"] != "stub" {
		t.Fatalf("got %+v, want one adapter named stub", got)
	}
}
