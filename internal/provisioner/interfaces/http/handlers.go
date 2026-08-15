// Package http exposes the provisioner's HTTP surface — GET /api/presets,
// POST /api/hosts/{id}/provision, GET /api/provisioning-runs, POST /api/provisioning-runs/{id}/retry
// — per docs/SPEC.md §4/§12. Session-gating is applied by the composition root, not by this package
// directly (mirrors every other interfaces/http package in this codebase).
package http

import (
	"encoding/json"
	"net/http"

	"sre-kit/internal/platform/apierror"
	"sre-kit/internal/provisioner/application"
	"sre-kit/internal/provisioner/domain"
)

// Handlers exposes the provisioner HTTP surface bound to a *application.Service and the presets
// directory (for listing installed presets — a static filesystem read, same shape as
// adapterengine's GET /api/adapters, so it doesn't need a Service method of its own).
type Handlers struct {
	service    *application.Service
	presetsDir string
}

// NewHandlers wires Handlers to svc and the directory installed presets live under.
func NewHandlers(svc *application.Service, presetsDir string) *Handlers {
	return &Handlers{service: svc, presetsDir: presetsDir}
}

// Register mounts every provisioner route on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/presets", h.listPresets)
	mux.HandleFunc("POST /api/hosts/{id}/provision", h.provision)
	mux.HandleFunc("GET /api/provisioning-runs", h.listRuns)
	mux.HandleFunc("POST /api/provisioning-runs/{id}/retry", h.retry)
}

type presetResponse struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProducesAdapter string `json:"produces_adapter"`
}

// listPresets godoc
// @Summary      List installed presets
// @Description  List every preset found under the presets directory, with its manifest
// @Tags         provisioning
// @Produce      json
// @Security     SessionCookie
// @Success      200  {array}   http.presetResponse
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/presets [get]
func (h *Handlers) listPresets(w http.ResponseWriter, r *http.Request) {
	presets, err := application.ListInstalledPresets(h.presetsDir)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]presetResponse, 0, len(presets))
	for _, preset := range presets {
		responses = append(responses, presetResponse{
			Name:            preset.Manifest.Name,
			Version:         preset.Manifest.Version,
			ProducesAdapter: preset.Manifest.ProducesAdapter,
		})
	}
	writeJSON(w, http.StatusOK, responses)
}

type runResponse struct {
	ID               string  `json:"id"`
	HostID           string  `json:"host_id"`
	PresetName       string  `json:"preset_name"`
	Status           string  `json:"status"`
	Step             string  `json:"step"`
	ErrorMessage     string  `json:"error_message,omitempty"`
	ProducedSourceID string  `json:"produced_source_id,omitempty"`
	StartedAt        string  `json:"started_at"`
	FinishedAt       *string `json:"finished_at"`
}

func toRunResponse(run domain.Run) runResponse {
	resp := runResponse{
		ID:               run.ID,
		HostID:           run.HostID,
		PresetName:       run.PresetName,
		Status:           run.Status,
		Step:             run.Step,
		ErrorMessage:     run.ErrorMessage,
		ProducedSourceID: run.ProducedSourceID,
		StartedAt:        run.StartedAt.Format(http.TimeFormat),
	}
	if run.FinishedAt != nil {
		formatted := run.FinishedAt.Format(http.TimeFormat)
		resp.FinishedAt = &formatted
	}
	return resp
}

type provisionRequest struct {
	PresetName string `json:"preset_name"`
}

// provision godoc
// @Summary      Deploy a preset to a host
// @Description  Runs the provisioning workflow synchronously (deploy, bootstrap, register) and returns its final state
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        id       path      string                     true  "host ID"
// @Param        preset   body      http.provisionRequest      true  "preset_name"
// @Success      201      {object}  http.runResponse
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Router       /api/hosts/{id}/provision [post]
func (h *Handlers) provision(w http.ResponseWriter, r *http.Request) {
	hostID := r.PathValue("id")
	var req provisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	run, err := h.service.Start(r.Context(), hostID, req.PresetName)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRunResponse(run))
}

// listRuns godoc
// @Summary      List provisioning runs for a host
// @Tags         provisioning
// @Produce      json
// @Security     SessionCookie
// @Param        host  query     string  true  "host ID"
// @Success      200   {array}   http.runResponse
// @Failure      401   {object}  map[string]string
// @Router       /api/provisioning-runs [get]
func (h *Handlers) listRuns(w http.ResponseWriter, r *http.Request) {
	hostID := r.URL.Query().Get("host")
	runs, err := h.service.ListByHost(r.Context(), hostID)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]runResponse, 0, len(runs))
	for _, run := range runs {
		responses = append(responses, toRunResponse(run))
	}
	writeJSON(w, http.StatusOK, responses)
}

// retry godoc
// @Summary      Retry a failed provisioning run
// @Description  Resumes from the run's last completed step rather than restarting (docs/SPEC.md §12.4)
// @Tags         provisioning
// @Produce      json
// @Security     SessionCookie
// @Param        id  path      string  true  "run ID"
// @Success      200 {object}  http.runResponse
// @Failure      400 {object}  map[string]string
// @Failure      401 {object}  map[string]string
// @Failure      404 {object}  map[string]string
// @Router       /api/provisioning-runs/{id}/retry [post]
func (h *Handlers) retry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.service.Retry(r.Context(), id)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRunResponse(run))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
