package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"sre-kit/internal/auth/application"
	"sre-kit/internal/auth/domain"
)

type fakeSecretsStore struct {
	values map[string]string
}

func newFakeSecretsStore() *fakeSecretsStore { return &fakeSecretsStore{values: map[string]string{}} }

func (f *fakeSecretsStore) Get(ref string) (string, error) {
	value, ok := f.values[ref]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (f *fakeSecretsStore) PutNamed(key string, value string) error {
	f.values[key] = value
	return nil
}

func TestEnsureAdminPassword_GeneratesOnFirstRunOnly(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(store)

	password, err := svc.EnsureAdminPassword(context.Background())
	if err != nil {
		t.Fatalf("EnsureAdminPassword: %v", err)
	}
	if password == "" {
		t.Fatal("EnsureAdminPassword: want a generated password on first run, got empty string")
	}

	second, err := svc.EnsureAdminPassword(context.Background())
	if err != nil {
		t.Fatalf("EnsureAdminPassword (second run): %v", err)
	}
	if second != "" {
		t.Fatalf("EnsureAdminPassword (second run): want empty string (already configured), got %q", second)
	}
}

func TestLogin_SucceedsWithCorrectPassword(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(store)
	password, err := svc.EnsureAdminPassword(context.Background())
	if err != nil {
		t.Fatalf("EnsureAdminPassword: %v", err)
	}

	session, err := svc.Login(context.Background(), password)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if session.Token == "" {
		t.Fatal("Login: want a non-empty session token")
	}

	if err := svc.ValidateSession(context.Background(), session.Token); err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
}

func TestLogin_RejectsWrongPassword(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(store)
	if _, err := svc.EnsureAdminPassword(context.Background()); err != nil {
		t.Fatalf("EnsureAdminPassword: %v", err)
	}

	if _, err := svc.Login(context.Background(), "definitely-wrong"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("Login with wrong password: got %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestValidateSession_RejectsUnknownToken(t *testing.T) {
	svc := application.NewService(newFakeSecretsStore())
	if err := svc.ValidateSession(context.Background(), "unknown-token"); !errors.Is(err, domain.ErrSessionInvalid) {
		t.Fatalf("ValidateSession(unknown token): got %v, want domain.ErrSessionInvalid", err)
	}
}

func TestValidateSession_RejectsExpiredSession(t *testing.T) {
	store := newFakeSecretsStore()
	svc := application.NewService(store)
	password, err := svc.EnsureAdminPassword(context.Background())
	if err != nil {
		t.Fatalf("EnsureAdminPassword: %v", err)
	}
	session, err := svc.Login(context.Background(), password)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	application.SetClockForTest(svc, func() time.Time { return time.Now().Add(48 * time.Hour) })

	if err := svc.ValidateSession(context.Background(), session.Token); !errors.Is(err, domain.ErrSessionInvalid) {
		t.Fatalf("ValidateSession(expired): got %v, want domain.ErrSessionInvalid", err)
	}
}
