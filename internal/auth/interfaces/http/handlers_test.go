package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sre-kit/internal/auth/application"
	authhttp "sre-kit/internal/auth/interfaces/http"
)

type fakeSecretsStore struct{ values map[string]string }

func newFakeSecretsStore() *fakeSecretsStore { return &fakeSecretsStore{values: map[string]string{}} }

func (f *fakeSecretsStore) Get(ref string) (string, error) {
	value, ok := f.values[ref]
	if !ok {
		return "", http.ErrNoCookie // any error works — Service just checks err != nil
	}
	return value, nil
}
func (f *fakeSecretsStore) PutNamed(key string, value string) error {
	f.values[key] = value
	return nil
}

func setup(t *testing.T) (*http.ServeMux, string) {
	t.Helper()
	store := newFakeSecretsStore()
	svc := application.NewService(store)
	password, err := svc.EnsureAdminPassword(context.Background())
	if err != nil {
		t.Fatalf("EnsureAdminPassword: %v", err)
	}

	mux := http.NewServeMux()
	authhttp.NewHandlers(svc).Register(mux)
	mux.HandleFunc("GET /api/sources", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := authhttp.RequireSession(svc)(mux)
	wrapped := http.NewServeMux()
	wrapped.Handle("/", protected)
	return wrapped, password
}

func TestLogin_SetsSessionCookieOnSuccess(t *testing.T) {
	mux, password := setup(t)

	body, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "session" || cookies[0].Value == "" {
		t.Fatalf("cookies = %+v, want one non-empty session cookie", cookies)
	}
	if !cookies[0].HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
}

func TestLogin_RejectsWrongPassword(t *testing.T) {
	mux, _ := setup(t)

	body, _ := json.Marshal(map[string]string{"password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestProtectedRoute_RejectsRequestWithoutSession(t *testing.T) {
	mux, _ := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestProtectedRoute_AllowsRequestWithValidSession(t *testing.T) {
	mux, password := setup(t)

	loginBody, _ := json.Marshal(map[string]string{"password": password})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, loginReq)
	cookie := loginRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHealthz_ExemptFromSessionRequirement(t *testing.T) {
	mux, _ := setup(t)
	mux2 := http.NewServeMux()
	mux2.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	_ = mux // exercised via the dedicated health mux below since setup()'s mux doesn't mount /healthz

	store := newFakeSecretsStore()
	svc := application.NewService(store)
	protected := authhttp.RequireSession(svc)(mux2)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (healthz must be exempt from session check)", rec.Code)
	}
}
