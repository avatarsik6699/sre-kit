package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sre-kit/internal/sources/domain"
	"sre-kit/internal/telemetry/application"
	telemetryhttp "sre-kit/internal/telemetry/interfaces/http"
)

type fakePushSecrets struct{ values map[string]string }

func (f *fakePushSecrets) Get(key string) (string, error) { return f.values[key], nil }
func (f *fakePushSecrets) PutNamed(key, value string) error {
	f.values[key] = value
	return nil
}

type fakeSourceLookup struct{}

func (fakeSourceLookup) Get(_ context.Context, id string) (domain.Source, error) {
	return domain.Source{ID: id}, nil
}

type fakeBatchStore struct{ reserved map[string]bool }

func (f *fakeBatchStore) Reserve(_ context.Context, sourceID, key string, _ int) (bool, error) {
	id := sourceID + ":" + key
	if f.reserved[id] {
		return false, nil
	}
	f.reserved[id] = true
	return true, nil
}
func (f *fakeBatchStore) Release(_ context.Context, sourceID, key string) error {
	delete(f.reserved, sourceID+":"+key)
	return nil
}

func TestPushRecordsRequiresTokenAndDeduplicatesBatch(t *testing.T) {
	metrics := &fakeMetricRepo{}
	service := application.NewService(metrics, &fakeCheckRepo{}, &fakeEventRepo{})
	secrets := &fakePushSecrets{values: map[string]string{}}
	batches := &fakeBatchStore{reserved: map[string]bool{}}
	mux := http.NewServeMux()
	telemetryhttp.NewPushHandlers(service, fakeSourceLookup{}, secrets, batches).Register(mux)

	rotate := httptest.NewRequest(http.MethodPost, "/api/sources/source-1/ingest-token", nil)
	rotate.SetPathValue("id", "source-1")
	rotateResponse := httptest.NewRecorder()
	mux.ServeHTTP(rotateResponse, rotate)
	if rotateResponse.Code != http.StatusCreated {
		t.Fatalf("rotate status = %d", rotateResponse.Code)
	}
	var tokenResponse map[string]string
	if err := json.Unmarshal(rotateResponse.Body.Bytes(), &tokenResponse); err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"schema_version":"1.0","records":[{"type":"metric","name":"requests","timestamp":"2024-08-21T12:00:00Z","value":7,"labels":{"traffic_class":"unclassified"}}]}`)
	unauthorized := httptest.NewRequest(http.MethodPost, "/api/sources/source-1/records", bytes.NewReader(body))
	unauthorized.SetPathValue("id", "source-1")
	unauthorized.Header.Set("Idempotency-Key", "batch-1")
	unauthorizedResponse := httptest.NewRecorder()
	mux.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorizedResponse.Code)
	}

	push := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/sources/source-1/records", bytes.NewReader(body))
		request.SetPathValue("id", "source-1")
		request.Header.Set("Authorization", "Bearer "+tokenResponse["token"])
		request.Header.Set("Idempotency-Key", "batch-1")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	if response := push(); response.Code != http.StatusAccepted {
		t.Fatalf("first push status = %d body=%s", response.Code, response.Body.String())
	}
	if response := push(); response.Code != http.StatusOK {
		t.Fatalf("duplicate push status = %d body=%s", response.Code, response.Body.String())
	}
	if len(metrics.items) != 1 || metrics.items[0].LabelsJSON != `{"traffic_class":"unclassified"}` {
		t.Fatalf("metrics = %+v", metrics.items)
	}
}
