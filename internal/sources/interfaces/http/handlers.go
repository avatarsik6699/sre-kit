// Package http exposes the Source use-cases as /api/sources (GET/POST/PATCH/DELETE), per
// docs/SPEC.md §4. Session-gating is applied by the composition root once internal/auth's
// middleware is wired (docs/changes/01-core-skeleton.md B9), not by this package directly.
package http

import (
	"encoding/json"
	"net/http"

	"sre-kit/internal/platform/apierror"
	"sre-kit/internal/sources/application"
	"sre-kit/internal/sources/domain"
)

// Handlers exposes the Source HTTP surface bound to a *application.Service.
type Handlers struct {
	service *application.Service
}

// NewHandlers wires Handlers to svc.
func NewHandlers(svc *application.Service) *Handlers {
	return &Handlers{service: svc}
}

// Register mounts every /api/sources route on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sources", h.list)
	mux.HandleFunc("POST /api/sources", h.create)
	mux.HandleFunc("PATCH /api/sources/{id}", h.update)
	mux.HandleFunc("DELETE /api/sources/{id}", h.delete)
}

type sourceResponse struct {
	ID         string  `json:"id"`
	AdapterID  string  `json:"adapter_id"`
	Config     string  `json:"config"`
	Enabled    bool    `json:"enabled"`
	LastStatus string  `json:"last_status"`
	LastSeenAt *string `json:"last_seen_at"`
}

func toResponse(source domain.Source) sourceResponse {
	resp := sourceResponse{
		ID:         source.ID,
		AdapterID:  source.AdapterName,
		Config:     source.ConfigJSON,
		Enabled:    source.Enabled,
		LastStatus: source.LastStatus,
	}
	if source.LastSeenAt != nil {
		formatted := source.LastSeenAt.Format(http.TimeFormat)
		resp.LastSeenAt = &formatted
	}
	return resp
}

// list godoc
// @Summary      List sources
// @Description  List every configured source and its current status
// @Tags         sources
// @Produce      json
// @Security     SessionCookie
// @Success      200  {array}   http.sourceResponse
// @Failure      401  {object}  map[string]string
// @Router       /api/sources [get]
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	sources, err := h.service.List(r.Context())
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]sourceResponse, 0, len(sources))
	for _, source := range sources {
		responses = append(responses, toResponse(source))
	}
	writeJSON(w, http.StatusOK, responses)
}

type createSourceRequest struct {
	AdapterID string          `json:"adapter_id"`
	Config    json.RawMessage `json:"config"`
}

// create godoc
// @Summary      Create a source
// @Description  Configure a new adapter instance
// @Tags         sources
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        source  body      http.createSourceRequest  true  "adapter_id + config"
// @Success      201     {object}  http.sourceResponse
// @Failure      400     {object}  map[string]string
// @Failure      401     {object}  map[string]string
// @Router       /api/sources [post]
func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var req createSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	configJSON := "{}"
	if len(req.Config) > 0 {
		configJSON = string(req.Config)
	}
	source, err := h.service.Create(r.Context(), req.AdapterID, configJSON)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(source))
}

type updateSourceRequest struct {
	Enabled *bool           `json:"enabled"`
	Config  json.RawMessage `json:"config"`
}

// update godoc
// @Summary      Update a source
// @Description  Enable/disable a source and/or update its config
// @Tags         sources
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        id      path      string                    true  "source ID"
// @Param        source  body      http.updateSourceRequest  true  "fields to patch"
// @Success      200     {object}  http.sourceResponse
// @Failure      400     {object}  map[string]string
// @Failure      401     {object}  map[string]string
// @Failure      404     {object}  map[string]string
// @Router       /api/sources/{id} [patch]
func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req updateSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	var configJSON *string
	if len(req.Config) > 0 {
		value := string(req.Config)
		configJSON = &value
	}
	source, err := h.service.Update(r.Context(), id, configJSON, req.Enabled)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(source))
}

// delete godoc
// @Summary      Delete a source
// @Description  Remove a configured source permanently
// @Tags         sources
// @Security     SessionCookie
// @Param        id  path  string  true  "source ID"
// @Success      204
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /api/sources/{id} [delete]
func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.Delete(r.Context(), id); err != nil {
		apierror.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
