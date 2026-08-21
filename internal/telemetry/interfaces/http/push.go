package http

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"sre-kit/internal/platform/apierror"
	"sre-kit/internal/sources/domain"
	"sre-kit/internal/telemetry/application"
)

type PushTokenStore interface {
	Get(string) (string, error)
	PutNamed(string, string) error
}
type SourceLookup interface {
	Get(context.Context, string) (domain.Source, error)
}
type BatchStore interface {
	Reserve(context.Context, string, string, int) (bool, error)
	Release(context.Context, string, string) error
}

type PushHandlers struct {
	telemetry *application.Service
	sources   SourceLookup
	tokens    PushTokenStore
	batches   BatchStore
	now       func() time.Time
}

func NewPushHandlers(t *application.Service, s SourceLookup, tokens PushTokenStore, b BatchStore) *PushHandlers {
	return &PushHandlers{telemetry: t, sources: s, tokens: tokens, batches: b, now: time.Now}
}
func (h *PushHandlers) Register(m *http.ServeMux) {
	m.HandleFunc("POST /api/sources/{id}/ingest-token", h.rotateToken)
	m.HandleFunc("POST /api/sources/{id}/records", h.records)
}
func tokenKey(id string) string { return "push_token_sha256:" + id }
func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func hashToken(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }

// rotateToken godoc
// @Summary Rotate a Source push token
// @Tags telemetry
// @Produce json
// @Security SessionCookie
// @Param id path string true "source ID"
// @Success 201 {object} map[string]string
// @Router /api/sources/{id}/ingest-token [post]
func (h *PushHandlers) rotateToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.sources.Get(r.Context(), id); err != nil {
		apierror.Write(w, err)
		return
	}
	token, err := newToken()
	if err == nil {
		err = h.tokens.PutNamed(tokenKey(id), hashToken(token))
	}
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}

type pushRecord struct {
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Timestamp string            `json:"timestamp"`
	Value     *float64          `json:"value"`
	Status    string            `json:"status"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels"`
	Meta      map[string]any    `json:"meta"`
}
type pushBatch struct {
	SchemaVersion string       `json:"schema_version"`
	Records       []pushRecord `json:"records"`
}

func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
}
func (h *PushHandlers) authorize(id, token string) bool {
	expected, err := h.tokens.Get(tokenKey(id))
	if err != nil || token == "" {
		return false
	}
	actual := hashToken(token)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
func validateRecord(record pushRecord, now time.Time) (time.Time, error) {
	ts, err := time.Parse(time.RFC3339, record.Timestamp)
	if err != nil {
		return time.Time{}, errors.New("timestamp must be RFC3339")
	}
	if ts.After(now.Add(5 * time.Minute)) {
		return time.Time{}, errors.New("timestamp is too far in the future")
	}
	switch record.Type {
	case "metric":
		if record.Name == "" || record.Value == nil {
			return time.Time{}, errors.New("metric requires name and value")
		}
	case "check":
		if record.Name == "" || (record.Status != "ok" && record.Status != "warn" && record.Status != "critical") {
			return time.Time{}, errors.New("check requires name and valid status")
		}
	case "event":
		if record.Level == "" || record.Message == "" {
			return time.Time{}, errors.New("event requires level and message")
		}
	default:
		return time.Time{}, errors.New("type must be metric, check or event")
	}
	return ts, nil
}

// records godoc
// @Summary Push an idempotent telemetry batch
// @Tags telemetry
// @Accept json
// @Produce json
// @Param id path string true "source ID"
// @Param Idempotency-Key header string true "unique batch key"
// @Param batch body pushBatch true "versioned Metric/Check/Event batch"
// @Success 202 {object} map[string]interface{}
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/sources/{id}/records [post]
func (h *PushHandlers) records(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.authorize(id, bearer(r)) {
		apierror.Write(w, apierror.Unauthorized("invalid source token"))
		return
	}
	if _, err := h.sources.Get(r.Context(), id); err != nil {
		apierror.Write(w, err)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 128 {
		apierror.Write(w, apierror.Invalid("Idempotency-Key is required and must be at most 128 characters"))
		return
	}
	var batch pushBatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&batch); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	if batch.SchemaVersion != "1.0" || len(batch.Records) == 0 || len(batch.Records) > 500 {
		apierror.Write(w, apierror.Invalid("schema_version 1.0 and 1..500 records are required"))
		return
	}
	timestamps := make([]time.Time, len(batch.Records))
	for i, record := range batch.Records {
		ts, err := validateRecord(record, h.now())
		if err != nil {
			apierror.Write(w, apierror.Invalid(err.Error()))
			return
		}
		timestamps[i] = ts
	}
	reserved, err := h.batches.Reserve(r.Context(), id, key, len(batch.Records))
	if err != nil {
		apierror.Write(w, err)
		return
	}
	if !reserved {
		writeJSON(w, http.StatusOK, map[string]any{"accepted": 0, "duplicate": true})
		return
	}
	for i, record := range batch.Records {
		var ingestErr error
		switch record.Type {
		case "metric":
			ingestErr = h.telemetry.IngestMetric(r.Context(), id, record.Name, timestamps[i], *record.Value, record.Labels)
		case "check":
			ingestErr = h.telemetry.IngestCheck(r.Context(), id, record.Name, timestamps[i], record.Status, record.Meta)
		case "event":
			ingestErr = h.telemetry.IngestEvent(r.Context(), id, timestamps[i], record.Level, record.Message, record.Labels)
		}
		if ingestErr != nil {
			_ = h.batches.Release(r.Context(), id, key)
			apierror.Write(w, ingestErr)
			return
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": len(batch.Records), "duplicate": false})
}
