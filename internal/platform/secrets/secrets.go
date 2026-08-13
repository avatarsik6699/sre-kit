// Package secrets is the shared kernel for the encrypted secrets store (secrets.enc.json, per
// docs/SPEC.md §3). SSH keys, third-party API tokens, and the admin password hash all live here,
// symmetrically encrypted with a key from an environment variable — never in SQLite, never
// committed to git. sources.config_json only ever stores the secret_ref id this package hands
// back, never the value itself.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// ErrNotFound is returned by Get when no secret exists for the given ref.
var ErrNotFound = errors.New("secrets: not found")

// Store is the in-memory view of secrets.enc.json, encrypted at rest and decrypted once on Open.
type Store struct {
	mu      sync.RWMutex
	path    string
	key     [32]byte
	secrets map[string]string // secret_ref -> plaintext value
}

// fileFormat is the on-disk shape of secrets.enc.json: AES-256-GCM ciphertext of the JSON-encoded
// secrets map, with its nonce, both base64-encoded so the file stays plain JSON.
type fileFormat struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// Open loads path (creating an empty store if the file doesn't exist yet) and decrypts it with a
// key derived from rawKey. rawKey is typically SRE_KIT_SECRETS_KEY as read by internal/platform/config.
func Open(path string, rawKey string) (*Store, error) {
	if rawKey == "" {
		return nil, errors.New("secrets: encryption key must not be empty")
	}
	store := &Store{
		path:    path,
		key:     sha256.Sum256([]byte(rawKey)),
		secrets: make(map[string]string),
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("secrets: read %s: %w", path, err)
	}

	var f fileFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("secrets: decode %s: %w", path, err)
	}
	plaintext, err := store.decrypt(f)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt %s: %w", path, err)
	}
	if err := json.Unmarshal(plaintext, &store.secrets); err != nil {
		return nil, fmt.Errorf("secrets: decode secrets payload: %w", err)
	}
	return store, nil
}

// Get returns the plaintext secret for ref, or ErrNotFound.
func (s *Store) Get(ref string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.secrets[ref]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

// Put stores value under a freshly generated secret_ref, persists the store, and returns the ref.
func (s *Store) Put(value string) (string, error) {
	ref := uuid.NewString()
	s.mu.Lock()
	s.secrets[ref] = value
	s.mu.Unlock()
	if err := s.persist(); err != nil {
		return "", err
	}
	return ref, nil
}

// PutNamed stores value under a caller-chosen, stable key and persists the store. Unlike Put
// (random ref, for adapter secrets referenced from sources.config_json), this is for the small set
// of well-known singleton secrets the core itself owns — e.g. internal/auth's admin password hash.
func (s *Store) PutNamed(key string, value string) error {
	s.mu.Lock()
	s.secrets[key] = value
	s.mu.Unlock()
	return s.persist()
}

// Delete removes ref from the store and persists the change. Deleting a ref that doesn't exist is
// a no-op.
func (s *Store) Delete(ref string) error {
	s.mu.Lock()
	delete(s.secrets, ref)
	s.mu.Unlock()
	return s.persist()
}

func (s *Store) persist() error {
	s.mu.RLock()
	plaintext, err := json.Marshal(s.secrets)
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("secrets: encode secrets payload: %w", err)
	}

	f, err := s.encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("secrets: encrypt: %w", err)
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("secrets: encode %s: %w", s.path, err)
	}

	if dir := filepath.Dir(s.path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("secrets: create dir: %w", err)
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("secrets: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("secrets: rename %s: %w", tmp, err)
	}
	return nil
}

func (s *Store) encrypt(plaintext []byte) (fileFormat, error) {
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		return fileFormat{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fileFormat{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fileFormat{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return fileFormat{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func (s *Store) decrypt(f fileFormat) ([]byte, error) {
	nonce, err := base64.StdEncoding.DecodeString(f.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(f.Ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
