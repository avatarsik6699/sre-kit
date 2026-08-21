package http

import (
	"encoding/json"
	"net/http"
	"time"

	"sre-kit/internal/platform/apierror"
	"sre-kit/internal/projects/application"
	"sre-kit/internal/projects/domain"
)

type Handlers struct{ service *application.Service }

func NewHandlers(s *application.Service) *Handlers { return &Handlers{service: s} }
func (h *Handlers) Register(m *http.ServeMux) {
	m.HandleFunc("GET /api/projects", h.list)
	m.HandleFunc("POST /api/projects", h.create)
	m.HandleFunc("PATCH /api/projects/{id}", h.update)
	m.HandleFunc("DELETE /api/projects/{id}", h.delete)
}

type response struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func encode(p domain.Project) response {
	return response{p.ID, p.Name, p.Slug, p.Description, p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339)}
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// list godoc
// @Summary List projects
// @Tags projects
// @Produce json
// @Security SessionCookie
// @Success 200 {array} response
// @Router /api/projects [get]
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		apierror.Write(w, err)
		return
	}
	out := make([]response, 0, len(items))
	for _, p := range items {
		out = append(out, encode(p))
	}
	write(w, 200, out)
}

type request struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
}

func decode(r *http.Request) (request, error) {
	var v request
	err := json.NewDecoder(r.Body).Decode(&v)
	return v, err
}

// create godoc
// @Summary Create project
// @Tags projects
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param project body request true "project"
// @Success 201 {object} response
// @Router /api/projects [post]
func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	v, err := decode(r)
	if err != nil || v.Name == nil || v.Slug == nil {
		apierror.Write(w, apierror.Invalid("name and slug are required"))
		return
	}
	desc := ""
	if v.Description != nil {
		desc = *v.Description
	}
	p, err := h.service.Create(r.Context(), *v.Name, *v.Slug, desc)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	write(w, 201, encode(p))
}

// update godoc
// @Summary Update project
// @Tags projects
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "project ID"
// @Param project body request true "project fields"
// @Success 200 {object} response
// @Router /api/projects/{id} [patch]
func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	v, err := decode(r)
	if err != nil {
		apierror.Write(w, apierror.Invalid("malformed request body"))
		return
	}
	p, err := h.service.Update(r.Context(), r.PathValue("id"), v.Name, v.Slug, v.Description)
	if err != nil {
		apierror.Write(w, err)
		return
	}
	write(w, 200, encode(p))
}

// delete godoc
// @Summary Delete project
// @Tags projects
// @Security SessionCookie
// @Param id path string true "project ID"
// @Success 204
// @Router /api/projects/{id} [delete]
func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), r.PathValue("id")); err != nil {
		apierror.Write(w, err)
		return
	}
	w.WriteHeader(204)
}
