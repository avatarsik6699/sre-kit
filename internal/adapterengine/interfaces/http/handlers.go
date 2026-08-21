// Package http exposes GET /api/adapters — installed adapters and their manifests — per
// docs/SPEC.md §4. Session-gating is applied by the composition root once internal/auth's
// middleware is wired (docs/changes/archive/01-core-skeleton.md B9), not by this package directly.
package http

import (
	"encoding/json"
	"net/http"

	"sre-kit/internal/adapterengine/application"
)

// Handlers exposes the adapter-engine HTTP surface.
type Handlers struct {
	adaptersDir string
}

// NewHandlers wires Handlers to the directory installed adapters live under
// (config.Config.AdaptersDir).
func NewHandlers(adaptersDir string) *Handlers {
	return &Handlers{adaptersDir: adaptersDir}
}

// Register mounts GET /api/adapters on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/adapters", h.list)
}

type adapterResponse struct {
	Name               string          `json:"name"`
	Version            string          `json:"version"`
	Mode               string          `json:"mode"`
	Emits              []string        `json:"emits"`
	ConfigSchema       json.RawMessage `json:"config_schema"`
	PresentationSchema json.RawMessage `json:"presentation_schema,omitempty"`
	HeartbeatSeconds   int             `json:"heartbeat_seconds,omitempty"`
}

// list godoc
// @Summary      List installed adapters
// @Description  List every adapter found under the adapters directory, with its manifest
// @Tags         adapters
// @Produce      json
// @Security     SessionCookie
// @Success      200  {array}   http.adapterResponse
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/adapters [get]
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	installed, err := application.ListInstalled(h.adaptersDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list installed adapters"})
		return
	}

	responses := make([]adapterResponse, 0, len(installed))
	for _, adapter := range installed {
		responses = append(responses, adapterResponse{
			Name:               adapter.Manifest.Name,
			Version:            adapter.Manifest.Version,
			Mode:               string(adapter.Manifest.Mode),
			Emits:              adapter.Manifest.Emits,
			ConfigSchema:       adapter.Manifest.ConfigSchema,
			PresentationSchema: adapter.Manifest.PresentationSchema,
			HeartbeatSeconds:   adapter.Manifest.HeartbeatSeconds,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(responses)
}
