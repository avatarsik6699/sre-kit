package secrets

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "secrets.enc.json"), "test-key")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return store
}

func TestResolveConfig_ReplacesSecretField(t *testing.T) {
	store := newTestStore(t)
	ref, err := store.Put("hunter2")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	schema := []byte(`{"type":"object","properties":{"host":{"type":"string"},"secret":{"type":"string","format":"secret"}}}`)
	configJSON := `{"host":"vps.example.com","secret":"` + ref + `"}`

	resolved, err := ResolveConfig(store, schema, configJSON)
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(resolved, &got); err != nil {
		t.Fatalf("unmarshal resolved config: %v", err)
	}
	if got["secret"] != "hunter2" {
		t.Fatalf("secret field = %q, want plaintext %q", got["secret"], "hunter2")
	}
	if got["host"] != "vps.example.com" {
		t.Fatalf("host field = %q, want unchanged %q", got["host"], "vps.example.com")
	}
}

func TestResolveConfig_NoSecretFields_PassesThrough(t *testing.T) {
	store := newTestStore(t)
	schema := []byte(`{"type":"object","properties":{"host":{"type":"string"}}}`)
	configJSON := `{"host":"vps.example.com"}`

	resolved, err := ResolveConfig(store, schema, configJSON)
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
	if string(resolved) != configJSON {
		t.Fatalf("resolved = %q, want unchanged %q", resolved, configJSON)
	}
}

func TestResolveConfig_EmptySchema_PassesThrough(t *testing.T) {
	store := newTestStore(t)
	configJSON := `{"host":"vps.example.com"}`

	resolved, err := ResolveConfig(store, nil, configJSON)
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
	if string(resolved) != configJSON {
		t.Fatalf("resolved = %q, want unchanged %q", resolved, configJSON)
	}
}

func TestResolveConfig_MissingRef_ReturnsError(t *testing.T) {
	store := newTestStore(t)
	schema := []byte(`{"type":"object","properties":{"secret":{"type":"string","format":"secret"}}}`)
	configJSON := `{"secret":"does-not-exist"}`

	if _, err := ResolveConfig(store, schema, configJSON); err == nil {
		t.Fatal("expected an error for an unresolvable secret_ref")
	}
}

func TestResolveConfig_NonStringSecretField_ReturnsError(t *testing.T) {
	store := newTestStore(t)
	schema := []byte(`{"type":"object","properties":{"secret":{"type":"string","format":"secret"}}}`)
	configJSON := `{"secret":123}`

	if _, err := ResolveConfig(store, schema, configJSON); err == nil {
		t.Fatal("expected an error for a non-string secret field")
	}
}

func TestResolveConfig_MissingFieldInConfig_Skipped(t *testing.T) {
	store := newTestStore(t)
	schema := []byte(`{"type":"object","properties":{"secret":{"type":"string","format":"secret"}}}`)
	configJSON := `{}`

	resolved, err := ResolveConfig(store, schema, configJSON)
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
	if string(resolved) != configJSON {
		t.Fatalf("resolved = %q, want unchanged %q", resolved, configJSON)
	}
}
