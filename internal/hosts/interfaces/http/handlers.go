// Package http exposes the Host use-cases as /api/hosts (GET/POST/DELETE) and
// /api/hosts/{id}/check-connection, per docs/SPEC.md §12/§4. Session-gating is applied by the
// composition root, not by this package directly (mirrors internal/sources/interfaces/http).
package http

import (
	"encoding/json"
	"net/http"

	"sre-kit/internal/hosts/application"
	"sre-kit/internal/hosts/domain"
	"sre-kit/internal/platform/apierror"
)

// Handlers exposes the Host HTTP surface bound to a *application.Service.
type Handlers struct {
	service *application.Service
}

// NewHandlers wires Handlers to svc.
func NewHandlers(svc *application.Service) *Handlers {
	return &Handlers{service: svc}
}

// Register mounts every /api/hosts route on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/hosts", h.list)
	mux.HandleFunc("POST /api/hosts", h.create)
	mux.HandleFunc("POST /api/hosts/{id}/check-connection", h.checkConnection)
	mux.HandleFunc("DELETE /api/hosts/{id}", h.delete)
}

type hostResponse struct {
	ID                 string  `json:"id"`
	Label              string  `json:"label"`
	Address            string  `json:"address"`
	SSHPort            int     `json:"ssh_port"`
	SSHUser            string  `json:"ssh_user"`
	HostKeyFingerprint string  `json:"host_key_fingerprint"`
	DockerAvailable    bool    `json:"docker_available"`
	LastConnectedAt    *string `json:"last_connected_at"`
	LastStatus         string  `json:"last_status"`
}

// toResponse never includes the SSH key or its secret_ref — the response is the same "never returns
// the secret value" contract sources/notification_channels already keep (docs/SPEC.md §4).
func toResponse(host domain.Host) hostResponse {
	resp := hostResponse{
		ID:                 host.ID,
		Label:              host.Label,
		Address:            host.Address,
		SSHPort:            host.SSHPort,
		SSHUser:            host.SSHUser,
		HostKeyFingerprint: host.HostKeyFingerprint,
		DockerAvailable:    host.DockerAvailable,
		LastStatus:         host.LastStatus,
	}
	if host.LastConnectedAt != nil {
		formatted := host.LastConnectedAt.Format(http.TimeFormat)
		resp.LastConnectedAt = &formatted
	}
	return resp
}

// list godoc
// @Summary      List hosts
// @Description  List every configured host and its connection status
// @Tags         hosts
// @Produce      json
// @Security     SessionCookie
// @Success      200  {array}   http.hostResponse
// @Failure      401  {object}  map[string]string
// @Router       /api/hosts [get]
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.service.List(r.Context())
	if err != nil {
		apierror.Write(w, err)
		return
	}
	responses := make([]hostResponse, 0, len(hosts))
	for _, host := range hosts {
		responses = append(responses, toResponse(host))
	}
	writeJSON(w, http.StatusOK, responses)
}

type createHostRequest struct {
	Label   string `json:"label"`
	Address string `json:"address"`
	SSHPort int    `json:"ssh_port"`
	SSHUser string `json:"ssh_user"`
	SSHKey  string `json:"ssh_key"`
}

// create godoc
// @Summary      Create a host
// @Description  Register an SSH-reachable deploy target; ssh_key is stored via the secret_ref mechanism, never returned
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Param        host  body      http.createHostRequest  true  "label, address, ssh_port, ssh_user, ssh_key"
// @Success      201   {object}  http.hostResponse
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Router       /api/hosts [post]
func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var req createHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	host, err := h.service.Create(r.Context(), req.Label, req.Address, req.SSHPort, req.SSHUser, req.SSHKey)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(host))
}

// checkConnection godoc
// @Summary      Check a host's SSH connection
// @Description  Dials the host, pins its host-key fingerprint on first connect (refuses a later mismatch), and probes Docker availability
// @Tags         hosts
// @Produce      json
// @Security     SessionCookie
// @Param        id  path      string  true  "host ID"
// @Success      200 {object}  http.hostResponse
// @Failure      401 {object}  map[string]string
// @Failure      404 {object}  map[string]string
// @Failure      409 {object}  map[string]string
// @Router       /api/hosts/{id}/check-connection [post]
func (h *Handlers) checkConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	host, err := h.service.CheckConnection(r.Context(), id)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(host))
}

// delete godoc
// @Summary      Delete a host
// @Description  Remove a configured host permanently, along with its stored SSH key
// @Tags         hosts
// @Security     SessionCookie
// @Param        id  path  string  true  "host ID"
// @Success      204
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /api/hosts/{id} [delete]
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
