package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sre-kit/internal/alertrouter/application"
	"sre-kit/internal/alertrouter/domain"
	alertrouterhttp "sre-kit/internal/alertrouter/interfaces/http"
)

type fakeAlerts struct{ byID map[string]domain.Alert }

func newFakeAlerts() *fakeAlerts                                     { return &fakeAlerts{byID: make(map[string]domain.Alert)} }
func (f *fakeAlerts) Create(_ context.Context, a domain.Alert) error { f.byID[a.ID] = a; return nil }
func (f *fakeAlerts) Resolve(_ context.Context, id string, resolvedAt time.Time) error {
	a := f.byID[id]
	a.ResolvedAt = &resolvedAt
	f.byID[id] = a
	return nil
}
func (f *fakeAlerts) FindOpenByRule(_ context.Context, ruleID string) (domain.Alert, bool, error) {
	for _, a := range f.byID {
		if a.RuleID != nil && *a.RuleID == ruleID && a.ResolvedAt == nil {
			return a, true, nil
		}
	}
	return domain.Alert{}, false, nil
}
func (f *fakeAlerts) FindOpenSystemAlert(_ context.Context, sourceID string) (domain.Alert, bool, error) {
	return domain.Alert{}, false, nil
}
func (f *fakeAlerts) List(_ context.Context, status string) ([]domain.Alert, error) {
	var out []domain.Alert
	for _, a := range f.byID {
		out = append(out, a)
	}
	return out, nil
}

type fakeRules struct{ byID map[string]domain.AlertRule }

func newFakeRules() *fakeRules                                          { return &fakeRules{byID: make(map[string]domain.AlertRule)} }
func (f *fakeRules) Create(_ context.Context, r domain.AlertRule) error { f.byID[r.ID] = r; return nil }
func (f *fakeRules) Update(_ context.Context, r domain.AlertRule) error {
	if _, ok := f.byID[r.ID]; !ok {
		return domain.ErrRuleNotFound
	}
	f.byID[r.ID] = r
	return nil
}
func (f *fakeRules) Get(_ context.Context, id string) (domain.AlertRule, error) {
	r, ok := f.byID[id]
	if !ok {
		return domain.AlertRule{}, domain.ErrRuleNotFound
	}
	return r, nil
}
func (f *fakeRules) List(_ context.Context, sourceID string) ([]domain.AlertRule, error) {
	var out []domain.AlertRule
	for _, r := range f.byID {
		if sourceID == "" || r.SourceID == sourceID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRules) ListEnabledByChannel(_ context.Context, channelID string) ([]domain.AlertRule, error) {
	var out []domain.AlertRule
	for _, r := range f.byID {
		if r.NotifyChannelID == channelID && r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRules) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return domain.ErrRuleNotFound
	}
	delete(f.byID, id)
	return nil
}

type fakeChannels struct {
	byID map[string]domain.NotificationChannel
}

func newFakeChannels() *fakeChannels {
	return &fakeChannels{byID: make(map[string]domain.NotificationChannel)}
}
func (f *fakeChannels) Create(_ context.Context, c domain.NotificationChannel) error {
	f.byID[c.ID] = c
	return nil
}
func (f *fakeChannels) Update(_ context.Context, c domain.NotificationChannel) error {
	if _, ok := f.byID[c.ID]; !ok {
		return domain.ErrChannelNotFound
	}
	f.byID[c.ID] = c
	return nil
}
func (f *fakeChannels) Get(_ context.Context, id string) (domain.NotificationChannel, error) {
	c, ok := f.byID[id]
	if !ok {
		return domain.NotificationChannel{}, domain.ErrChannelNotFound
	}
	return c, nil
}
func (f *fakeChannels) List(_ context.Context) ([]domain.NotificationChannel, error) {
	var out []domain.NotificationChannel
	for _, c := range f.byID {
		out = append(out, c)
	}
	return out, nil
}
func (f *fakeChannels) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return domain.ErrChannelNotFound
	}
	delete(f.byID, id)
	return nil
}

type fakeSecrets struct {
	byRef map[string]string
	seq   int
}

func newFakeSecrets() *fakeSecrets { return &fakeSecrets{byRef: make(map[string]string)} }
func (f *fakeSecrets) Put(value string) (string, error) {
	f.seq++
	ref := "ref-" + string(rune('a'+f.seq))
	f.byRef[ref] = value
	return ref, nil
}
func (f *fakeSecrets) Get(ref string) (string, error) {
	v, ok := f.byRef[ref]
	if !ok {
		return "", domain.ErrChannelNotFound
	}
	return v, nil
}
func (f *fakeSecrets) Delete(ref string) error { delete(f.byRef, ref); return nil }

func newTestService() *application.Service {
	return application.NewService(newFakeAlerts(), newFakeRules(), newFakeChannels(), newFakeSecrets())
}

func TestCreateAndListAlertRule(t *testing.T) {
	svc := newTestService()
	mux := http.NewServeMux()
	alertrouterhttp.NewHandlers(svc).Register(mux)

	body, _ := json.Marshal(map[string]any{
		"source_id": "src-1", "target_name": "cpu.usage_percent", "condition": ">", "threshold": "90", "debounce_seconds": 30,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/alert-rules", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/alert-rules?source=src-1", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var rules []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rules); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %+v, want 1", rules)
	}
}

func TestCreateAndDeleteNotificationChannel(t *testing.T) {
	svc := newTestService()
	mux := http.NewServeMux()
	alertrouterhttp.NewHandlers(svc).Register(mux)

	body, _ := json.Marshal(map[string]any{"type": "telegram", "chat_id": "123", "bot_token": "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/notification-channels", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, leaked := created["bot_token"]; leaked {
		t.Fatalf("response leaked bot_token: %+v", created)
	}
	if _, leaked := created["secret_ref"]; leaked {
		t.Fatalf("response leaked secret_ref: %+v", created)
	}

	id := created["id"].(string)
	req = httptest.NewRequest(http.MethodDelete, "/api/notification-channels/"+id, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
}

func TestListAlerts(t *testing.T) {
	svc := newTestService()
	mux := http.NewServeMux()
	alertrouterhttp.NewHandlers(svc).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want empty", got)
	}
}
