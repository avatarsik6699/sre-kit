package secrets

import (
	"path/filepath"
	"testing"
)

func TestPutGetRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc.json")

	store, err := Open(path, "test-key")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ref, err := store.Put("ssh-private-key-contents")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	reopened, err := Open(path, "test-key")
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	value, err := reopened.Get(ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != "ssh-private-key-contents" {
		t.Fatalf("Get returned %q, want original secret", value)
	}
}

func TestGetMissingReturnsErrNotFound(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "secrets.enc.json"), "test-key")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.Get("does-not-exist"); err != ErrNotFound {
		t.Fatalf("Get missing ref: got %v, want ErrNotFound", err)
	}
}

func TestPutNamed_StableKeyRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc.json")
	store, err := Open(path, "test-key")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.PutNamed("admin_password_hash", "bcrypt-hash-value"); err != nil {
		t.Fatalf("PutNamed: %v", err)
	}

	reopened, err := Open(path, "test-key")
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	value, err := reopened.Get("admin_password_hash")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != "bcrypt-hash-value" {
		t.Fatalf("Get returned %q, want %q", value, "bcrypt-hash-value")
	}
}

func TestWrongKeyFailsToDecrypt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc.json")
	store, err := Open(path, "right-key")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.Put("secret-value"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := Open(path, "wrong-key"); err == nil {
		t.Fatal("Open with wrong key: want error, got nil")
	}
}
