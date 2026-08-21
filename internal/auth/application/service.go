// Package application holds the auth use-cases: first-run admin-password bootstrap, Login, and
// session validation, per docs/SPEC.md §6.
package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sre-kit/internal/auth/domain"
)

const (
	adminPasswordHashKey = "admin_password_hash"
	sessionTTL           = 24 * time.Hour
)

// SecretsStore is the port onto internal/platform/secrets.Store this service needs: get/set one
// named secret (the admin password hash). A narrow interface (not the whole Store) keeps this
// package's dependency on platform/secrets minimal and easy to fake in tests.
type SecretsStore interface {
	Get(ref string) (string, error)
	PutNamed(key string, value string) error
}

// Service implements admin-password bootstrap, Login, and in-memory session validation.
// Sessions are ephemeral (not persisted to SQLite, per docs/STACK.md's internal/auth tree, which
// has no infrastructure/ layer) — losing them on restart just forces a re-login.
type Service struct {
	secrets SecretsStore
	now     func() time.Time

	mu       sync.RWMutex
	sessions map[string]domain.Session
}

// NewService wires a Service to its SecretsStore.
func NewService(secrets SecretsStore) *Service {
	return &Service{secrets: secrets, now: time.Now, sessions: map[string]domain.Session{}}
}

// EnsureAdminPassword bootstraps the admin password on first run: if no hash is stored yet, it
// generates a random password, stores its bcrypt hash, and returns the plaintext once so the
// composition root can print it — the only time it's ever available in cleartext. Returns "" (no
// error) if a password is already configured.
func (s *Service) EnsureAdminPassword(ctx context.Context) (string, error) {
	if _, err := s.secrets.Get(adminPasswordHashKey); err == nil {
		return "", nil
	}

	password, err := domain.GeneratePassword()
	if err != nil {
		return "", fmt.Errorf("auth: generate admin password: %w", err)
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("auth: hash admin password: %w", err)
	}
	if err := s.secrets.PutNamed(adminPasswordHashKey, hash); err != nil {
		return "", fmt.Errorf("auth: store admin password hash: %w", err)
	}
	return password, nil
}

// RotateAdminPassword generates and stores a replacement owner password, invalidating all current
// in-memory sessions. It is exposed only by the local admin CLI, never by HTTP.
func (s *Service) RotateAdminPassword(ctx context.Context) (string, error) {
	password, err := domain.GeneratePassword()
	if err != nil {
		return "", fmt.Errorf("auth: generate admin password: %w", err)
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("auth: hash admin password: %w", err)
	}
	if err := s.secrets.PutNamed(adminPasswordHashKey, hash); err != nil {
		return "", fmt.Errorf("auth: store admin password hash: %w", err)
	}
	s.mu.Lock()
	s.sessions = map[string]domain.Session{}
	s.mu.Unlock()
	return password, nil
}

// Login verifies password against the stored admin password hash and, on success, issues a new
// session token valid for sessionTTL.
func (s *Service) Login(ctx context.Context, password string) (domain.Session, error) {
	hash, err := s.secrets.Get(adminPasswordHashKey)
	if err != nil {
		return domain.Session{}, domain.ErrInvalidCredentials
	}
	if !domain.VerifyPassword(hash, password) {
		return domain.Session{}, domain.ErrInvalidCredentials
	}

	token, err := domain.NewToken()
	if err != nil {
		return domain.Session{}, fmt.Errorf("auth: generate session token: %w", err)
	}
	session := domain.Session{Token: token, ExpiresAt: s.now().Add(sessionTTL)}

	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()
	return session, nil
}

// ValidateSession reports whether token identifies a live, unexpired session.
func (s *Service) ValidateSession(ctx context.Context, token string) error {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok || session.Expired(s.now()) {
		return domain.ErrSessionInvalid
	}
	return nil
}
