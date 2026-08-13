// Package http exposes GET /api/metrics, /api/checks, /api/events, per docs/SPEC.md §4.
// Session-gating is applied globally by the composition root via internal/auth's middleware
// (docs/changes/01-core-skeleton.md B9's httpserver.Server.Use wiring), not by this package
// directly.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"sre-kit/internal/platform/apierror"
	"sre-kit/internal/platform/wshub"
	"sre-kit/internal/telemetry/application"
	"sre-kit/internal/telemetry/domain"
)

// Handlers exposes the telemetry query HTTP surface bound to a *application.Service, plus the
// live GET /api/stream WS endpoint bound to a *wshub.Hub.
type Handlers struct {
	service *application.Service
	hub     *wshub.Hub
}

// NewHandlers wires Handlers to svc and hub.
func NewHandlers(svc *application.Service, hub *wshub.Hub) *Handlers {
	return &Handlers{service: svc, hub: hub}
}

// Register mounts /api/metrics, /api/checks, /api/events, /api/stream on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/metrics", h.listMetrics)
	mux.HandleFunc("GET /api/checks", h.listChecks)
	mux.HandleFunc("GET /api/events", h.listEvents)
	mux.HandleFunc("GET /api/stream", h.stream)
}

type metricResponse struct {
	SourceID string  `json:"source_id"`
	Name     string  `json:"name"`
	TS       string  `json:"ts"`
	Value    float64 `json:"value"`
	Labels   string  `json:"labels"`
}

// listMetrics godoc
// @Summary      List metrics
// @Description  Query a time-series slice of metric points
// @Tags         telemetry
// @Produce      json
// @Security     SessionCookie
// @Param        source  query     string  false  "source ID filter"
// @Param        name    query     string  false  "metric name filter"
// @Param        from    query     string  false  "RFC3339 lower bound"
// @Param        to      query     string  false  "RFC3339 upper bound"
// @Success      200  {array}   http.metricResponse
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /api/metrics [get]
func (h *Handlers) listMetrics(w http.ResponseWriter, r *http.Request) {
	query := domain.MetricQuery{
		SourceID: r.URL.Query().Get("source"),
		Name:     r.URL.Query().Get("name"),
	}
	if from, err := parseTimeParam(r, "from"); err != nil {
		apierror.Write(w, apierror.Invalid("from must be RFC3339"))
		return
	} else {
		query.From = from
	}
	if to, err := parseTimeParam(r, "to"); err != nil {
		apierror.Write(w, apierror.Invalid("to must be RFC3339"))
		return
	} else {
		query.To = to
	}

	metrics, err := h.service.QueryMetrics(r.Context(), query)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]metricResponse, 0, len(metrics))
	for _, metric := range metrics {
		responses = append(responses, metricResponse{
			SourceID: metric.SourceID,
			Name:     metric.Name,
			TS:       metric.TS.Format(time.RFC3339),
			Value:    metric.Value,
			Labels:   metric.LabelsJSON,
		})
	}
	writeJSON(w, http.StatusOK, responses)
}

type checkResponse struct {
	SourceID string `json:"source_id"`
	Name     string `json:"name"`
	TS       string `json:"ts"`
	Status   string `json:"status"`
	Meta     string `json:"meta"`
}

// listChecks godoc
// @Summary      List checks
// @Description  Query current check statuses
// @Tags         telemetry
// @Produce      json
// @Security     SessionCookie
// @Param        source  query  string  false  "source ID filter"
// @Success      200  {array}   http.checkResponse
// @Failure      401  {object}  map[string]string
// @Router       /api/checks [get]
func (h *Handlers) listChecks(w http.ResponseWriter, r *http.Request) {
	query := domain.CheckQuery{SourceID: r.URL.Query().Get("source")}
	checks, err := h.service.QueryChecks(r.Context(), query)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]checkResponse, 0, len(checks))
	for _, check := range checks {
		responses = append(responses, checkResponse{
			SourceID: check.SourceID,
			Name:     check.Name,
			TS:       check.TS.Format(time.RFC3339),
			Status:   check.Status,
			Meta:     check.MetaJSON,
		})
	}
	writeJSON(w, http.StatusOK, responses)
}

type eventResponse struct {
	SourceID string `json:"source_id"`
	TS       string `json:"ts"`
	Level    string `json:"level"`
	Message  string `json:"message"`
	Labels   string `json:"labels"`
}

// listEvents godoc
// @Summary      List events
// @Description  Query the event feed
// @Tags         telemetry
// @Produce      json
// @Security     SessionCookie
// @Param        source  query  string  false  "source ID filter"
// @Param        limit   query  int     false  "max events returned"
// @Success      200  {array}   http.eventResponse
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /api/events [get]
func (h *Handlers) listEvents(w http.ResponseWriter, r *http.Request) {
	query := domain.EventQuery{SourceID: r.URL.Query().Get("source")}
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		limit, err := strconv.Atoi(limitParam)
		if err != nil || limit < 0 {
			apierror.Write(w, apierror.Invalid("limit must be a non-negative integer"))
			return
		}
		query.Limit = limit
	}

	events, err := h.service.QueryEvents(r.Context(), query)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]eventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, eventResponse{
			SourceID: event.SourceID,
			TS:       event.TS.Format(time.RFC3339),
			Level:    event.Level,
			Message:  event.Message,
			Labels:   event.LabelsJSON,
		})
	}
	writeJSON(w, http.StatusOK, responses)
}

func parseTimeParam(r *http.Request, key string) (*time.Time, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &ts, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
